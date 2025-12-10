package cleanup

import (
	"time"

	"github.com/dario61k/conversion-service/internal/services/crons"
	"github.com/robfig/cron/v3"
)

func Start(cr *crons.Cron) *cron.Cron {
	c := cron.New(cron.WithLocation(time.UTC))

	// 0 3 * * *
	// * * * * *
	_, err := c.AddFunc("* * * * *", func() {
		cr.CleanUp()
	})

	if err != nil {
		return nil
	}

	c.Start()
	return c
}
