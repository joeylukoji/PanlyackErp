.PHONY: dev test vet generate assets migrate-up integration
dev:
	go run ./cmd/erp
test:
	go test ./...
vet:
	go vet ./...
generate:
	go run ./scripts/generate-templates
	sqlc generate -f db/sqlc.yaml
assets:
	npm ci
	cp node_modules/htmx.org/dist/htmx.min.js web/static/htmx.min.js
	npm run build
migrate-up:
	go run ./cmd/migrate
integration:
	go test ./internal/workflows/invoiceposting -v
test-race:
	go test -race ./...
