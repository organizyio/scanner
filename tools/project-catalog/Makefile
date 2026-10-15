.PHONY: default build build-all build-local build-gui clean install test tidy verify

GO   ?= go
DIST ?= dist

# Native binary suffix on Windows hosts.
EXE :=
ifeq ($(OS),Windows_NT)
EXE := .exe
endif

# GUI packages use //go:build cgo; without CGO, stubs still allow ./... to test.
TEST_PKGS := ./... 

default: build-local

# Alias: cross-compile unified CLI (same as build.sh on Unix).
build: build-all

tidy:
	$(GO) mod tidy

test: tidy
	$(GO) test $(TEST_PKGS)

# Current OS/arch — unified scan-cli plus legacy shim binaries (same flags as before).
build-local: tidy
	@echo Building scan-cli and shims into $(DIST)/
	$(GO) build -o $(DIST)/scan-cli$(EXE) ./cmd/scan-cli
	$(GO) build -o $(DIST)/scan-git$(EXE) ./cmd/scan-git
	$(GO) build -o $(DIST)/scan-fs$(EXE) ./cmd/scan-fs

# Fyne desktop app; requires CGO and platform graphics stack.
build-gui: tidy
	$(GO) build -o scan-gui$(EXE) ./cmd/scan-gui

install:
	$(GO) install ./cmd/scan-cli ./cmd/scan-git ./cmd/scan-fs

verify: tidy test build-all
	@echo verify: OK

ifeq ($(OS),Windows_NT)

build-all: tidy
	@echo Building scan-cli into $(DIST)/ (cross-compile via cmd set GOOS/GOARCH)
	@cmd /c "set GOOS=linux&& set GOARCH=amd64&& $(GO) build -o $(DIST)/scan-cli-linux-amd64 ./cmd/scan-cli"
	@cmd /c "set GOOS=linux&& set GOARCH=arm64&& $(GO) build -o $(DIST)/scan-cli-linux-arm64 ./cmd/scan-cli"
	@cmd /c "set GOOS=darwin&& set GOARCH=amd64&& $(GO) build -o $(DIST)/scan-cli-macos-amd64 ./cmd/scan-cli"
	@cmd /c "set GOOS=darwin&& set GOARCH=arm64&& $(GO) build -o $(DIST)/scan-cli-macos-arm64 ./cmd/scan-cli"
	@cmd /c "set GOOS=windows&& set GOARCH=amd64&& $(GO) build -o $(DIST)/scan-cli-windows-amd64.exe ./cmd/scan-cli"
	@cmd /c "set GOOS=windows&& set GOARCH=arm64&& $(GO) build -o $(DIST)/scan-cli-windows-arm64.exe ./cmd/scan-cli"

clean:
	-if exist "$(DIST)" rmdir /s /q "$(DIST)"
	-del /f /q scan-cli$(EXE) scan-git$(EXE) scan-fs$(EXE) scan-gui$(EXE) 2>nul
	-del /f /q scan-cli scan-git scan-fs scan-gui 2>nul

else

build-all: tidy
	@echo Building scan-cli into $(DIST)/
	GOOS=linux   GOARCH=amd64   $(GO) build -o $(DIST)/scan-cli-linux-amd64        ./cmd/scan-cli
	GOOS=linux   GOARCH=arm64   $(GO) build -o $(DIST)/scan-cli-linux-arm64        ./cmd/scan-cli
	GOOS=darwin  GOARCH=amd64   $(GO) build -o $(DIST)/scan-cli-macos-amd64        ./cmd/scan-cli
	GOOS=darwin  GOARCH=arm64   $(GO) build -o $(DIST)/scan-cli-macos-arm64        ./cmd/scan-cli
	GOOS=windows GOARCH=amd64   $(GO) build -o $(DIST)/scan-cli-windows-amd64.exe  ./cmd/scan-cli
	GOOS=windows GOARCH=arm64   $(GO) build -o $(DIST)/scan-cli-windows-arm64.exe  ./cmd/scan-cli

clean:
	rm -rf $(DIST) scan-cli scan-git scan-fs scan-gui scan-cli.exe scan-git.exe scan-fs.exe scan-gui.exe

endif
