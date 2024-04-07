#!/bin/bash

export GOPROXY=https://goproxy.io

go get

export CGO_ENABLED=0
PKG=aurora

targets=(
    "windows/amd64"
    "linux/amd64"
    "darwin/amd64"
    "windows/386"
    "linux/386"
    "darwin/386"
    "linux/arm"
    "linux/arm64"
)

upxPath=$(command -v upx)

for target in "${targets[@]}"; do
    GOOS=${target%/*}
    GOARCH=${target#*/}
    outputDir="bin/${GOOS}_${GOARCH}"
    outputFile="${outputDir}/${PKG}"
    archiveName="${PKG}-${GOOS}-${GOARCH}.tar.gz"
    mkdir -p $(dirname ${outputFile})
    GOOS=$GOOS GOARCH=$GOARCH go build -ldflags="-s -w -extldflags '-static'" -o ${outputFile} *.go
    if [ -n "$upxPath" ]; then
        $upxPath -9 ${outputFile}
    fi
    # Archive the binary
    if [ "$GOOS" = "windows" ]; then
        zip -j "${outputDir}/${PKG}-${GOOS}-${GOARCH}.zip" "${outputFile}"
    else
        tar -C "${outputDir}" -czf "${outputDir}/${archiveName}" "${PKG}"
    fi
done
