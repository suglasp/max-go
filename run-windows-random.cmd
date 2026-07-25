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
if not exist "%WORKDIR%bin\max.res" goto FAILDATA
"%WORKDIR%bin\max-go.exe" -random -difficulty 1 -npc 1 -derilicts common
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