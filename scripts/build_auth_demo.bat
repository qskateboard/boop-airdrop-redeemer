@echo off

echo Building auth_demo utility...
go build -o ../bin/auth_demo.exe ../cmd/auth_demo/main.go
if %ERRORLEVEL% EQU 0 (
    echo Build successful! The utility is available at ../bin/auth_demo.exe
) else (
    echo Build failed with error code %ERRORLEVEL%
) 

pause