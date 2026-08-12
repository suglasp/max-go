@echo off

REM Windows stage script for M.A.X. GO x86-64
REM Suglasp

set "WORKDIR=%~dp0"

REM Stage the Windows build into bin\, which is the folder the game runs from.
set "BUILDDIR=%WORKDIR%build\windows-x64"

REM cls
if not exist "%WORKDIR%bin" mkdir "%WORKDIR%bin"
if exist "%BUILDDIR%" goto PREP
GOTO RUN

:PREP
REM PREP FILES
echo.
echo Prepping M.A.X. files ...
REM Copy only binaries and help files. A broad max-*.* glob would also overwrite bin\max-go.json.
copy /Y "%BUILDDIR%\max-*.exe" "%WORKDIR%bin\" > nul
copy /Y "%BUILDDIR%\max-*-help.txt" "%WORKDIR%bin\" > nul
copy /Y "%BUILDDIR%\soloud_x64.dll" "%WORKDIR%bin\" > nul
copy /Y "%BUILDDIR%\miniaudio_x64.dll" "%WORKDIR%bin\" > nul
copy /Y "%BUILDDIR%\README.md" "%WORKDIR%bin\" > nul
copy /Y "%BUILDDIR%\max-lavender.png" "%WORKDIR%bin\" > nul
REM The game loads its UI strings from beside the executable.
copy /Y "%BUILDDIR%\lang-en-us.ini" "%WORKDIR%bin\" > nul
REM Refresh the version stamp during every stage.
copy /Y "%BUILDDIR%\version" "%WORKDIR%bin\" > nul
REM Ship attribution files with the bundled audio libraries.
copy /Y "%BUILDDIR%\*-LICENSE*" "%WORKDIR%bin\" > nul
copy /Y "%BUILDDIR%\*-readme*" "%WORKDIR%bin\" > nul
REM Seed max-go.json only when bin\ has no live settings file.
REM The game still reads a legacy settings.json when present, but releases do not ship one.
if not exist "%WORKDIR%bin\max-go.json" copy /Y "%BUILDDIR%\max-go.json" "%WORKDIR%bin\max-go.json" > nul
goto EXIT

:EXIT
echo.
echo -------------------------------
echo Start run-windows.cmd to launch
echo.
exit
