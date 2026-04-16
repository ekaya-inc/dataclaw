.DEFAULT_GOAL := none
SHELL := /bin/sh

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BINARY_NAME := dataclaw
BINARY_PATH := bin/$(BINARY_NAME)
EMBEDDED_UI_DIR := internal/uifs/dist

.PHONY: none build-ui build-binary build check run dev dev-ui

none: ## Show available targets
	@echo "DataClaw"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-12s %s\n", $$1, $$2}'

build-ui: ## Build ui/dist and refresh the embedded UI bundle
	@set -eu; \
	if [ ! -d ui/node_modules ]; then \
		echo "Installing UI dependencies..."; \
		npm --prefix ui install; \
	fi; \
	echo "Building embedded UI..."; \
	npm --prefix ui run build; \
	rm -rf "$(EMBEDDED_UI_DIR)"; \
	mkdir -p "$(EMBEDDED_UI_DIR)"; \
	cp -R ui/dist/. "$(EMBEDDED_UI_DIR)/"; \
	printf '%s\n%s\n' \
		'Placeholder so `//go:embed all:dist` resolves when no UI bundle has been built yet.' \
		'Run `make run` (or `make dev` + `make dev-ui`) to populate this directory.' \
		> "$(EMBEDDED_UI_DIR)/.gitkeep"

build-binary: ## Build the dataclaw binary to bin/dataclaw
	@set -eu; \
	mkdir -p "$(dir $(BINARY_PATH))"; \
	go build -trimpath -ldflags="-X main.Version=$(VERSION)" -o "$(BINARY_PATH)" .

build: build-ui build-binary ## Build the embedded UI and local binary

check: ## Run quiet backend and UI verification
	@set -eu; \
	run_step() { \
		label="$$1"; \
		shift; \
		log_file=$$(mktemp); \
		printf "%-24s" "$$label"; \
		if "$$@" >"$$log_file" 2>&1; then \
			rm -f "$$log_file"; \
			echo "ok"; \
		else \
			status=$$?; \
			echo "failed"; \
			cat "$$log_file"; \
			rm -f "$$log_file"; \
			exit "$$status"; \
		fi; \
	}; \
	run_step "go mod tidy" sh -c '\
		go_mod_backup=$$(mktemp); \
		go_sum_backup=$$(mktemp); \
		cp go.mod "$$go_mod_backup"; \
		cp go.sum "$$go_sum_backup"; \
		restore() { \
			mv "$$go_mod_backup" go.mod; \
			mv "$$go_sum_backup" go.sum; \
		}; \
		if ! go mod tidy; then \
			restore; \
			exit 1; \
		fi; \
		if ! cmp -s go.mod "$$go_mod_backup" || ! cmp -s go.sum "$$go_sum_backup"; then \
			restore; \
			echo "go.mod/go.sum are not tidy; run '\''go mod tidy'\''" >&2; \
			exit 1; \
		fi; \
		restore'; \
	run_step "gofmt" sh -c '\
		files=$$(find . -type f -name '\''*.go'\'' -print); \
		unformatted=""; \
		if [ -n "$$files" ]; then \
			unformatted=$$(gofmt -l $$files); \
		fi; \
		if [ -n "$$unformatted" ]; then \
			echo "$$unformatted"; \
			exit 1; \
		fi'; \
	run_step "ui deps" sh -c 'test -d ui/node_modules || npm --prefix ui install'; \
	run_step "ui lint" npm --prefix ui run lint; \
	run_step "ui typecheck" npm --prefix ui run typecheck; \
	run_step "ui test" npm --prefix ui test -- --run; \
	run_step "ui build" $(MAKE) build-ui; \
	run_step "go test" go test ./...; \
	run_step "go build" $(MAKE) build-binary; \
	echo ""; \
	echo "All checks passed."

run: ## Rebuild embedded assets if needed, then start the server
	@set -eu; \
	dist_index="internal/uifs/dist/index.html"; \
	needs_ui_build=0; \
	if [ ! -f "$$dist_index" ]; then \
		needs_ui_build=1; \
	else \
		if find ui/src -type f -newer "$$dist_index" | grep -q .; then \
			needs_ui_build=1; \
		fi; \
		for path in ui/index.html ui/package.json ui/package-lock.json ui/postcss.config.js ui/tailwind.config.js ui/tsconfig.json ui/vite.config.ts ui/eslint.config.js; do \
			if [ "$$path" -nt "$$dist_index" ]; then \
				needs_ui_build=1; \
				break; \
			fi; \
		done; \
	fi; \
	if [ "$$needs_ui_build" -eq 1 ]; then \
		$(MAKE) build-ui; \
	fi; \
	echo "Starting DataClaw..."; \
	exec go run .

dev: ## Start the Go server against ui/dist on disk (pair with `make dev-ui`)
	@DATACLAW_UI_DIR="$(CURDIR)/ui/dist" exec go run .

dev-ui: ## Watch ui/src and rebuild ui/dist on save (pair with `make dev`)
	@set -eu; \
	if [ ! -d ui/node_modules ]; then \
		echo "Installing UI dependencies..."; \
		npm --prefix ui install; \
	fi; \
	exec npm --prefix ui run build:watch
