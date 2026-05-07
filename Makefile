# Version — seul endroit à modifier pour une nouvelle release
VERSION = 2.45

# Variables
BINARY_NAME=DXHunter.exe
FRONTEND_DIR=frontend
DIST_DIR=$(FRONTEND_DIR)/dist
GO_FILES=$(shell find . -name '*.go' -not -path "./$(FRONTEND_DIR)/*")
CGO_ENABLED ?= 1
GOFLAGS = CGO_ENABLED=$(CGO_ENABLED)
LDFLAGS = -X main.version=$(VERSION) -H=windowsgui

# Cross-compilation targets
BINARY_LINUX_AMD64   = DXHunter-linux-amd64
BINARY_DARWIN_AMD64  = DXHunter-darwin-amd64
BINARY_DARWIN_ARM64  = DXHunter-darwin-arm64

.PHONY: all build build-all frontend backend run clean dev help install-deps \
        build-linux build-darwin-amd64 build-darwin-arm64 \
        _bin-linux _bin-darwin-amd64 _bin-darwin-arm64

# Commande par défaut
all: build

## help: Affiche cette aide
help:
	@echo "DXHunter - Makefile"
	@echo ""
	@echo "Commandes disponibles:"
	@echo "  make build              - Build complet Windows (frontend + backend)"
	@echo "  make build-all          - Build pour toutes les plateformes"
	@echo "  make build-linux        - Build Linux amd64"
	@echo "  make build-darwin-amd64 - Build macOS Intel"
	@echo "  make build-darwin-arm64 - Build macOS Apple Silicon"
	@echo "  make frontend           - Build uniquement le frontend"
	@echo "  make backend            - Build uniquement le backend Go"
	@echo "  make run                - Build et lance l'application"
	@echo "  make dev                - Lance le frontend en mode dev"
	@echo "  make clean              - Nettoie les fichiers générés"
	@echo "  make install-deps       - Installe toutes les dépendances"
	@echo "  make help               - Affiche cette aide"

## install-deps: Installe les dépendances npm
install-deps:
	@echo "[1/2] Installation des dependances npm..."
	cd $(FRONTEND_DIR) && npm install
	@echo "Dependances installees"
	@echo ""
	@echo "[2/2] Verification de Go..."
	@go version
	@echo "Go est installe"

## frontend: Build le frontend Svelte
frontend:
	@echo "Building frontend..."
	cd $(FRONTEND_DIR) && npm run build
	@echo "Frontend built successfully"

## backend: Build le backend Go
backend:
	@echo "Building Go binary..."
	go build -o $(BINARY_NAME) -ldflags "$(LDFLAGS)" .
	@echo "Backend built successfully"

## backend: Build le backend Go
backendi: frontend
	@echo "Building Go binary..."
	go build -o $(BINARY_NAME) .
	@echo "Backend built successfully"

## build: Build complet (frontend + backend)
build: install-deps frontend backend
	@echo ""
	@echo "====================================="
	@echo "  BUILD COMPLETE!"
	@echo "====================================="
	@echo ""
	@echo "Run: ./$(BINARY_NAME)"
	@echo ""

## build: Build complet (frontend + backend)
buildi: install-deps frontend backendi
	@echo ""
	@echo "====================================="
	@echo "  BUILD COMPLETE!"
	@echo "====================================="
	@echo ""
	@echo "Run: ./$(BINARY_NAME)"
	@echo ""


## run: Build et lance l'application
run: build
	@echo "Starting DXHunter..."
	@echo ""
	./$(BINARY_NAME)

## dev: Lance le frontend en mode développement (hot reload)
dev:
	@echo "Starting frontend dev server..."
	@echo "Frontend: http://localhost:3000"
	@echo "Backend:  http://localhost:8080"
	@echo ""
	cd $(FRONTEND_DIR) && npm run dev

## clean: Nettoie les fichiers générés
clean:
	@echo "Cleaning build files..."
	@if exist $(BINARY_NAME)          del /f /q $(BINARY_NAME)
	@if exist $(BINARY_LINUX_AMD64)   del /f /q $(BINARY_LINUX_AMD64)
	@if exist $(BINARY_DARWIN_AMD64)  del /f /q $(BINARY_DARWIN_AMD64)
	@if exist $(BINARY_DARWIN_ARM64)  del /f /q $(BINARY_DARWIN_ARM64)
	@if exist $(DIST_DIR) rmdir /s /q $(DIST_DIR)
	@echo "Clean complete"

## build-linux: Cross-compile pour Linux amd64
build-linux: frontend _bin-linux

## build-darwin-amd64: Cross-compile pour macOS Intel
build-darwin-amd64: frontend _bin-darwin-amd64

## build-darwin-arm64: Cross-compile pour macOS Apple Silicon
build-darwin-arm64: frontend _bin-darwin-arm64

# Cibles internes — Go uniquement, sans rebuild du frontend
_bin-linux:
	@echo "Building Linux amd64..."
	cmd /C "set CGO_ENABLED=0&&set GOOS=linux&&set GOARCH=amd64&&go build -o $(BINARY_LINUX_AMD64) -ldflags \"-X main.version=$(VERSION)\" ."
	@echo "  -> $(BINARY_LINUX_AMD64)"

_bin-darwin-amd64:
	@echo "Building macOS Intel (amd64)..."
	cmd /C "set CGO_ENABLED=0&&set GOOS=darwin&&set GOARCH=amd64&&go build -o $(BINARY_DARWIN_AMD64) -ldflags \"-X main.version=$(VERSION)\" ."
	@echo "  -> $(BINARY_DARWIN_AMD64)"

_bin-darwin-arm64:
	@echo "Building macOS Apple Silicon (arm64)..."
	cmd /C "set CGO_ENABLED=0&&set GOOS=darwin&&set GOARCH=arm64&&go build -o $(BINARY_DARWIN_ARM64) -ldflags \"-X main.version=$(VERSION)\" ."
	@echo "  -> $(BINARY_DARWIN_ARM64)"

## build-all: Build pour toutes les plateformes (frontend compilé une seule fois)
build-all: frontend _bin-linux _bin-darwin-amd64 _bin-darwin-arm64 backend
	@echo ""
	@echo "====================================="
	@echo "  BUILD ALL COMPLETE!"
	@echo "====================================="
	@echo "  Windows : $(BINARY_NAME)"
	@echo "  Linux   : $(BINARY_LINUX_AMD64)"
	@echo "  macOS   : $(BINARY_DARWIN_AMD64)"
	@echo "  macOS M : $(BINARY_DARWIN_ARM64)"
	@echo "====================================="

## watch: Build auto lors des changements (nécessite watchexec)
watch:
	@echo "Watching for changes..."
	@echo "Install watchexec: choco install watchexec"
	watchexec -w . -e go -- make build