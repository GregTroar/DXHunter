# Variables
BINARY_NAME=FlexDXCluster.exe
FRONTEND_DIR=frontend
DIST_DIR=$(FRONTEND_DIR)/dist
GO_FILES=$(shell find . -name '*.go' -not -path "./$(FRONTEND_DIR)/*")

.PHONY: all build frontend backend run clean dev help install-deps

# Commande par défaut
all: build

## help: Affiche cette aide
help:
	@echo "FlexDXCluster - Makefile"
	@echo ""
	@echo "Commandes disponibles:"
	@echo "  make build         - Build complet (frontend + backend)"
	@echo "  make frontend      - Build uniquement le frontend"
	@echo "  make backend       - Build uniquement le backend Go"
	@echo "  make run           - Build et lance l'application"
	@echo "  make dev           - Lance le frontend en mode dev"
	@echo "  make clean         - Nettoie les fichiers générés"
	@echo "  make install-deps  - Installe toutes les dépendances"
	@echo "  make help          - Affiche cette aide"

## install-deps: Installe les dépendances npm
install-deps:
	@echo "[1/2] Installation des dépendances npm..."
	cd $(FRONTEND_DIR) && npm install
	@echo "Dépendances installées"
	@echo ""
	@echo "[2/2] Vérification de Go..."
	@go version
	@echo "Go est installé"

## frontend: Build le frontend Svelte
frontend:
	@echo "Building frontend..."
	cd $(FRONTEND_DIR) && npm run build
	@echo "Frontend built successfully"

## backend: Build le backend Go
backend: frontend
	@echo "Building Go binary..."
	go build -ldflags -H=windowsgui .
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

## run: Build et lance l'application
run: build
	@echo "Starting FlexDXCluster..."
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
	@if exist $(BINARY_NAME) del /f /q $(BINARY_NAME)
	@if exist $(DIST_DIR) rmdir /s /q $(DIST_DIR)
	@echo "Clean complete"

## watch: Build auto lors des changements (nécessite watchexec)
watch:
	@echo "Watching for changes..."
	@echo "Install watchexec: choco install watchexec"
	watchexec -w . -e go -- make build