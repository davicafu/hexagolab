# ==============================================================================
# Makefile para el proyecto Hexagolab
# ==============================================================================

# Variables para los paquetes, facilitan el mantenimiento
ALL_PACKAGES := $(shell go list ./...)
INTEGRATION_PACKAGES_PATTERN := %/integration/%
E2E_PACKAGES_PATTERN := %/e2e/%
CONTRACTS_PACKAGES_PATTERN := %/contacts/%
CONTRACTS_PACKAGES_PATTERN := %/internal/%

# 3. Lista de paquetes para cada tipo de test (usando 'filter' para seleccionar).
INTEGRATION_PACKAGES := $(filter $(INTEGRATION_PACKAGES_PATTERN), $(ALL_PACKAGES))
E2E_PACKAGES := $(filter $(E2E_PACKAGES_PATTERN), $(ALL_PACKAGES))
CONTRACTS_PACKAGES := $(filter $(CONTRACTS_PACKAGES_PATTERN), $(ALL_PACKAGES))
UNIT_PACKAGES := $(filter $(UNIT_PACKAGES_PATTERN), $(ALL_PACKAGES))

# .PHONY declara que estos son comandos, no archivos.
.PHONY: all tests unit-test integration-test e2e-test contracts-test coverage coverage-html build-proto clean build run

all: build

# ----------------- Construcción y Ejecución -----------------
build:
	@echo "🏗️  Construyendo binarios..."
	go build -o bin/api ./cmd/api/main.go
	go build -o bin/relayer ./cmd/outbox-relayer/main.go

run:
	@echo "🚀 Ejecutando la API..."
	go run ./cmd/api/main.go

# ----------------- Testing y Cobertura -----------------

# Ejecuta TODOS los tests del proyecto
tests:
	@echo "🧪 Ejecutando todos los tests (unitarios + integración + e2e + contracts)..."
	go test $(ALL_PACKAGES) -v

# Ejecuta SÓLO los tests unitarios
unit-test:
	@echo "🧪 Ejecutando tests unitarios..."
	go test $(UNIT_PACKAGES) -v -cover

# Ejecuta SÓLO los tests de integración
integration-test:
	@echo "🧪 Ejecutando tests de integración..."
	go test $(INTEGRATION_PACKAGES) -v

# Ejecuta SÓLO los tests End-to-End
e2e-test:
	@echo "🧪 Ejecutando tests End-to-End..."
	go test $(E2E_PACKAGES) -v

# Ejecuta SÓLO los tests de Contrato
contracts-test:
	@echo "🧪 Ejecutando tests de Contrato..."
	go test $(CONTRACTS_PACKAGES) -v

# Muestra la cobertura de tests en la terminal
coverage:
	@echo "📊 Calculando cobertura de tests..."
	go test $(ALL_PACKAGES) -coverprofile=coverage.out
	go tool cover -func=coverage.out

# Genera un informe de cobertura en formato HTML
coverage-html:
	@echo "📊 Generando informe de cobertura HTML..."
	go test $(ALL_PACKAGES) -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "👉 Abre coverage.html en tu navegador"

# ----------------- Herramientas y Limpieza -----------------

# Genera el código Go a partir de los archivos .proto
build-proto:
	@echo "🤖 Generando código gRPC..."
	protoc --go_out=. --go-grpc_out=. proto/*.proto

# Limpia los archivos generados
clean:
	@echo "🧹 Limpiando archivos generados..."
	rm -f coverage.out coverage.html
	rm -rf ./bin