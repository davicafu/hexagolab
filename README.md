# Microservicio con arquitectura hexagonal 

## Requisitos

### 1️⃣ Funcionales

- [ ] CRUD completo para:
    - Usuarios
    - Tareas

- [ ] Consultas avanzadas:
    - [ ] Filtrado, paginación (offset y cursor) y ordenamiento
    - [ ] Búsqueda de usuarios por nombre, mail, rango de edad
    - [ ] Obtener tareas de un usuario, usuario con más tareas, tareas sin asignar
    - [ ] Validaciones de entrada (prevención de: inyección SQL, búsqueda por columnas ocultas)
    - [ ] Manejo de errores claro y consistente
    - [ ] Autenticación y autorización básicas
    - [ ] Logging y métricas mínimas
    - [ ] Publicación de eventos relevantes:
        - UsuarioCreado, UsuarioModificado, UsuarioEliminado, TareaCreada, TareaActualizada, TareaFinalizada

### 2️⃣ Arquitectura hexagonal

#### Domain
Toda la lógica de negocio y las interfaces (ports) están aisladas.
- user.go y task.go contendrán solo entidades y reglas de negocio.
- ports.go contendrá interfaces para repositorio, cache y events. Esto permite testear la lógica de negocio con mocks sin depender de implementaciones concretas.
- Tests unitarios de dominio aislados.

#### Application
- Servicios que orquestan la lógica de negocio usando los ports.
- Los services (user_service.go, etc.) reciben interfaces como dependencias, lo que permite inyectar repositorios, cache y event publishers concretos.
- Aquí se implementa la coordinación de operaciones (por ejemplo, crear un usuario, guardar en cache y publicar evento).
- Uso del patrón Outbox Event para la publicación de eventos del dominio. Se prioriza garantarizar la no pérdida de eventos.
- Uso del patrón Criteria para la búsqueda genérica de usuario o tareas.

#### Infra (Adapters)
- Inbound: HTTP/gRPC/Events handlers. Los handlers HTTP solo llaman a los services de application, no conocen detalles de DB, cache o events.
- Outbound: implementan las interfaces definidas en ports.go. Permitirá cambiar tecnologías (Postgres → MySQL, Redis → Memcached, Kafka → RabbitMQ) sin tocar la lógica de negocio.
- Independencia de frameworks y librerías.

💡 Tip para implementación:

- Mantener las interfaces de ports puras en domain/ports.go.
- Nunca instanciar adapters concretos dentro del dominio o de application, hacerlo en main.go y pasarlos a los services.
- Para cache y events, implementar TTL y manejo de errores, así los services no fallan si el broker o Redis tienen un problema temporal.
- Utilizar goroutines para tareas que no deben bloquear al cliente (eventos, cache updates, logging).
- Utilizar channels como colas internas para desacoplar productores/consumidores (event bus, workers).
- Para consultas compuestas, aprovechar goroutines para paralelizar.

### 3️⃣ Cache

- [x] Guardar lecturas frecuentes (usuarios, tareas)
- [x] TTL configurable para los datos
- [x] Actualización de cache después de modificaciones

### 4️⃣ Event-driven

- [x] Publicación de eventos al crear o modificar entidades
- [x] Posibilidad de suscribirse a eventos de otros servicios

### 5️⃣ Buenas prácticas de programación

- 12 Factor App
    - Código base: un repositorio por microservicio
    - Dependencias: declaradas explícitamente (go.mod)
    - Configuración: mediante variables de entorno
    - Backing services: DB, Redis, broker como recursos adjuntos
    - Build, release, run: fases separadas
    - Procesos: stateless, ejecutables independientes
    - Port binding: exponer servicios HTTP y event consumers
    - Concurrency: soportar escalado horizontal
    - Disposability: iniciar y detener rápido
    - Dev/prod parity: entornos similares
    - Logs: flujos de eventos, no ficheros locales
    - Admin processes: tareas de administración como procesos independientes
- [x] Config y pkg: configuración centralizada y utilidades compartidas.
- [ ] Tests: 
    - [x] Unit tests: solo importa domain y application, usando mocks (stubs, mocks, fakes, dummy, spy) de adapters.
    - [ ] Integration tests: importan adapters concretos para probar DB, cache y eventos juntos.
- [ ] Documentación de API: OpenAPI / Swagger
- [-] Logs estructurados
- [ ] Manejo centralizado de errores
- [ ] Seguridad: sanitización de inputs, rate limiting, HTTPS
- [ ] CI/CD básico
- [ ] Monitorización mínima (Prometheus/Grafana)

### 6️⃣ Casos de Uso

##### Flujo ejemplo: Crear un Usuario

1. Cliente envía POST.
2. user_handler.go recibe la request y la transforma en un objeto User.
3. Llama a user_service.createUser(usuario).
    1. Guarda el usuario en repositorio (DB).
    2. Guarda en cache (Redis) para lecturas rápidas.
    3. Publica evento userCreated en broker.
4. Handler devuelve 201 Created al cliente.

##### Flujo ejemplo: Tarea asignada a un usuario

1. Crear tarea: TaskService recibe userID.
2. Guarda en DB → actualiza cache → publica evento taskCreated.
3. Otros servicios pueden suscribirse al evento y reaccionar (por ejemplo, notificación de nueva tarea).