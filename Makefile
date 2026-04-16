.DEFAULT_GOAL := none
SHELL := /bin/sh

.PHONY: none check

none: ## Show available targets
	@echo "DataClaw"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-12s %s\n", $$1, $$2}'

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
	run_step "go test" go test ./...; \
	run_step "go build" go build ./...; \
	run_step "ui deps" sh -c 'test -d ui/node_modules || npm --prefix ui install'; \
	run_step "ui lint" npm --prefix ui run lint; \
	run_step "ui typecheck" npm --prefix ui run typecheck; \
	run_step "ui test" npm --prefix ui test -- --run; \
	run_step "ui build" npm --prefix ui run build; \
	echo ""; \
	echo "All checks passed."
