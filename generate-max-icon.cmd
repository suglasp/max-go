@echo off
setlocal

REM Suglasp
REM Regenerate the Windows icon resource files for cmd/max-go.
REM Override these variables locally if your tool or icon layout differs.

set "WORKDIR=%~dp0"
if not defined PACKAGE_DIR set "PACKAGE_DIR=%WORKDIR%cmd\max-go"
if not defined WINDRES set "WINDRES=windres.exe"
if not defined WINDRES_TARGET set "WINDRES_TARGET=pe-x86-64"
if not defined ICON_DIR set "ICON_DIR=%WORKDIR%ico"
if not defined ICON_NAME set "ICON_NAME=max-go.ico"
if not defined RC_DIR set "RC_DIR=%PACKAGE_DIR%"
if not defined RC_NAME set "RC_NAME=max_icon.rc"
if not defined SYSO_NAME set "SYSO_NAME=max_icon_windows_amd64.syso"
if not defined RC_ONLY set "RC_ONLY=0"

set "ICON_PATH=%ICON_DIR%\%ICON_NAME%"
set "RC_PATH=%RC_DIR%\%RC_NAME%"
set "SYSO_PATH=%RC_DIR%\%SYSO_NAME%"

if not defined ICON_REF (
    if /I "%ICON_DIR%"=="%WORKDIR%ico" (
        set "ICON_REF=../../ico/%ICON_NAME%"
    ) else if /I "%ICON_DIR%"=="%RC_DIR%" (
        set "ICON_REF=%ICON_NAME%"
    ) else (
        set "ICON_REF=%ICON_PATH:\=/%"
    )
)

if not exist "%ICON_PATH%" (
    echo Icon not found: "%ICON_PATH%"
    exit /b 1
)

if not exist "%RC_DIR%" mkdir "%RC_DIR%"

> "%RC_PATH%" echo IDI_ICON1 ICON "%ICON_REF%"
echo Wrote "%RC_PATH%"

if "%RC_ONLY%"=="1" exit /b 0

set "WINDRES_CMD=%WINDRES%"
if exist "%WINDRES%" for %%I in ("%WINDRES%") do set "WINDRES_CMD=%%~fI"

if not exist "%WINDRES_CMD%" (
    where "%WINDRES_CMD%" > nul 2>&1
    if errorlevel 1 (
        echo windres tool not found: "%WINDRES%"
        echo Set WINDRES to your windres.exe path, for example tools\w64devkit\bin\windres.exe.
        exit /b 1
    )
)

pushd "%RC_DIR%" > nul
"%WINDRES_CMD%" --target=%WINDRES_TARGET% -i "%RC_NAME%" -o "%SYSO_NAME%"
set "WINDRES_RC=%ERRORLEVEL%"
popd > nul

if not "%WINDRES_RC%"=="0" exit /b %WINDRES_RC%
echo Wrote "%SYSO_PATH%"
