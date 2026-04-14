# Upscaler — build targets
#
# Requirements:
#   Linux build:   gcc, libgl-dev, xorg-dev  (Fyne deps)
#   Windows build: mingw-w64  (dnf install mingw64-gcc  OR  apt install gcc-mingw-w64-x86-64)

BINARY  := upscaler
MODULE  := github.com/Magneticdud/UpscalerGUI

.PHONY: linux windows all clean

all: linux windows

linux:
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
		go build -ldflags="-s -w" -o $(BINARY)-linux ./
	@echo "Built: $(BINARY)-linux"

windows:
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 \
		CC=x86_64-w64-mingw32-gcc \
		go build -ldflags="-s -w -H windowsgui" -o $(BINARY)-windows.exe ./
	@echo "Built: $(BINARY)-windows.exe"

clean:
	rm -f $(BINARY)-linux $(BINARY)-windows.exe
