package main

import (
	"context"
	"database/sql"

	config "github.com/davicafu/hexagolab/internal/config"
	userApp "github.com/davicafu/hexagolab/internal/user/application"
	userDomain "github.com/davicafu/hexagolab/internal/user/domain"
	userEventsIn "github.com/davicafu/hexagolab/internal/user/infra/inbound/events"
	userHttp "github.com/davicafu/hexagolab/internal/user/infra/inbound/http"
	userCache "github.com/davicafu/hexagolab/internal/user/infra/outbound/cache"
	userRepo "github.com/davicafu/hexagolab/internal/user/infra/outbound/db/sqlite"
	userEventsOut "github.com/davicafu/hexagolab/internal/user/infra/outbound/events"
	"github.com/davicafu/hexagolab/pkg/logger"

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

	// ---------------- SQLite ----------------
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

	// ---------------- Redis ----------------
	var userCacheInstance userDomain.UserCache
	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Warn("⚠️ Redis no disponible, cache deshabilitado:", zap.Error(err))
		userCacheInstance = nil
	} else {
		userCacheInstance = userCache.NewRedisUserCache(rdb, cfg.CacheTTL)
		log.Info("✅ Redis conectado, cache habilitado")
	}

	// ---------------- Kafka ---------------
	userWriter := kafka.NewWriter(kafka.WriterConfig{
		Brokers: cfg.KafkaBrokers,
		Topic:   cfg.KafkaTopicUser,
	})
	userKafka := userEventsOut.NewKafkaUserPublisher(userWriter)

	// --------------- Servicio --------------
	userService := userApp.NewUserService(userRepoSQLite, userCacheInstance, userKafka, log)

	// ------------ Outbox Worker ------------
	// Se podría ejecutar externamente
	outboxWorker := userApp.NewOutboxWorker(userRepoSQLite, userKafka, cfg.OutboxPeriod, cfg.OutboxLimit, log)
	outboxWorker.Start(ctx)

	// ---------------- Events ---------------
	userConsumer := userEventsIn.NewUserConsumer(userService, 10, log)
	userEvents := make(chan userEventsIn.UserEvent, 100)
	userEventsIn.BackgroundConsumerChan(ctx, userEvents, userConsumer)

	userPayload := `[{"ID":"11111111-1111-1111-1111-111111111111","Email":"test@example.com","Nombre":"Test User","BirthDate":"2000-01-01T00:00:00Z"}]`
	userEvents <- userEventsIn.UserEvent{
		Key:     "user.created",
		Payload: []byte(userPayload),
	}

	// ---------------- HTTP ----------------
	userHandler := userHttp.NewUserHandler(userService)
	router := gin.Default()
	userHttp.RegisterUserRoutes(router, userHandler)

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
