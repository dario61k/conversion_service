package cron

import (
	"log"
	"time"

	"github.com/dario61k/conversion-service/internal/cron/jobs"
	"github.com/robfig/cron/v3"
)

func Start(cp *jobs.CronParams) *cron.Cron {
	c := cron.New(cron.WithLocation(time.UTC))

	// 0 3 * * *
	// * * * * *
	_, err := c.AddFunc("* * * * *", func() {
		jobs.CleanUp(cp)
		
	})

	if err != nil {
		log.Fatal("Error setting ClenUp cron")
	}

	c.Start()
	return c
}
