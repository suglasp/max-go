@echo off

REM Windows run script for M.A.X. GO x86-64 in a random game
REM Suglasp

set "WORKDIR=%~dp0"

REM cls
if not exist "%WORKDIR%bin" mkdir "%WORKDIR%bin"
GOTO RUN

:RUN
REM RUN M.A.X.
echo.
echo Running M.A.X. ...
if not exist "%WORKDIR%bin\max-go.exe" goto FAILEXE
REM Old check, it ignored a configured data folder:
REM if not exist "%WORKDIR%bin\max.res" goto FAILDATA
REM A data folder named by -datadir or by data_dir in max-go.json is checked by the game itself,
REM so the script only checks its own folder when neither is given. The json test is a plain text
REM findstr for a non empty "data_dir" value, not a real parse.
REM The pattern has no quote character in it on purpose: an escaped quote breaks the quote counting of cmd.
REM A value must start with a letter, digit, dot, slash, backslash, tilde, colon, underscore or dash.
set "HASDIR="
for %%A in (%*) do for /F "delims==" %%B in ("%%~A") do (
    if /I "%%B"=="-datadir" set "HASDIR=1"
    if /I "%%B"=="--datadir" set "HASDIR=1"
)
if exist "%WORKDIR%bin\max-go.json" findstr /R /C:"data_dir.[ ]*:[ ]*.[A-Za-z0-9_.:/~\-]" "%WORKDIR%bin\max-go.json" > nul && set "HASDIR=1"
if not defined HASDIR if not exist "%WORKDIR%bin\max.res" goto FAILDATA
REM Old launch, extra arguments were not passed on:
REM "%WORKDIR%bin\max-go.exe" -random -difficulty 1 -npc 1 -derelicts common
"%WORKDIR%bin\max-go.exe" -random -difficulty 1 -npc 1 -derelicts common %*
goto EXIT

:FAILEXE
echo Err: No M.A.X. binary?
PAUSE > NUL
GOTO EXIT

:FAILDATA
echo Err: No M.A.X. data in folder?
PAUSE > NUL
GOTO EXIT

:EXIT
echo.
echo END RUN.
exit