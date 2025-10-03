package logger

import (
	"go.uber.org/zap"
)

var log *zap.Logger

// Init inicializa el logger global
func Init() {
	var err error
	cfg := zap.NewProductionConfig()
	cfg.Encoding = "json"            // Logs estructurados en JSON
	cfg.EncoderConfig.TimeKey = "ts" // timestamp
	cfg.EncoderConfig.MessageKey = "msg"
	cfg.EncoderConfig.LevelKey = "level"
	cfg.EncoderConfig.CallerKey = "caller"

	log, err = cfg.Build()
	if err != nil {
		panic(err)
	}
}

// Sugar retorna un logger más “friendly” para usar con printf-like
func Sugar() *zap.SugaredLogger {
	return log.Sugar()
}

// Logger retorna el logger estructurado
func Logger() *zap.Logger {
	return log
}
