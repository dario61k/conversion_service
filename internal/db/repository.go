package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

type Repository struct {
	db *sql.DB
}

func New(db *sql.DB) *Repository { return &Repository{db: db} }

type pubQuality struct {
	Low    string `json:"low"`
	Medium string `json:"medium"`
	High   string `json:"high"`
	Pro    string `json:"pro"`
}

func (r *Repository) PublicationManifestAndQuality(ctx context.Context, id, quality string) (manifest string, err error) {
	var qualityDataDb string

	err = r.db.QueryRowContext(ctx,
		`SELECT url_manifiesto, descarga
         FROM app_publicacion
         WHERE contenido_id = $1`,
		id).Scan(&manifest, &qualityDataDb)
	if err != nil {
		return "", err
	}

	qualityData := pubQuality{}
	err = json.Unmarshal([]byte(qualityDataDb), &qualityData)
	if err != nil {
		return "", err
	}

	switch quality {
	case "low":
		if qualityData.Low == "" {
			return "", errors.New("no low quality")
		}
	case "medium":
		if qualityData.Medium == "" {
			return "", errors.New("no medium quality")
		}
	case "high":
		if qualityData.High == "" {
			return "", errors.New("no high quality")
		}
	case "pro":
		if qualityData.Pro == "" {
			return "", errors.New("no pro quality")
		}
	}

	return
}

func (r *Repository) PublicationManifest(ctx context.Context, id string) (manifest string, err error) {
	err = r.db.QueryRowContext(ctx, `SELECT url_manifiesto FROM app_publicacion WHERE id=$1`, id).Scan(&manifest)
	return
}
