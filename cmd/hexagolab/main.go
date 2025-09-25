package main

import (
	"context"
	"database/sql"
	"log"

	config "github.com/davicafu/hexagolab/internal/config"
	"github.com/davicafu/hexagolab/internal/user/application"
	"github.com/davicafu/hexagolab/internal/user/domain"
	userEventsIn "github.com/davicafu/hexagolab/internal/user/infra/inbound/events"
	userHttp "github.com/davicafu/hexagolab/internal/user/infra/inbound/http"
	userCache "github.com/davicafu/hexagolab/internal/user/infra/outbound/cache"
	userRepo "github.com/davicafu/hexagolab/internal/user/infra/outbound/db/sqlite"
	userEventsOut "github.com/davicafu/hexagolab/internal/user/infra/outbound/events"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/segmentio/kafka-go"

	// _ "github.com/mattn/go-sqlite3" // requires gcc
	_ "modernc.org/sqlite"
)

// ---------------- Main ----------------
func main() {
	ctx := context.Background()
	cfg := config.LoadConfig()

	// ---------------- SQLite ----------------
	db, err := sql.Open("sqlite", cfg.SQLitePath)
	if err != nil {
		log.Fatalf("failed to open SQLite: %v", err)
	}
	defer db.Close()

	if err := userRepo.InitSQLite(db); err != nil {
		log.Fatalf("failed to initialize SQLite: %v", err)
	}

	userRepoSQLite := userRepo.NewUserRepoSQLite(db)

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("failed to ping SQLite: %v", err)
	}

	// ---------------- Redis ----------------
	var userCacheInstance domain.UserCache
	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Println("⚠️ Redis no disponible, cache deshabilitado:", err)
		userCacheInstance = nil
	} else {
		userCacheInstance = userCache.NewRedisUserCache(rdb, cfg.CacheTTL)
		log.Println("✅ Redis conectado, cache habilitado")
	}

	// ---------------- Kafka ---------------
	userWriter := kafka.NewWriter(kafka.WriterConfig{
		Brokers: cfg.KafkaBrokers,
		Topic:   cfg.KafkaTopicUser,
	})
	userKafka := userEventsOut.NewKafkaUserPublisher(userWriter)

	// --------------- Servicio --------------
	userService := application.NewUserService(userRepoSQLite, userCacheInstance, userKafka)

	// ------------ Outbox Worker ------------
	outboxWorker := application.NewOutboxWorker(userRepoSQLite, userKafka, cfg.OutboxPeriod, cfg.OutboxLimit)
	outboxWorker.Start(ctx)

	// ---------------- Events ---------------
	userConsumer := userEventsIn.NewUserConsumer(userService, 10)
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

	log.Printf("🚀 Server running at http://localhost:%s", cfg.HTTPPort)
	if err := router.Run(":" + cfg.HTTPPort); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
