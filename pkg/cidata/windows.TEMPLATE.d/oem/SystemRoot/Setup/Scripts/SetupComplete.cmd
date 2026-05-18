@echo off
powershell.exe -NoProfile -ExecutionPolicy RemoteSigned -File "%ProgramData%\Lima\windows-setup.ps1"
exit /b 0
