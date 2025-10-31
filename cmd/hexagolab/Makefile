# ==============================================================================
# Makefile for the Hexagolab project
# ==============================================================================

# Variables for packages facilitate maintenance
ALL_PACKAGES := $(shell go list ./...)
INTEGRATION_PACKAGES_PATTERN := %/integration/%
E2E_PACKAGES_PATTERN := %/e2e/%
CONTRACTS_PACKAGES_PATTERN := %/contacts/%
CONTRACTS_PACKAGES_PATTERN := %/internal/%

# List of packages for each type of test (using ‘filter’ to select).
INTEGRATION_PACKAGES := $(filter $(INTEGRATION_PACKAGES_PATTERN), $(ALL_PACKAGES))
E2E_PACKAGES := $(filter $(E2E_PACKAGES_PATTERN), $(ALL_PACKAGES))
CONTRACTS_PACKAGES := $(filter $(CONTRACTS_PACKAGES_PATTERN), $(ALL_PACKAGES))
UNIT_PACKAGES := $(filter $(UNIT_PACKAGES_PATTERN), $(ALL_PACKAGES))

# .PHONY declares that these are commands, not files.
.PHONY: all tests unit-test integration-test e2e-test contracts-test coverage coverage-html build-proto clean build run

all: build

# ----------------- Build and Run -----------------
build:
	@echo "🏗️  Construyendo binarios..."
	go build -o bin/api ./cmd/api/main.go
	go build -o bin/relayer ./cmd/outbox-relayer/main.go

run:
	@echo "🚀 Ejecutando la API..."
	go run ./cmd/api/main.go

# ----------------- Testing and Cover -----------------

# Run ALL tests in the project
tests:
	@echo "🧪 Ejecutando todos los tests (unitarios + integración + e2e + contracts)..."
	go test $(ALL_PACKAGES) -v

# Run ONLY the unit tests
unit-test:
	@echo "🧪 Ejecutando tests unitarios..."
	go test $(UNIT_PACKAGES) -v -cover

# Run ONLY the integration tests
integration-test:
	@echo "🧪 Ejecutando tests de integración..."
	go test $(INTEGRATION_PACKAGES) -v

# Run ONLY the End-to-End tests
e2e-test:
	@echo "🧪 Ejecutando tests End-to-End..."
	go test $(E2E_PACKAGES) -v

# Run ONLY the contract tests
contracts-test:
	@echo "🧪 Ejecutando tests de Contrato..."
	go test $(CONTRACTS_PACKAGES) -v

# Display test coverage in the terminal
coverage:
	@echo "📊 Calculando cobertura de tests..."
	go test $(ALL_PACKAGES) -coverprofile=coverage.out
	go tool cover -func=coverage.out

# Generate a coverage report in HTML format
coverage-html:
	@echo "📊 Generando informe de cobertura HTML..."
	go test $(ALL_PACKAGES) -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "👉 Abre coverage.html en tu navegador"

# ----------------- Tools and Clean -----------------

# Generate Go code from .proto files
build-proto:
	@echo "🤖 Generando código gRPC..."
	protoc --go_out=. --go-grpc_out=. proto/*.proto

# Clean up the generated files
clean:
	@echo "🧹 Limpiando archivos generados..."
	rm -f coverage.out coverage.html
	rm -rf ./bin