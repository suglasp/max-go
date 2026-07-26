@echo off

REM Windows stage script for M.A.X. GO x86-64
REM Suglasp

set "WORKDIR=%~dp0"

REM cls
if not exist "%WORKDIR%bin" mkdir "%WORKDIR%bin"
if exist "%WORKDIR%build" goto PREP
GOTO RUN

:PREP
REM PREP FILES
echo.
echo Prepping M.A.X. files ...
REM The binaries and their help texts are named by pattern, never swept up by a bare
REM max-*.* glob: that would also match the max-go.json template and overwrite the
REM live settings the player has in bin\.
copy /Y "%WORKDIR%build\max-*.exe" "%WORKDIR%bin\" > nul
copy /Y "%WORKDIR%build\max-*-help.txt" "%WORKDIR%bin\" > nul
copy /Y "%WORKDIR%build\soloud_x64.dll" "%WORKDIR%bin\" > nul
copy /Y "%WORKDIR%build\miniaudio_x64.dll" "%WORKDIR%bin\" > nul
copy /Y "%WORKDIR%build\README.md" "%WORKDIR%bin\" > nul
copy /Y "%WORKDIR%build\max-lavender.png" "%WORKDIR%bin\" > nul
REM The port's own UI string file; the game expects it beside the executable.
copy /Y "%WORKDIR%build\lang-en-us.ini" "%WORKDIR%bin\" > nul
REM The release version stamp, refreshed every stage so bin\ never reports a stale build.
copy /Y "%WORKDIR%build\version" "%WORKDIR%bin\" > nul
REM Attribution for the bundled audio libraries: it must sit beside the binaries we ship.
copy /Y "%WORKDIR%build\*-LICENSE*" "%WORKDIR%bin\" > nul
copy /Y "%WORKDIR%build\*-readme*" "%WORKDIR%bin\" > nul
REM Seed the settings template only into a fresh bin.
REM The player's live settings are bin\max-go.json (the game runs from bin\ and writes them there),
REM so a stage must never overwrite it. The legacy settings.json is still READ if a player has one
REM (max-go.json wins), but it is no longer shipped, so nothing seeds it here.
if not exist "%WORKDIR%bin\max-go.json" copy /Y "%WORKDIR%build\max-go.json" "%WORKDIR%bin\max-go.json" > nul
goto EXIT

:EXIT
echo.
echo -------------------------------
echo Start run-windows.cmd to launch
echo.
exit