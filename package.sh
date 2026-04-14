#!/usr/bin/env bash
set -euo pipefail

BINARY="upscaler"
OUTDIR="dist"

# ── build ──────────────────────────────────────────────────────────────────────

echo "==> Building Linux..."
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o "${BINARY}-linux" ./

echo "==> Building Windows..."
CGO_ENABLED=1 GOOS=windows GOARCH=amd64 \
    CC=x86_64-w64-mingw32-gcc \
    go build -ldflags="-s -w -H windowsgui" -o "${BINARY}-windows.exe" ./

# ── package ────────────────────────────────────────────────────────────────────

mkdir -p "${OUTDIR}"

echo "==> Packaging Linux..."
LINUX_ZIP="${OUTDIR}/${BINARY}-linux.zip"
rm -f "${LINUX_ZIP}"
zip -j "${LINUX_ZIP}" "${BINARY}-linux"
zip -r "${LINUX_ZIP}" \
    bin/realesrgan-ncnn-vulkan \
    bin/realcugan-ncnn-vulkan \
    bin/jpeg2png \
    bin/README-jpeg2png.md \
    bin/models \
    bin/models-pro \
    bin/models-se \
    bin/README-realesrgan.md \
    bin/README-realcugan.md

echo "==> Packaging Windows..."
WIN_ZIP="${OUTDIR}/${BINARY}-windows.zip"
rm -f "${WIN_ZIP}"
zip -j "${WIN_ZIP}" "${BINARY}-windows.exe"
zip -r "${WIN_ZIP}" \
    bin/realesrgan-ncnn-vulkan.exe \
    bin/realcugan-ncnn-vulkan.exe \
    bin/jpeg2png.exe \
    bin/README-jpeg2png.md \
    bin/vcomp140.dll \
    bin/vcomp140d.dll \
    bin/models \
    bin/models-pro \
    bin/models-se \
    bin/README-realesrgan.md \
    bin/README-realcugan.md

echo ""
echo "Done! Archives in ${OUTDIR}/:"
ls -lh "${OUTDIR}/"
