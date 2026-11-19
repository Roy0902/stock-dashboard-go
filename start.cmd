@echo off
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0scripts\start.ps1" -Detached %*
if "%~1"=="" pause
