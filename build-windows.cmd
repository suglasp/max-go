@echo off

REM Windows build script for M.A.X. GO x86-64
REM   note : We target only 64 bit OS.
REM make sure go.exe in the PATH env var to be found!
REM Suglasp

REM Keep GOOS and GOARCH local to this script.
setlocal
set GOARCH=amd64
set CGO_ENABLED=0
set "WORKDIR=%~dp0"

REM Keep each platform in its own build folder so Windows and Linux outputs coexist.
set "BUILDDIR=%WORKDIR%build\windows-x64"
set "LINUXDIR=%WORKDIR%build\linux-x64"
set "PACKDIR=%WORKDIR%build\pack"
set "RELEASEDIR=%WORKDIR%release"

REM Resolve relative package paths from this script's folder.
pushd "%WORKDIR%"

REM The first version-file line names the release archives.
set "VERSION=0.0.0"
for /f "usebackq delims=" %%V in ("%WORKDIR%version") do (
    set "VERSION=%%V"
    goto GOTVERSION
)
:GOTVERSION
set "WINNAME=max-go-%VERSION%-windows-x64"
set "LINUXNAME=max-go-%VERSION%-linux-x64"

echo.
echo Prepping Go ...
go mod tidy

REM Rebuild the Windows output folder so stale files cannot enter a release archive.
REM Keep Linux help text produced on a Linux host, but remove Windows DLLs from that folder.
if exist "%BUILDDIR%" rmdir /S /Q "%BUILDDIR%"
mkdir "%BUILDDIR%"
if not exist "%LINUXDIR%" mkdir "%LINUXDIR%"
del /Q "%LINUXDIR%\*.dll" 2> nul

echo.
echo Building M.A.X. for Windows x64 ...
set GOOS=windows
go build -o "%BUILDDIR%/" ./cmd/max-go
if errorlevel 1 goto FAIL

echo.
echo Building M.A.X. utilities for Windows x64 ...
go build -o "%BUILDDIR%/" ./cmd/max-restool
if errorlevel 1 goto FAIL
go build -o "%BUILDDIR%/" ./cmd/max-convert
if errorlevel 1 goto FAIL
go build -o "%BUILDDIR%/" ./cmd/max-settings
if errorlevel 1 goto FAIL

REM Ebitengine 2.10 drives X11 and OpenGL from pure Go, so the Linux game cross-builds with CGO off.
REM The utilities are pure Go on both platforms.
echo.
echo Cross building M.A.X. for Linux x64 ...
set GOOS=linux
go build -o "%LINUXDIR%/" ./cmd/max-go
if errorlevel 1 goto FAIL

echo.
echo Cross building M.A.X. utilities for Linux x64 ...
go build -o "%LINUXDIR%/" ./cmd/max-restool
if errorlevel 1 goto FAIL
go build -o "%LINUXDIR%/" ./cmd/max-convert
if errorlevel 1 goto FAIL
go build -o "%LINUXDIR%/" ./cmd/max-settings
if errorlevel 1 goto FAIL

REM Restore the host platform before running the Windows binaries for help text.
set GOOS=

echo.
echo Writing CLI help ...
REM Generate Windows help text here. Linux help text must be generated on Linux.
"%BUILDDIR%\max-go.exe" -h > "%BUILDDIR%\max-go-help.txt" 2>&1
"%BUILDDIR%\max-restool.exe" help > "%BUILDDIR%\max-restool-help.txt" 2>&1
"%BUILDDIR%\max-convert.exe" help > "%BUILDDIR%\max-convert-help.txt" 2>&1
"%BUILDDIR%\max-settings.exe" -h > "%BUILDDIR%\max-settings-help.txt" 2>&1

echo.
echo Packing LANG.RES ...
REM The port's addon archive: language strings + unit description keys, packed from langres\.
"%BUILDDIR%\max-restool.exe" pack "%WORKDIR%langres" "%BUILDDIR%\LANG.RES"
if errorlevel 1 goto FAIL
copy /Y "%BUILDDIR%\LANG.RES" "%LINUXDIR%\" > nul

echo.
echo Copying documentation ...
call :PAYLOAD "%BUILDDIR%"
call :PAYLOAD "%LINUXDIR%"

REM Copy the matching native audio libraries so each archive contains only its platform's files.
for %%F in ("%WORKDIR%dependencies\soloud-bin\*.dll") do if exist "%%~fF" copy /Y "%%~fF" "%BUILDDIR%\" > nul
for %%F in ("%WORKDIR%dependencies\miniaudio-bin\*.dll") do if exist "%%~fF" copy /Y "%%~fF" "%BUILDDIR%\" > nul
for %%F in ("%WORKDIR%dependencies\soloud-bin\*_x64.so") do if exist "%%~fF" copy /Y "%%~fF" "%LINUXDIR%\" > nul
for %%F in ("%WORKDIR%dependencies\miniaudio-bin\*_x64.so") do if exist "%%~fF" copy /Y "%%~fF" "%LINUXDIR%\" > nul

if "%MAX_GO_BUILD_ONLY%"=="1" goto DONE

echo.
echo Packing release archives ...
if not exist "%RELEASEDIR%" mkdir "%RELEASEDIR%"
if exist "%PACKDIR%" rmdir /S /Q "%PACKDIR%"
mkdir "%PACKDIR%"

REM Copy under the release name so each archive unpacks into one folder.
REM Compress-Archive replaces the ZIP only after it creates the new archive.
xcopy /E /I /Q /Y "%BUILDDIR%" "%PACKDIR%\%WINNAME%" > nul
powershell -NoProfile -Command "Compress-Archive -Path '%PACKDIR%\%WINNAME%' -DestinationPath '%RELEASEDIR%\%WINNAME%.zip' -Force"
if errorlevel 1 goto FAIL

REM Pack the Linux folder only when it contains the game binary.
REM pack-linux-tar.py writes Unix modes because NTFS does not store executable bits.
if exist "%LINUXDIR%\max-go" (
    python "%WORKDIR%pack-linux-tar.py" "%LINUXDIR%" "%RELEASEDIR%\%LINUXNAME%.tar.zst" "%LINUXNAME%"
    if errorlevel 1 goto FAILTAR
) else (
    echo   Linux archive skipped, build\linux-x64\max-go is not there.
)
goto PACKED

:FAILTAR
echo   Linux archive FAILED. It needs python on PATH, because tar.exe cannot store
echo   the Unix execute bit and the unpacked binaries would not run. Any previous
echo   archive was left untouched.

:PACKED
rmdir /S /Q "%PACKDIR%"

:DONE
echo.
echo ---------------------------------
echo Built:
echo   build\windows-x64  (game + utilities)
echo   build\linux-x64    (game + utilities, no help text)
echo.
echo Packed into release\ (skipped with MAX_GO_BUILD_ONLY=1).
echo.
echo Start stage-windows.cmd to stage.
echo.

popd
endlocal
exit /b 0

REM Copy the common release payload.
:PAYLOAD
copy /Y "%WORKDIR%README.md" "%~1\" > nul
copy /Y "%WORKDIR%CHANGES.md" "%~1\" > nul
copy /Y "%WORKDIR%max-lavender.png" "%~1\" > nul

REM LANG.RES (UI strings + unit descriptions) is packed above, beside the executable.

REM Refresh the build-only settings template. Live settings remain in bin\max-go.json.
REM The game still reads a legacy settings.json when present, but releases do not ship one.
copy /Y "%WORKDIR%max-go.json" "%~1\max-go.json" > nul

REM Ship the version stamp beside the executable.
copy /Y "%WORKDIR%version" "%~1\" > nul

REM Ship license and readme files for the bundled audio libraries.
for %%F in ("%WORKDIR%dependencies\miniaudio-bin\*-LICENSE*") do if exist "%%~fF" copy /Y "%%~fF" "%~1\" > nul
for %%F in ("%WORKDIR%dependencies\soloud-bin\*-LICENSE*") do if exist "%%~fF" copy /Y "%%~fF" "%~1\" > nul
for %%F in ("%WORKDIR%dependencies\soloud-bin\*-readme*") do if exist "%%~fF" copy /Y "%%~fF" "%~1\" > nul
goto :eof

:FAIL
set GOOS=
echo.
echo Build failed.
popd
endlocal
exit /b 1
