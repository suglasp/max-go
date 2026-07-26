@echo off

REM Windows build script for M.A.X. GO x86-64
REM   note : We target only 64 bit OS.
REM make sure go.exe in the PATH env var to be found!
REM Suglasp

set GOARCH=amd64
set "WORKDIR=%~dp0"

REM Anchor to the script's own folder so the relative ./cmd/... package paths and
REM the -o build/ output resolve correctly no matter where the script is launched from.
pushd "%WORKDIR%"

echo.
echo Prepping Go ...
go mod tidy
if not exist "%WORKDIR%build" mkdir "%WORKDIR%build"

echo.
echo Building M.A.X. in Golang ...
go build -o build/ ./cmd/max-go

echo.
echo Building M.A.X. utilities ...
go build -o build/ ./cmd/max-restool
go build -o build/ ./cmd/max-convert
go build -o build/ ./cmd/max-settings

echo.
echo Writing CLI help ...
"%WORKDIR%build\max-go.exe" -h > "%WORKDIR%build\max-go-help.txt" 2>&1
"%WORKDIR%build\max-restool.exe" help > "%WORKDIR%build\max-restool-help.txt" 2>&1
"%WORKDIR%build\max-convert.exe" help > "%WORKDIR%build\max-convert-help.txt" 2>&1
"%WORKDIR%build\max-settings.exe" -h > "%WORKDIR%build\max-settings-help.txt" 2>&1

echo.
echo Copying documentation ...
copy /Y "%WORKDIR%README.md" "%WORKDIR%build\" > nul

REM The port's own UI string file; the game expects it beside the executable.
copy /Y "%WORKDIR%lang-en-us.ini" "%WORKDIR%build\" > nul

REM The settings template. build\ is pure output, so it is refreshed every build
REM the player's live settings live in bin\max-go.json and are never touched from here.
REM The (beta) legacy settings.json is still read if a player has one, but it is no longer shipped.
copy /Y "%WORKDIR%max-go.json" "%WORKDIR%build\max-go.json" > nul

REM The release version stamp. It ships beside the binary so a player can tell which build they have without launching the game.
copy /Y "%WORKDIR%version" "%WORKDIR%build\" > nul


REM Check dependancies
for %%F in ("%WORKDIR%dependencies\soloud-bin\*.dll") do if exist "%%~fF" copy /Y "%%~fF" "%WORKDIR%build\" > nul
for %%F in ("%WORKDIR%dependencies\miniaudio-bin\*.dll") do if exist "%%~fF" copy /Y "%%~fF" "%WORKDIR%build\" > nul
for %%F in ("%WORKDIR%dependencies\soloud-bin\*.so") do if exist "%%~fF" copy /Y "%%~fF" "%WORKDIR%build\" > nul
for %%F in ("%WORKDIR%dependencies\miniaudio-bin\*.so") do if exist "%%~fF" copy /Y "%%~fF" "%WORKDIR%build\" > nul
REM License and readme files for the bundled audio libraries.
REM Both must ship: redistribute both binaries, so both attributions travel with them.
REM The second line used to be a duplicate of the miniaudio glob, which left soloud-LICENSE behind.
for %%F in ("%WORKDIR%dependencies\miniaudio-bin\*-LICENSE*") do if exist "%%~fF" copy /Y "%%~fF" "%WORKDIR%build\" > nul
for %%F in ("%WORKDIR%dependencies\soloud-bin\*-LICENSE*") do if exist "%%~fF" copy /Y "%%~fF" "%WORKDIR%build\" > nul
for %%F in ("%WORKDIR%dependencies\soloud-bin\*-readme*") do if exist "%%~fF" copy /Y "%%~fF" "%WORKDIR%build\" > nul
if "%MAX_GO_BUILD_ONLY%"=="1" (
    popd
    exit /b 0
)

echo.
echo ---------------------------------
echo Start stage-windows.cmd to stage.
echo.

popd
