package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/dario61k/conversion-service/internal/domain"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

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
         WHERE id = $1`,
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

func (r *Repository) GetExpiredAssets(ctx context.Context, minutes int) ([]domain.ExpiredAsset, error) {
	query := `
		SELECT
			p.id,
			l.calidad,
			p.url_manifiesto
		FROM app_publicacion_lru l
		JOIN app_publicacion p ON p.id = l.publicacion_id
		WHERE l.lru < NOW() - ($1 * INTERVAL '1 minute');
	`

	rows, err := r.db.QueryContext(ctx, query, minutes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.ExpiredAsset
	for rows.Next() {
		var e domain.ExpiredAsset
		if err := rows.Scan(&e.PublicacionID, &e.Calidad, &e.Manifiesto); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *Repository) DeleteLRUs(ctx context.Context, ea []domain.ExpiredAsset) error {

	if len(ea) == 0 {
		return nil
	}

	args := make([]any, 0, len(ea)*2)
	values := make([]string, 0, len(ea))

	for i, e := range ea {
		p1 := i*2 + 1
		p2 := i*2 + 2

		values = append(values,
			fmt.Sprintf("($%d::bigint, $%d::text)", p1, p2),
		)

		args = append(args,
			e.PublicacionID,
			e.Calidad,
		)
	}

	query := fmt.Sprintf(`
		DELETE FROM app_publicacion_lru l
		USING (
			VALUES %s
		) AS v(publicacion_id, calidad)
		WHERE
			l.publicacion_id = v.publicacion_id
			AND l.calidad = v.calidad
	`, strings.Join(values, ","))

	_, err := r.db.ExecContext(ctx, query, args...)
	return err
}

func (r *Repository) UpdateLRU(ctx context.Context, id string, quality string) error {
	query := `
		INSERT INTO app_publicacion_lru (publicacion_id, calidad, lru)
		VALUES ($1, $2, NOW())
		ON CONFLICT (publicacion_id, calidad)
		DO UPDATE SET lru = EXCLUDED.lru;
	`

	_, err := r.db.ExecContext(ctx, query, id, quality)
	return err
}
