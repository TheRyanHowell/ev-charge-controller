.PHONY: start dev stop logs test-api test-ui test-e2e test-e2e-chromium test-e2e-firefox test-e2e-chromium-stateless test-e2e-chromium-stateful test-e2e-firefox-stateless test-e2e-firefox-stateful test-e2e-ui build-ui test-mock test-race test-race-mock cover-api cover-ui cover-mock cover-all clean lint vet deadcode deadcode-api deadcode-mock deadcode-ui lint-api lint-ui lint-mock lint-e2e vet-api vet-mock fix fix-api fix-mock fix-ui fix-e2e create-user seed seed-no-clear test-api-file test-ui-file test-e2e-file test-e2e-chromium-file test-e2e-firefox-file

UID  := $(shell id -u)
GID  := $(shell id -g)
COMPOSE_FLAGS := --env-file .env
export UID GID

HTTPS_ENABLED := $(shell grep '^ENABLE_HTTPS=' .env 2>/dev/null | cut -d= -f2-)
ifeq ($(HTTPS_ENABLED),true)
COMPOSE_PROD_FILES := -f docker-compose.yml -f docker-compose.caddy.yml
else
COMPOSE_PROD_FILES := -f docker-compose.yml -f docker-compose.no-caddy.yml
endif

# DEADCODE_CHECK runs the reachability-based dead-code analyzer (test code as
# roots) and fails the build if it reports anything. Pinned for reproducibility.
DEADCODE_VERSION := v0.46.0
DEADCODE_CHECK := out=$$(go run golang.org/x/tools/cmd/deadcode@$(DEADCODE_VERSION) -test ./...); if [ -n "$$out" ]; then echo "Dead code detected:"; echo "$$out"; exit 1; fi

start:             ## Start API + UI (+ Caddy HTTPS if ENABLE_HTTPS=true)
	docker compose ${COMPOSE_FLAGS} ${COMPOSE_PROD_FILES} up --build -d

dev:               ## Start API + UI + mock Tasmota (development mode)
	docker compose ${COMPOSE_FLAGS} -f docker-compose.yml -f docker-compose.dev.yml up --build -d

stop:              ## Stop all containers
	docker compose ${COMPOSE_FLAGS} ${COMPOSE_PROD_FILES} down --remove-orphans

logs:              ## Follow logs from all containers
	docker compose ${COMPOSE_FLAGS} ${COMPOSE_PROD_FILES} logs -f

test-api:          ## Run Go API tests in container
	docker compose ${COMPOSE_FLAGS} -f docker-compose.yml -f docker-compose.dev.yml exec api go test ./...

test-ui:           ## Run Next.js unit tests in container
	docker compose ${COMPOSE_FLAGS} -f docker-compose.yml -f docker-compose.dev.yml exec ui npx vitest run

build-ui:          ## Verify Next.js production build (catches type errors, duplicate exports)
	docker compose ${COMPOSE_FLAGS} -f docker-compose.yml -f docker-compose.dev.yml exec ui npm run build

test-race:         ## Run Go API tests under the race detector (concurrency safety)
	docker compose ${COMPOSE_FLAGS} -f docker-compose.yml -f docker-compose.dev.yml exec api go test -race ./...

test-race-mock:    ## Run mock-tasmota tests under the race detector
	docker compose ${COMPOSE_FLAGS} -f docker-compose.yml -f docker-compose.dev.yml exec mock-tasmota go test -race ./...

cover-api:         ## Run Go API tests with coverage report in container
	docker compose ${COMPOSE_FLAGS} -f docker-compose.yml -f docker-compose.dev.yml exec api sh -c 'go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out'

cover-ui:          ## Run Next.js unit tests with coverage report in container
	docker compose ${COMPOSE_FLAGS} -f docker-compose.yml -f docker-compose.dev.yml exec ui npx vitest run --coverage

cover-all:         ## Run both Go and UI coverage reports
	$(MAKE) -j cover-api cover-ui

test-mock:         ## Run mock-tasmota tests in container
	docker compose ${COMPOSE_FLAGS} -f docker-compose.yml -f docker-compose.dev.yml exec mock-tasmota go test -v ./...

cover-mock:        ## Run mock-tasmota tests with coverage report in container
	docker compose ${COMPOSE_FLAGS} -f docker-compose.yml -f docker-compose.dev.yml exec mock-tasmota sh -c 'go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out'

clean:             ## Remove containers, volumes, images
	docker compose ${COMPOSE_FLAGS} ${COMPOSE_PROD_FILES} down -v --rmi all --remove-orphans

lint:              ## Run all linters (Go + UI + E2E)
	$(MAKE) -j lint-api lint-mock lint-ui lint-e2e

lint-api:          ## Run golangci-lint on Go API
	docker compose ${COMPOSE_FLAGS} -f docker-compose.yml -f docker-compose.dev.yml exec api golangci-lint run ./...

lint-ui:           ## Run Prettier + ESLint + knip (unused code) on UI
	docker compose ${COMPOSE_FLAGS} -f docker-compose.yml -f docker-compose.dev.yml exec ui sh -c 'npm run lint:prettier && npm run lint && npm run knip'

lint-mock:         ## Run golangci-lint on mock-tasmota
	docker compose ${COMPOSE_FLAGS} -f docker-compose.yml -f docker-compose.dev.yml exec mock-tasmota golangci-lint run ./...

lint-e2e:          ## Run Prettier + ESLint + TypeScript check on E2E tests
	docker compose ${COMPOSE_FLAGS} -f docker-compose.yml -f docker-compose.dev.yml run --rm e2e sh -c 'npm run lint:prettier && npm run lint && npm run typecheck'

deadcode:          ## Detect dead/unused code (Go API + mock + UI); fails if any found
	$(MAKE) -j deadcode-api deadcode-mock deadcode-ui

deadcode-api:      ## Detect dead code in Go API (golang.org/x/tools deadcode)
	docker compose ${COMPOSE_FLAGS} -f docker-compose.yml -f docker-compose.dev.yml exec -e GOSUMDB=off -e GOFLAGS=-mod=mod api sh -c '$(DEADCODE_CHECK)'

deadcode-mock:     ## Detect dead code in mock-tasmota
	docker compose ${COMPOSE_FLAGS} -f docker-compose.yml -f docker-compose.dev.yml exec -e GOSUMDB=off -e GOFLAGS=-mod=mod mock-tasmota sh -c '$(DEADCODE_CHECK)'

deadcode-ui:       ## Detect unused files/exports/types in UI (knip)
	docker compose ${COMPOSE_FLAGS} -f docker-compose.yml -f docker-compose.dev.yml exec ui npm run knip

vet:               ## Run go vet on all Go services
	$(MAKE) -j vet-api vet-mock

vet-api:           ## Run go vet on Go API
	docker compose ${COMPOSE_FLAGS} -f docker-compose.yml -f docker-compose.dev.yml exec api sh -c 'go vet ./...'

vet-mock:          ## Run go vet on mock-tasmota
	docker compose ${COMPOSE_FLAGS} -f docker-compose.yml -f docker-compose.dev.yml exec mock-tasmota sh -c 'go vet ./...'

fix:               ## Auto-fix all linter issues (Go + UI + E2E)
	$(MAKE) -j fix-api fix-mock fix-ui fix-e2e

fix-api:           ## Auto-fix Go API lint issues (golangci-lint --fix)
	docker compose ${COMPOSE_FLAGS} -f docker-compose.yml -f docker-compose.dev.yml exec api golangci-lint run --fix ./...

fix-mock:          ## Auto-fix mock-tasmota lint issues (golangci-lint --fix)
	docker compose ${COMPOSE_FLAGS} -f docker-compose.yml -f docker-compose.dev.yml exec mock-tasmota golangci-lint run --fix ./...

fix-ui:            ## Auto-fix UI lint issues (Prettier --write, ESLint --fix)
	docker compose ${COMPOSE_FLAGS} -f docker-compose.yml -f docker-compose.dev.yml exec ui npm run lint:fix

fix-e2e:           ## Auto-fix E2E lint issues (Prettier --write, ESLint --fix)
	docker compose ${COMPOSE_FLAGS} -f docker-compose.yml -f docker-compose.dev.yml run --rm e2e npm run lint:fix

create-user:       ## Create initial user: make create-user EMAIL=you@example.com
	docker compose ${COMPOSE_FLAGS} -f docker-compose.yml -f docker-compose.dev.yml exec -it api go run cmd/createuser/main.go -email "$(EMAIL)"

seed:              ## Clear all data and seed with comprehensive test dataset
	docker compose ${COMPOSE_FLAGS} -f docker-compose.yml -f docker-compose.dev.yml exec api go run ./cmd/seed

seed-no-clear:     ## Seed test data without clearing existing data
	docker compose ${COMPOSE_FLAGS} -f docker-compose.yml -f docker-compose.dev.yml exec api go run ./cmd/seed -clear=false

test-e2e:          ## Run Playwright E2E tests: stateless (both browsers, parallel) then stateful (both browsers, serial)
	$(MAKE) test-e2e-stateless
	$(MAKE) test-e2e-stateful

test-e2e-stateless: ## Run stateless E2E tests (Chromium + Firefox in parallel)
	docker compose ${COMPOSE_FLAGS} -f docker-compose.yml -f docker-compose.dev.yml run --rm e2e npx playwright test --project=chromium-stateless --project=firefox-stateless

test-e2e-stateful: ## Run stateful E2E tests (Chromium + Firefox, workers=1 to protect shared DB)
	docker compose ${COMPOSE_FLAGS} -f docker-compose.yml -f docker-compose.dev.yml run --rm e2e npx playwright test --project=chromium-stateful --project=firefox-stateful --workers=1

test-e2e-chromium: ## Run Playwright E2E tests (Chromium only, stateless then stateful)
	$(MAKE) test-e2e-chromium-stateless
	$(MAKE) test-e2e-chromium-stateful

test-e2e-firefox:  ## Run Playwright E2E tests (Firefox only, stateless then stateful)
	$(MAKE) test-e2e-firefox-stateless
	$(MAKE) test-e2e-firefox-stateful

test-e2e-chromium-stateless: ## Run stateless E2E tests (Chromium only)
	docker compose ${COMPOSE_FLAGS} -f docker-compose.yml -f docker-compose.dev.yml run --rm e2e npx playwright test --project=chromium-stateless

test-e2e-chromium-stateful: ## Run stateful E2E tests (Chromium only, workers=1 to protect shared DB)
	docker compose ${COMPOSE_FLAGS} -f docker-compose.yml -f docker-compose.dev.yml run --rm e2e npx playwright test --project=chromium-stateful --workers=1

test-e2e-firefox-stateless: ## Run stateless E2E tests (Firefox only)
	docker compose ${COMPOSE_FLAGS} -f docker-compose.yml -f docker-compose.dev.yml run --rm e2e npx playwright test --project=firefox-stateless

test-e2e-firefox-stateful: ## Run stateful E2E tests (Firefox only, workers=1 to protect shared DB)
	docker compose ${COMPOSE_FLAGS} -f docker-compose.yml -f docker-compose.dev.yml run --rm e2e npx playwright test --project=firefox-stateful --workers=1

test-e2e-ui:       ## Run Playwright E2E tests in interactive UI mode
	docker compose ${COMPOSE_FLAGS} -f docker-compose.yml -f docker-compose.dev.yml run --rm e2e npx playwright test --ui

# Individual test file targets (pass FILE= with one or more space-separated paths relative to service root)
# Examples:
#   make test-api-file FILE=./services/vehicle_service_test.go
#   make test-api-file FILE=./services/vehicle_service_test.go ./handlers/vehicle_handler_test.go
#   make test-ui-file FILE=src/app/vehicles/page.test.tsx
#   make test-ui-file FILE=src/app/vehicles/page.test.tsx src/app/vehicles/VehicleDetailClient.test.tsx
#   make test-e2e-file FILE=tests/stateless/vehicles.spec.ts
#   make test-e2e-file FILE=tests/stateful/stateful-dashboard.spec.ts
test-api-file:     ## Run Go test file(s): make test-api-file FILE=./path/to/test.go [./path2/test.go ...]
	docker compose ${COMPOSE_FLAGS} -f docker-compose.yml -f docker-compose.dev.yml exec api go test -v $(FILE)

test-ui-file:      ## Run Vitest test file(s): make test-ui-file FILE=src/path/to/test.tsx [src/path2/test.tsx ...]
	docker compose ${COMPOSE_FLAGS} -f docker-compose.yml -f docker-compose.dev.yml exec ui npx vitest run $(FILE)

test-e2e-file:     ## Run Playwright test file(s): make test-e2e-file FILE=tests/stateless/spec.spec.ts [tests/stateful/spec2.spec.ts ...]
	docker compose ${COMPOSE_FLAGS} -f docker-compose.yml -f docker-compose.dev.yml run --rm e2e npx playwright test $(FILE)

test-e2e-chromium-file: ## Run Playwright test file(s) (Chromium only): make test-e2e-chromium-file FILE=tests/stateless/spec.spec.ts [tests/stateful/spec2.spec.ts ...]
	docker compose ${COMPOSE_FLAGS} -f docker-compose.yml -f docker-compose.dev.yml run --rm e2e npx playwright test --project=chromium-stateless --project=chromium-stateful $(FILE)

test-e2e-firefox-file:  ## Run Playwright test file(s) (Firefox only): make test-e2e-firefox-file FILE=tests/stateless/spec.spec.ts [tests/stateful/spec2.spec.ts ...]
	docker compose ${COMPOSE_FLAGS} -f docker-compose.yml -f docker-compose.dev.yml run --rm e2e npx playwright test --project=firefox-stateless --project=firefox-stateful $(FILE)
