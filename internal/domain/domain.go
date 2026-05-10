package domain

import (
	"github.com/dario61k/conversion-service/internal/config"
	"github.com/dario61k/conversion-service/internal/db"
	"github.com/dario61k/conversion-service/internal/storage"
)

type CronParams struct {
	Repo  *db.Repository
	Store *storage.S3
	Cfg *config.Config
}
