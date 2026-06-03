-include .env
export

.PHONY: dev-up dev-down migrate build run test spike lint

dev-up:
	docker compose up -d db
	@echo "Waiting for DB..." && sleep 3
	$(MAKE) migrate

dev-down:
	docker compose down

migrate:
	@psql "$(DATABASE_URL)" -c \
		"CREATE TABLE IF NOT EXISTS schema_migrations (filename TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now());" \
		> /dev/null
	@for f in internal/store/migrations/*.up.sql; do \
		name=$$(basename $$f); \
		applied=$$(psql "$(DATABASE_URL)" -tAc "SELECT 1 FROM schema_migrations WHERE filename='$$name'"); \
		if [ "$$applied" = "1" ]; then \
			echo "  skip $$name (already applied)"; \
		else \
			echo "  apply $$name..."; \
			psql "$(DATABASE_URL)" -f "$$f" || exit 1; \
			psql "$(DATABASE_URL)" -c "INSERT INTO schema_migrations(filename) VALUES('$$name');" > /dev/null; \
		fi \
	done

build:
	go build -o dist/papertrail ./cmd/papertrail

run: build
	./dist/papertrail $(ARGS)

spike:
	@if [ -z "$(PDF)" ]; then echo "Usage: make spike PDF=/path/to/file.pdf"; exit 1; fi
	go run ./cmd/papertrail spike --pdf "$(PDF)"

test:
	go test ./... -race -count=1

lint:
	golangci-lint run ./...
