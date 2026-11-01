# torq — see specs/001-torq-core-platform/quickstart.md
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
DATA_DIR ?= ./.data

# Optional local settings for `make dev`, e.g. GEMINI_API_KEY=… (one KEY=value
# per line, no quotes). The file is git-ignored.
-include .env
export $(shell sed -n 's/^\([A-Za-z_][A-Za-z0-9_]*\)=.*/\1/p' .env 2>/dev/null)

.PHONY: setup generate dev dev-api dev-web build-web build check test e2e docker clean catalog

setup:
	go mod download
	npm ci --prefix web

generate:
	sqlc generate

# Runs the Go API (:8080) and the Vite dev server (:5173, proxies /api) together.
dev:
	@trap 'kill 0' INT TERM EXIT; \
	TORQ_DATA_DIR=$(DATA_DIR) TORQ_LOG_FORMAT=text go run ./cmd/torq & \
	npm run dev --prefix web & \
	wait

build-web:
	npm run build --prefix web

build: build-web
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o bin/torq ./cmd/torq

check:
	go vet ./...
	golangci-lint run ./...
	sqlc diff
	npm run typecheck --prefix web
	npm run lint --prefix web
	npm run i18n:check --prefix web

test:
	go test ./...
	npm test --prefix web -- --run

# End-to-end suite: starts bin/torq on a temp data dir, or targets TORQ_E2E_URL.
e2e: build
	npm run e2e --prefix web

# Maintainers only: regenerates the embedded vehicle catalog from downloaded source
# files (specs/002-vehicle-catalog-lookup/quickstart.md). Never run by builds or CI.
#   FZ6       KBA FZ 6 workbook (.xlsx), dl-de/by-2-0
#   GOR3A     gor3a/vehicle-makes-models data/json/all.json at GOR3A_SHA, ODbL 1.0
#   RETRIEVED retrieval date of the source files (YYYY-MM-DD)
#   SV42      optional: KBA SV 4.2 converted with `pdftotext -layout` (poppler).
#             SV 4.2 is "all rights reserved"; only use it with written KBA
#             consent and set SV42_CONSENT=yes.
catalog:
	@test -n "$(FZ6)" -a -n "$(GOR3A)" -a -n "$(GOR3A_SHA)" -a -n "$(RETRIEVED)" || \
		{ echo "usage: make catalog FZ6=fz6.xlsx GOR3A=all.json GOR3A_SHA=<commit> RETRIEVED=YYYY-MM-DD [SV42=sv42.txt SV42_CONSENT=yes]"; exit 2; }
	go run ./cmd/catalog-sync --fz6-xlsx "$(FZ6)" --gor3a-json "$(GOR3A)" --gor3a-commit "$(GOR3A_SHA)" \
		--retrieved "$(RETRIEVED)" $(if $(SV42),--sv42-text "$(SV42)" --sv42-consent="$(SV42_CONSENT)") \
		--out internal/catalog/data/catalog_seed.json.gz

docker:
	docker build --build-arg VERSION=$(VERSION) -t torq:$(VERSION) -t torq:latest .

clean:
	rm -rf bin web/dist/assets web/dist/index.html web/dist/favicon.svg
