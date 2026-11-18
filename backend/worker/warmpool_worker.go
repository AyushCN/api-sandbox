package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/provider"
)

// MaintainWarmPool runs continuously in a goroutine and tops up the warm pool
func MaintainWarmPool() {
	runtimes := []string{"node", "python", "go"}
	targetSize := 2
	poolReady := false

	for {
		if !poolReady {
			if err := provider.ResetWarmPool(context.Background()); err != nil {
				slog.Error("Failed to migrate legacy warm pool", "error", err)
				time.Sleep(10 * time.Second)
				continue
			}
			poolReady = true
		}
		for _, rt := range runtimes {
			count, err := db.RedisClient.LLen(context.Background(), "warm-pool:"+rt).Result()
			if err == nil && count < int64(targetSize) {
				slog.Info("Pre-pulling runtime image", "runtime", rt)
				imageRef, err := provider.CreateWarmContainer(context.Background(), rt)
				if err == nil {
					db.RedisClient.RPush(context.Background(), "warm-pool:"+rt, imageRef)
				} else {
					slog.Error("Failed to create warm container", "error", err)
				}
			}
		}
		time.Sleep(10 * time.Second)
	}
}
