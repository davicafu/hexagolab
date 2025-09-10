# Microservicio con arquitectura hexagonal 

## Requisitos

### 1️⃣ Funcionales

- [ ] CRUD completo para:
    - Usuarios
    - Coches (cada coche pertenece a un usuario)
    - Tareas (asociadas a usuarios y coches)

- [ ] Consultas avanzadas:
    - [ ] Filtrado, paginación y ordenamiento
    - [ ] Obtener coches de un usuario, tareas de un coche y usuario
    - [ ] Validaciones de entrada
    - [ ] Manejo de errores claro y consistente
    - [ ] Autenticación y autorización básicas
    - [ ] Logging y métricas mínimas
    - [ ] Publicación de eventos relevantes:
        - UsuarioCreado, CocheAsignado, TareaActualizada

### 2️⃣ Arquitectura hexagonal

#### Domain
Toda la lógica de negocio y las interfaces (ports) están aisladas.
- user.go, car.go y task.go contendrán solo entidades y reglas de negocio.
- ports.go contendrá interfaces para repositorio, cache y events. Esto permite testear la lógica de negocio con mocks sin depender de implementaciones concretas.

#### Application
- Servicios que orquestan la lógica de negocio usando los ports.
- Los services (user_service.go, etc.) reciben interfaces como dependencias, lo que permite inyectar repositorios, cache y event publishers concretos.
- Aquí se implementa la coordinación de operaciones (por ejemplo, crear un usuario, guardar en cache y publicar evento).

#### Adapters 
- Inbound: HTTP/gRPC handlers. Los handlers HTTP solo llaman a los services de application, no conocen detalles de DB, cache o events.
- Outbound: implementan las interfaces definidas en ports.go. Permitirá cambiar tecnologías (Postgres → MySQL, Redis → Memcached, Kafka → RabbitMQ) sin tocar la lógica de negocio.
- Independencia de frameworks y librerías.
- Tests unitarios de dominio aislados.

💡 Tip para implementación:

- Mantener las interfaces de ports puras en domain/ports.go.
- Nunca instanciar adapters concretos dentro del dominio o de application, hacerlo en main.go y pasarlos a los services.
- Para cache y events, implementar TTL y manejo de errores, así los services no fallan si el broker o Redis tienen un problema temporal.
- Utilizar goroutines para tareas que no deben bloquear al cliente (eventos, cache updates, logging).
- Utilizar channels como colas internas para desacoplar productores/consumidores (event bus, workers).
- Para consultas compuestas, aprovechar goroutines para paralelizar.

### 3️⃣ Cache

- [ ] Guardar lecturas frecuentes (usuarios, coches)
- [ ] TTL configurable para los datos
- [ ] Actualización de cache después de modificaciones

### 4️⃣ Event-driven

- [ ] Publicación de eventos al crear o modificar entidades
- [ ] Posibilidad de suscribirse a eventos de otros servicios

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
- [ ] Config y pkg: configuración centralizada y utilidades compartidas.
- [ ] Tests: 
    - [ ] Unit tests: solo importa domain y application, usando mocks (stubs, mocks, fakes, dummy, spy) de adapters.
    - [ ] Integration tests: importan adapters concretos para probar DB, cache y eventos juntos.
- [ ] Documentación de API: OpenAPI / Swagger
- [ ] Logs estructurados
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

##### Flujo ejemplo: Obtener un Coche

1. Cliente envía GET /cars/{id}.
2. car_handler.go llama a car_service.get_car(id).
    1. Intenta obtener el coche desde cache (Redis).
    2. Si cache miss → obtiene de DB, luego actualiza la cache.
    3. Devuelve la entidad al handler → respuesta al cliente.

##### Flujo ejemplo: Tarea asignada a un coche y usuario

1. Crear tarea: TaskService recibe userID y carID.
2. Guarda en DB → actualiza cache → publica evento TareaCreada.
3. Otros servicios pueden suscribirse al evento y reaccionar (por ejemplo, notificación de nueva tarea).

### 7️⃣ Algoritmos