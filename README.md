# Hexagolab: Microservicio con Arquitectura Hexagonal en Go 🚀

Este proyecto es una implementación de referencia de un microservicio en Go, diseñado siguiendo los principios de la **Arquitectura Hexagonal (Puertos y Adaptadores)** y estructurado como un **Monolito Modular**. Su propósito es servir como un ejemplo práctico de cómo construir aplicaciones robustas, escalables y fáciles de mantener.

---

## ✨ Características Principales

- ✅ **CRUD completo** para dos dominios de negocio independientes: **Usuarios** y **Tareas**.
- ✅ **API dual**: expone la funcionalidad a través de una API **REST (Gin)** y una API de alto rendimiento **gRPC**.
- ✅ **Sistema de eventos robusto** con el patrón **Transactional Outbox**, garantizando que nunca se pierdan eventos de dominio (`UserCreated`, `TaskCompleted`, etc.).
- ✅ **Adaptadores de infraestructura intercambiables**:
    - **Bases de Datos**: Soporte para PostgreSQL y SQLite.
    - **Caché**: Soporte para Redis y una caché en memoria.
    - **Bus de Eventos**: Soporte para Kafka y un bus en memoria basado en canales, ideal para desarrollo local.
- ✅ **Consultas avanzadas** mediante el **Patrón Criteria**, permitiendo filtrado, paginación (offset) y ordenamiento dinámico.
- ✅ **Tests completos**: Cobertura de tests unitarios (dominio), de componente (servicios con mocks) y de integración (con bases de datos reales).
- ✅ **Configuración centralizada** a través de variables de entorno, siguiendo las mejores prácticas de **12-Factor App**.
- ✅ **Logging estructurado** con `zap` para una mejor observabilidad.

---

## 🏛️ Arquitectura: Monolito Modular Hexagonal

El proyecto está organizado como un **Monolito Modular**, donde cada dominio de negocio (`user`, `task`) es un módulo autocontenido. La comunicación con el mundo exterior se gestiona a través de una capa de infraestructura centralizada, siguiendo la **Arquitectura Hexagonal**.

La regla fundamental es la **Inversión de Dependencias**: la infraestructura (`infra`) depende de las abstracciones del dominio (`domain`), pero el dominio nunca depende de la infraestructura.

### 🧱 Capas Principales

1.  **`shared/` (Contratos y Abstracciones)**
    - Contiene las interfaces (puertos) y los DTOs compartidos por toda la aplicación. Es el "plano" de la arquitectura.
    - **`platform/`**: Define los puertos de infraestructura (`EventPublisher`, `Cache`).
    - **`domain/`**: Define conceptos de dominio compartidos (`Criteria`, `OutboxEvent`).

2.  **`internal/` (El núcleo de la Aplicación)**
    - Esta carpeta contiene todo el código privado de la aplicación, organizado en módulos de dominio y una capa de infraestructura compartida.

    - Módulos de Dominio (internal/user, internal/task): Cada módulo es una "porción vertical" autocontenida que agrupa toda la lógica de negocio para una entidad específica.

        - **domain/**: La lógica de negocio pura. Contiene las entidades (User, Task), las reglas y las interfaces de repositorio (UserRepository).

        - **application/**: Los casos de uso. Contiene los Services que orquestan la lógica de negocio.

        - **infra/** (específica del Dominio): Contiene adaptadores que están íntimamente ligados a este dominio.

    - **internal/infra/** (Infraestructura Compartida):
    Contiene los adaptadores de infraestructura tecnológica y compartida que dan servicio a toda la aplicación. Aquí es donde residen las implementaciones concretas para tecnologías de propósito general como PostgreSQL (como el patrón Outbox), Kafka, Redis, el servidor web (Gin), gRPC, etc.

3.  **`cmd/` (Puntos de Entrada)**
    - Contiene los ejecutables (`main.go`). Su única responsabilidad es leer la configuración, construir todas las dependencias (el "ensamblaje") y arrancar la aplicación (el servidor HTTP, el `relayer` de outbox, etc.).

---

## 🚀 Cómo Empezar

### Prerrequisitos
- Go 1.24+
- Docker y Docker Compose si no se desea usar memoria (para ejecutar PostgreSQL, Kafka y Redis)
- `protoc` (para generar código gRPC)

### Configuración
1.  Copia el archivo de configuración de ejemplo:
    ```bash
    cp .env.example .env
    ```
2.  Revisa y ajusta las variables de entorno en el archivo `.env` según tu configuración local.

### Ejecutar la Aplicación
1.  Inicia los servicios de infraestructura (Postgres, Kafka, etc.):
    ```bash
    docker-compose up -d
    ```
2.  Ejecuta la aplicación principal (servidor API):
    ```bash
    go run ./cmd/api/main.go
    ```
3.  Ejecuta el `relayer` del Outbox en una terminal separada:
    ```bash
    go run ./cmd/outbox-relayer/main.go
    ```

## 🛠️ Comandos de Desarrollo (Makefile)
Este proyecto utiliza un Makefile para automatizar las tareas de desarrollo más comunes. Abre una terminal en la raíz del proyecto y ejecuta los siguientes comandos:

### Compilación y Ejecución
`make build`: Compila los binarios de la aplicación (api y relayer) en la carpeta bin/.

`make run`: Ejecuta la aplicación principal (el servidor API).

### Testing
`make tests`: Ejecuta todos los tests del proyecto (unitarios y de integración).

`make unit-test`: Ejecuta solo los tests unitarios, que son rápidos y no requieren dependencias externas.

`make integration-test`: Ejecuta solo los tests de integración, que prueban la conexión con bases de datos reales (requiere tener Docker corriendo).

### Cobertura de Código
`make coverage`: Calcula la cobertura de los tests y muestra un resumen por función en la terminal.

`make coverage-html`: Genera un informe visual de la cobertura en un archivo coverage.html. Ábrelo en tu navegador para ver qué líneas de código están cubiertas.

### Herramientas Adicionales
`make build-proto`: Genera (o regenera) el código Go a partir de los archivos .proto para gRPC.

`make clean`: Elimina todos los archivos generados por la compilación y los tests (bin/, coverage.out, coverage.html).

### Ejemplos de uso
```bash
# Para ejecutar solo los tests unitarios
make unit-test

# Para generar el informe de cobertura y abrirlo
make coverage-html