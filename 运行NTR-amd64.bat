@echo off
chcp 65001 >nul
set "TARGET="
set /p "TARGET=请输入目标域名或 IP 地址: "
if "%TARGET%"=="" (
    echo 未输入目标地址，程序退出。
    pause
    exit /b 1
)
"%~dp0ntr-windows-amd64.exe" "%TARGET%"
echo.
pause
