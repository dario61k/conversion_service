package jobs

import (
	"context"
	"log"
	"github.com/dario61k/conversion-service/internal/domain"
)


func CleanUp(c *domain.CronParams) {
	ctx := context.Background()
	
	// 72 horas = 4320 min
	expired, err := c.Repo.GetExpiredAssets(ctx, c.Cfg.TTL)
	if err != nil {
		log.Printf("[CRON] Error obteniendo expirados: %v", err)
		return
	}

	for _, e := range expired {
		object := e.Manifiesto + "/" + e.Calidad + ".mp4"

		err := c.Store.Remove(ctx, "descargas", object)
		if err != nil {
			log.Printf("[CRON] Error borrando %s: %v", object, err)
		} else {
			log.Printf("[CRON] Borrado: %s", object)
		}
	}
}
