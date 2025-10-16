package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	config "github.com/davicafu/hexagolab/internal/config"
	"github.com/davicafu/hexagolab/internal/infra/db/postgres"
	"github.com/davicafu/hexagolab/internal/infra/db/sqlite"
	infraEvents "github.com/davicafu/hexagolab/internal/infra/events"
	infraRelayer "github.com/davicafu/hexagolab/internal/infra/relayer"
	taskApp "github.com/davicafu/hexagolab/internal/task/application"
	taskDomain "github.com/davicafu/hexagolab/internal/task/domain"
	taskEvents "github.com/davicafu/hexagolab/internal/task/infra/inbound/events"
	taskHttp "github.com/davicafu/hexagolab/internal/task/infra/inbound/http"
	taskRepo "github.com/davicafu/hexagolab/internal/task/infra/outbound/db/postgre"
	userApp "github.com/davicafu/hexagolab/internal/user/application"
	userDomain "github.com/davicafu/hexagolab/internal/user/domain"
	userEvents "github.com/davicafu/hexagolab/internal/user/infra/inbound/events"
	userHttp "github.com/davicafu/hexagolab/internal/user/infra/inbound/http"
	userCache "github.com/davicafu/hexagolab/internal/user/infra/outbound/cache"
	userRepo "github.com/davicafu/hexagolab/internal/user/infra/outbound/db/sqlite"
	"github.com/google/uuid"

	"github.com/davicafu/hexagolab/pkg/logger"
	sharedEvents "github.com/davicafu/hexagolab/shared/events"
	sharedBus "github.com/davicafu/hexagolab/shared/platform/bus"
	sharedCache "github.com/davicafu/hexagolab/shared/platform/cache"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	// _ "github.com/mattn/go-sqlite3" // requires gcc
	_ "modernc.org/sqlite"
)

// ---------------- Main ----------------
func main() {
	logger.Init()          // inicializa zap
	log := logger.Logger() // obtiene logger estructurado
	defer log.Sync()       // flush buffers al salir

	ctx := context.Background()
	cfg := config.LoadConfig()

	// ---------------- DB ----------------
	db, err := sql.Open("sqlite", cfg.SQLitePath)
	if err != nil {
		log.Fatal("failed to open SQLite", zap.Error(err))
	}
	defer db.Close()

	if err := userRepo.InitSQLite(db); err != nil {
		log.Fatal("failed to initialize SQLite", zap.Error(err))
	}

	userRepoSQLite := userRepo.NewUserRepoSQLite(db)

	if err := db.PingContext(ctx); err != nil {
		log.Fatal("failed to ping SQLite", zap.Error(err))
	}

	taskRepoPostgres := taskRepo.NewTaskRepoPostgres(db)

	// ---------------- Cache ----------------
	var cacheInstance sharedCache.Cache
	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Warn("⚠️ Redis no disponible, cache en memoria:", zap.Error(err))
		cacheInstance = userCache.NewInMemoryCache(cfg.CacheTTL, 3*cfg.CacheTTL)
	} else {
		cacheInstance = userCache.NewRedisCache(rdb, cfg.CacheTTL)
		log.Info("✅ Redis conectado, cache habilitado")
	}

	// --------------- Servicio --------------
	userService := userApp.NewUserService(userRepoSQLite, cacheInstance, log)
	taskService := taskApp.NewTaskService(taskRepoPostgres, cacheInstance, log)

	// ---------------- Events ---------------
	var eventUserPublisher sharedBus.EventPublisher
	var eventTaskPublisher sharedBus.EventPublisher

	if cfg.UseKafka {
		log.Info("🚀 Usando Kafka como bus de eventos")

		// El writer ya no necesita un topic específico, es genérico.
		// Puede escribir en cualquier topic que se le pase en el mensaje.
		userWriter := kafka.NewWriter(kafka.WriterConfig{
			Brokers: cfg.KafkaBrokers,
			Topic:   userDomain.UserTopic, // Topic por defecto, pero puede ser cualquier otro.
		})

		taskWriter := kafka.NewWriter(kafka.WriterConfig{
			Brokers: cfg.KafkaBrokers,
			Topic:   taskDomain.TaskTopic, // Topic por defecto, pero puede ser cualquier otro.
		})

		defer userWriter.Close()
		defer taskWriter.Close()

		eventUserPublisher = infraEvents.NewKafkaPublisher(userWriter, log)
		eventTaskPublisher = infraEvents.NewKafkaPublisher(taskWriter, log)

		userConsumer := userEvents.NewUserConsumer(userService, log)
		taskConsumer := taskEvents.NewTaskConsumer(taskService, log)

		userKafkaReader := kafka.NewReader(kafka.ReaderConfig{
			Brokers:  cfg.KafkaBrokers,
			Topic:    userDomain.UserTopic,
			GroupID:  "hexagolab-user-service",
			MinBytes: 10e3, // 10KB
			MaxBytes: 10e6, // 10MB
		})
		defer userKafkaReader.Close()

		taskKafkaReader := kafka.NewReader(kafka.ReaderConfig{
			Brokers:  cfg.KafkaBrokers,
			Topic:    taskDomain.TaskTopic,
			GroupID:  "hexagolab-user-service",
			MinBytes: 10e3, // 10KB
			MaxBytes: 10e6, // 10MB
		})
		defer userKafkaReader.Close()

		userConsumerAdapter := infraEvents.NewConsumerAdapter(userKafkaReader, userConsumer, log)
		taskConsumerAdapter := infraEvents.NewConsumerAdapter(taskKafkaReader, taskConsumer, log)

		userConsumerAdapter.Start(ctx)
		taskConsumerAdapter.Start(ctx)

	} else {
		log.Info("⚡️Usando bus de eventos en memoria (canales de Go)")

		inMemoryUserBus := infraEvents.NewInMemoryEventBus(userDomain.UserTopic)
		inMemoryTaskBus := infraEvents.NewInMemoryEventBus(taskDomain.TaskTopic)

		eventUserPublisher = inMemoryUserBus
		eventTaskPublisher = infraEvents.NewInMemoryEventBus(taskDomain.TaskTopic)

		userConsumer := userEvents.NewUserConsumer(userService, log)
		taskConsumer := taskEvents.NewTaskConsumer(taskService, log)

		userEventsChannel := inMemoryUserBus.Subscribe(10)
		taskEventsChannel := inMemoryTaskBus.Subscribe(10)

		log.Info("🎧 Iniciando listener en memoria para eventos de usuario")
		userEvents.BackgroundConsumerChan(ctx, userEventsChannel, userConsumer)

		log.Info("🎧 Iniciando listener en memoria para eventos de tarea")
		taskEvents.BackgroundConsumerChan(ctx, taskEventsChannel, taskConsumer)

		// Simulamos la publicación de un evento de usuario
		userCreatedEvent := sharedEvents.UserCreated{
			ID:        uuid.New(),
			Email:     "simulated.user@example.com",
			Nombre:    "Usuario Simulado",
			BirthDate: time.Now().AddDate(-30, 0, 0),
		}

		payloadBytes, _ := json.Marshal(userCreatedEvent)
		simulatedEvent := sharedEvents.IntegrationEvent{
			Type: userDomain.UserCreated,
			Data: payloadBytes,
		}
		err := eventUserPublisher.Publish(context.Background(), simulatedEvent)
		if err != nil {
			log.Error("Fallo al publicar el evento simulado", zap.Error(err))
		} else {
			log.Info("✅ Evento 'UserCreated' simulado y publicado correctamente")
		}

	}

	// ------------ Outbox Worker ------------
	// Se podría ejecutar externamente
	eventRegistry := make(map[string]sharedEvents.EventMetadata)

	// Merge de los registros de cada dominio

	for k, v := range userDomain.NewEventRegistry() {
		eventRegistry[k] = v
	}
	for k, v := range taskDomain.NewEventRegistry() {
		eventRegistry[k] = v
	}

	if cfg.LocalDeployment {
		outboxRepoSQLite := sqlite.NewOutboxRepoSQLite(db)
		outboxUserWorker := infraRelayer.NewOutboxWorker(outboxRepoSQLite, eventUserPublisher, eventRegistry, cfg.OutboxPeriod, cfg.OutboxLimit, log)
		outboxUserWorker.Start(ctx)
		outboxTaskWorker := infraRelayer.NewOutboxWorker(outboxRepoSQLite, eventTaskPublisher, eventRegistry, cfg.OutboxPeriod, cfg.OutboxLimit, log)
		outboxTaskWorker.Start(ctx)
	} else {
		outboxRepoPostgres := postgres.NewOutboxRepoPostgres(db)
		outboxUserWorker := infraRelayer.NewOutboxWorker(outboxRepoPostgres, eventUserPublisher, eventRegistry, cfg.OutboxPeriod, cfg.OutboxLimit, log)
		outboxUserWorker.Start(ctx)
	}

	// ---------------- HTTP ----------------
	userHandler := userHttp.NewUserHandler(userService)
	taskHandler := taskHttp.NewTaskHandler(taskService)
	router := gin.Default()
	userHttp.RegisterUserRoutes(router, userHandler)
	taskHttp.RegisterTaskRoutes(router, taskHandler)

	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	log.Info("🚀 Server running",
		zap.String("url", "http://localhost:"+cfg.HTTPPort),
	)
	if err := router.Run(":" + cfg.HTTPPort); err != nil {
		log.Fatal("failed to start server: %v", zap.Error(err))
	}
}
