#!/usr/bin/env pwsh

# 设置编译版本
$VERSION = "5.0.2026.0116"
$PROJECT_NAME = "ntr"
$OUTPUT_DIR = "dist"
$PROJECT_ROOT = "$(Get-Location)"
$BUILD_DATE = $(Get-Date -Format 'yyyy-MM-dd')

# 清理并创建输出目录
if (Test-Path $OUTPUT_DIR) {
    Remove-Item -Path $OUTPUT_DIR -Recurse -Force | Out-Null
}
New-Item -ItemType Directory -Path $OUTPUT_DIR | Out-Null

# 编译函数
function Build-Project {
    param(
        [string]$GOOS,
        [string]$GOARCH,
        [string]$EXT = ""
    )
    
    # 格式化输出文件名
    $OUTPUT_NAME = "$PROJECT_NAME-$GOOS-$GOARCH$EXT"
    $OUTPUT_PATH = Join-Path $OUTPUT_DIR $OUTPUT_NAME
    
    Write-Host "Building $GOOS/$GOARCH..." -ForegroundColor Cyan
    
    # 设置环境变量
    $env:GOOS = $GOOS
    $env:GOARCH = $GOARCH
    
    # 构建参数 - 简化版本，只设置必要的版本信息
    $BUILD_FLAGS = @(
        "build",
        "-o", $OUTPUT_PATH,
        "-ldflags", "-X 'github.com/moeart/ntr/cli.version=$VERSION' -X 'github.com/moeart/ntr/cli.date=$BUILD_DATE'"
    )
    
    # 执行构建
    & go @BUILD_FLAGS
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host "✓ Build successful: $OUTPUT_PATH" -ForegroundColor Green
    } else {
        Write-Host "✗ Build failed: $GOOS/$GOARCH" -ForegroundColor Red
    }
    
    # 清理环境变量
    Remove-Item Env:GOOS -ErrorAction SilentlyContinue
    Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
}

# 编译 Linux 平台 (只保留常见版本)
Write-Host "=== Building Linux Platforms ===" -ForegroundColor Yellow
Build-Project -GOOS "linux" -GOARCH "arm" -EXT ""
Build-Project -GOOS "linux" -GOARCH "arm64" -EXT ""
Build-Project -GOOS "linux" -GOARCH "386" -EXT ""
Build-Project -GOOS "linux" -GOARCH "amd64" -EXT ""

# 编译 Windows 平台 (只保留常见版本)
Write-Host "=== Building Windows Platforms ===" -ForegroundColor Yellow
Build-Project -GOOS "windows" -GOARCH "386" -EXT ".exe"
Build-Project -GOOS "windows" -GOARCH "amd64" -EXT ".exe"
Build-Project -GOOS "windows" -GOARCH "arm64" -EXT ".exe"

# 编译 Darwin 平台 (只保留常见版本)
Write-Host "=== Building Darwin Platforms ===" -ForegroundColor Yellow
Build-Project -GOOS "darwin" -GOARCH "amd64" -EXT ""
Build-Project -GOOS "darwin" -GOARCH "arm64" -EXT ""

Write-Host "=== All Builds Complete! ===" -ForegroundColor Yellow
Write-Host "Output Directory: $(Join-Path $PROJECT_ROOT $OUTPUT_DIR)" -ForegroundColor Yellow
Write-Host "Build Version: $VERSION" -ForegroundColor Yellow
