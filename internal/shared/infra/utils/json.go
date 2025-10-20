package utils

import (
	"encoding/json"

	"go.uber.org/zap"
)

func UnmarshalAndHandle[T any](log *zap.Logger, data json.RawMessage, handler func(T)) {
	var evt T
	if err := json.Unmarshal(data, &evt); err != nil {
		log.Warn("Failed to unmarshal event data", zap.Error(err))
		return
	}
	handler(evt)
}
