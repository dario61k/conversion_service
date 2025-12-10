package crons

import (
	"context"
	"log"

	"github.com/dario61k/conversion-service/internal/db"
	"github.com/dario61k/conversion-service/internal/storage"
)

type Cron struct {
	repo  *db.Repository
	store *storage.S3
}

func New(repo *db.Repository, store *storage.S3) *Cron {
	return &Cron{
		repo:  repo,
		store: store,
	}
}


func (c *Cron) CleanUp() {
	ctx := context.Background()

	// 72 horas = 4320 min
	expired, err := c.repo.GetExpiredAssets(ctx, 1)
	if err != nil {
		log.Printf("[CRON] Error obteniendo expirados: %v", err)
		return
	}



	for _, e := range expired {
		object := e.Manifiesto + "/" + e.Calidad + ".mp4"

		err := c.store.Remove(ctx, "descargas", object)
		if err != nil {
			log.Printf("[CRON] Error borrando %s: %v", object, err)
		} else {
			log.Printf("[CRON] Borrado: %s", object)
		}
	}
}
