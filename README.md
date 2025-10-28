# Hexagolab: Microservice with Hexagonal Architecture in Go 🚀

This project is a reference implementation of a microservice in Go, designed following the principles of Hexagonal Architecture (Ports and Adapters) and structured as a Modular Monolith. Its purpose is to serve as a practical example of how to build robust, scalable and maintainable applications.

---

## ✨ Main Features

- ✅ **Full CRUD** for two independent business domains: **Users** and **Tasks**.
- ✅ Dual API: exposes functionality through both a **REST API (Gin)** and a high-performance **gRPC API**.
- ✅ Robust event system using the **Transactional Outbox pattern**, ensuring domain events (UserCreated, TaskCompleted, etc.) are never lost.
- ✅ Interchangeable infrastructure adapters:
    - Databases: Support for PostgreSQL and SQLite.
    - Cache: Support for Redis and an in-memory cache.
    - Event Bus: Support for Kafka and an in-memory channel-based bus, ideal for local development.
- ✅ Advanced querying via the **Criteria Pattern**, enabling filtering, pagination (offset and cursor) and dynamic sorting.
- ✅ **Comprehensive tests**: Unit tests (domain), component tests (services with mocks) *and integration tests (with real databases)*.
- ✅ **Centralized configuration** through environment variables, following 12-Factor App best practices.
- ✅ **Structured logging** with zap for better observability.

##### Note: Italic -> TODO
---

## 🏛️ Architecture: Modular Hexagonal Monolith
The project is organized as a Modular Monolith, where each business domain (`user`, `task`) is a self-contained module. Communication with the outside world is handled through a centralized infrastructure layer, following the **Hexagonal Architecture**.

The fundamental rule is Dependency Inversion: infrastructure (`infra`) depends on domain abstractions (`domain`), but the domain never depends on infrastructure.

### 🧱 Main Layers

1.  **`shared/` (Contracts and Abstractions)**
    - Contains interfaces (ports) and DTOs shared across the application. It serves as the "blueprint" of the architecture.
    - **`platform/`**: Defines infrastuture ports (`EventPublisher`, `Cache`).
    - **`domain/`**: Defines shared domain concepts (`Criteria`, `OutboxEvent`).

2.  **`internal/` (Core App)**
    - This folder contains all private application code, organized into domain modules and a shared infrastructure layer.

    - Domain Modules (internal/user, internal/task): Each module is a self-contained "vertical slice" that groups all business logic for a specific entity.
        - `domain/`: Pure business logic. Contains entities (User, Task), rules, and repository interfaces (UserRepository).
        - `application/`: Use cases. Contains Services that orchestrate business logic.
        - `infra/` (domain-specific): Contains adapters tightly coupled to that domain.
        - `internal/infra/` (Shared Infrastructure): contains technology-agnostic infrastructure adapters that serve the entire application. This is where concrete implementations for general-purpose technologies like PostgreSQL (including the Outbox pattern), Kafka, Redis, the web server (Gin), gRPC, etc., reside.

3.  **`cmd/` (Entry Points)**
    - Contains the executables (`main.go`). Their only responsibility is to read configuration, build all dependencies (the “assembly”), and start the application (HTTP server, outbox relayer, etc.).

---

## 🚀 How to start

### Prerequisites
- Go 1.24+
- Docker and Docker Compose (if you prefer not to use in-memory implementations for PostgreSQL, Kafka, and Redis)
- `protoc` (to generate gRPC code)

### Configuration
NOTE: TODO — configuration is currently loaded in code.
1.  Copy the example configuration file:
    ```bash
    cp .env.example .env
    ```
2.  Review and adjust the environment variables in `.env` according to your local setup.

### Run Application
1. Start the infrastructure services (Postgres, Kafka, etc. if you need):
    ```bash
    docker-compose up -d
    ```
2.  Run the main application (API server):
    ```bash
    go run ./cmd/api/main.go
    ```
3.  Run the `Outbox relayer` in a separate terminal(pending to TODO, rith now runs with API Server):
    ```bash
    go run ./cmd/outbox-relayer/main.go
    ```

## 🛠️ Development Commands (Makefile)
This project uses a Makefile to automate common development tasks. Open a terminal at the project root and run the following commands:

### Build and Run
`make build`: Compiles the application binaries (api and relayer) into the `bin/` folder.

`make run`: Runs the main application (API server).

### Testing
`make tests`: Runs all project tests (unit, contracts, e2e and integration).

`make contract-test`: Runs only the contract tests.

`make unit-test`: Runs only the unit tests, which are fast and require no external dependencies.

`make integration-test`: Runs only the integration tests, which test mocks database connections.

`make e2e-test`: Runs only the integration tests, which test real database connections (requires Docker running).

### Coverage Code
`make coverage`: Calculates test coverage and displays a summary by function in the terminal.

`make coverage-html`: Generates a visual coverage report as `coverage.html`. Open it in your browser to see which lines are covered.

### Additional Tools
`make build-proto`: Generates (or regenerates) Go code from `.proto` files for gRPC.

`make clean`: Removes all build and test-generated files (`bin/`, `coverage.out`, `coverage.html`).

### Examples
```bash
# Run only unit tests
make unit-test

# Generate and open the coverage report
make coverage-html