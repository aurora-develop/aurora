@echo off
SETLOCAL

REM 指定编码为 UTF-8
chcp 65001

REM 设置要生成的可执行文件的名称
set OUTPUT_NAME=aurora

REM 设置 Go 源文件的名称
SET GOFILE=aurora

REM 设置输出目录
SET OUTPUTDIR=target

REM 确保输出目录存在
IF NOT EXIST %OUTPUTDIR% MKDIR %OUTPUTDIR%

REM 编译为 Windows/amd64
echo 开始编译 Windows/amd64
SET GOOS=windows
SET GOARCH=amd64
go build -o %OUTPUTDIR%/%OUTPUT_NAME%_windows_amd64.exe %GOFILE%
echo 编译完成 Windows/amd64

REM 编译为 Windows/386
echo 开始编译 Windows/386
SET GOOS=windows
SET GOARCH=386
go build -o %OUTPUTDIR%/%OUTPUT_NAME%_windows_386.exe %GOFILE%
echo 编译完成 Windows/386

REM 编译为 Linux/amd64
echo 开始编译 Linux/amd64
SET GOOS=linux
SET GOARCH=amd64
go build -o %OUTPUTDIR%/%OUTPUT_NAME%_linux_amd64 %GOFILE%
echo 编译完成 Linux/amd64

REM 编译为 macOS/amd64
echo 开始编译 macOS/amd64
SET GOOS=darwin
SET GOARCH=amd64
go build -o %OUTPUTDIR%/%OUTPUT_NAME%_macos_amd64 %GOFILE%
echo 编译完成 macOS/amd64

REM 编译为 freebsd/amd64
echo 开始编译 freebsd/amd64
SET GOOS=freebsd
SET GOARCH=amd64
go build -o %OUTPUTDIR%/%OUTPUT_NAME%_freebsd_amd64 %GOFILE%
echo 编译完成 freebsd/amd64

REM 结束批处理脚本
ENDLOCAL
echo 编译完成!
