package cleanup

import (
	"context"
	"log"
	"time"

	"github.com/dario61k/conversion-service/internal/config"
	"github.com/dario61k/conversion-service/internal/storage"
	"github.com/robfig/cron/v3"
)

func Start(cfg config.Config, store *storage.S3) *cron.Cron {
	c := cron.New(cron.WithLocation(time.UTC))
	// 03:00 UTC todos los días
	_, err := c.AddFunc("0 3 * * *", func() {
		ctx := context.Background()
		cutoff := time.Now().Add(-30 * 24 * time.Hour)
		for obj := range store.List(ctx, cfg.DownloadsBucket, "") {
			if obj.Err != nil {
				log.Printf("cleanup: %v", obj.Err)
				continue
			}
			if obj.LastModified.Before(cutoff) {
				if err := store.Remove(ctx, cfg.DownloadsBucket, obj.Key); err != nil {
					log.Printf("cleanup remove: %v", err)
				}
			}
		}
	})
	if err != nil {
		return nil
	}
	c.Start()
	return c
}
