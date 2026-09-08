@echo off
REM Build Z-Image WebUI: frontend -> embed -> single exe
REM Usage: build.bat  (run from this directory)

setlocal
cd /d "%~dp0"

echo [1/3] Building frontend...
pushd web
call npm run build
if errorlevel 1 (
  echo Frontend build failed.
  popd
  exit /b 1
)
popd

echo [2/3] Building backend with embedded frontend...
go build -o zimage-webui.exe .
if errorlevel 1 (
  echo Backend build failed.
  exit /b 1
)

echo [3/3] Done: zimage-webui.exe
endlocal
