package jobs

import (
	"context"
	"log"

	"github.com/dario61k/conversion-service/internal/config"
	"github.com/dario61k/conversion-service/internal/db"
	"github.com/dario61k/conversion-service/internal/domain"
	"github.com/dario61k/conversion-service/internal/storage"
)

type CronParams struct {
	Repo  *db.Repository
	Store *storage.S3
	Cfg   *config.Config
}

func CleanUp(c *CronParams) {
	ctx := context.Background()

	log.Println("Init Cleanup Cron....")

	expired, err := c.Repo.GetExpiredAssets(ctx, c.Cfg.TTL)
	if err != nil {
		log.Printf("[CRON] Error obteniendo expirados: %v", err)
		return
	}

	if len(expired) == 0 {
		log.Println("Finish Cleanup Cron...no expired assets")
		return
	}

	ea := make([]domain.ExpiredAsset, 0, len(expired))

	for _, e := range expired {
		object := e.Manifiesto + "/" + e.Calidad + ".mp4"

		err := c.Store.Remove(ctx, c.Cfg.DownloadsBucket, object)
		if err != nil {
			log.Printf("[CRON] Error borrando %s: %v", object, err)
		} else {
			log.Printf("[CRON] Borrado: %s", object)
			ea = append(ea, e)
		}
	}

	err = c.Repo.DeleteLRUs(ctx, ea)
	if err != nil {
		log.Printf("[CRON] Error borrando lru de base de datos: %v", err)
	}

	if err := c.Repo.ExpireJobs(ctx, ea); err != nil {
		log.Printf("[CRON] Error expirando jobs: %v", err)
	}

}
