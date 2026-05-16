package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
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

var ErrQualityUnavailable = errors.New("quality unavailable")

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
			return "", ErrQualityUnavailable
		}
	case "medium":
		if qualityData.Medium == "" {
			return "", ErrQualityUnavailable
		}
	case "high":
		if qualityData.High == "" {
			return "", ErrQualityUnavailable
		}
	case "pro":
		if qualityData.Pro == "" {
			return "", ErrQualityUnavailable
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
		LEFT JOIN app_job j ON j.publicacion_id = l.publicacion_id AND j.calidad = l.calidad
		WHERE l.lru < NOW() - ($1 * INTERVAL '1 minute')
		  AND (j.id IS NULL OR j.estado NOT IN ('queued', 'processing'));
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

func (r *Repository) ExpireJobs(ctx context.Context, ea []domain.ExpiredAsset) error {
	if len(ea) == 0 {
		return nil
	}

	args := make([]any, 0, len(ea)*2)
	values := make([]string, 0, len(ea))

	for i, e := range ea {
		p1 := i*2 + 1
		p2 := i*2 + 2

		values = append(values, fmt.Sprintf("($%d::bigint, $%d::text)", p1, p2))
		args = append(args, e.PublicacionID, e.Calidad)
	}

	query := fmt.Sprintf(`
		UPDATE app_job j
		SET
			estado = 'expired',
			error = '',
			updated_at = NOW(),
			finished_at = NOW()
		FROM (
			VALUES %s
		) AS v(publicacion_id, calidad)
		WHERE j.publicacion_id = v.publicacion_id
		  AND j.calidad = v.calidad
		  AND j.estado = 'completed'
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

type jobScanner interface {
	Scan(dest ...any) error
}

func scanJob(scanner jobScanner) (domain.Job, error) {
	var job domain.Job
	var errorMsg sql.NullString
	var finishedAt sql.NullTime

	err := scanner.Scan(
		&job.ID,
		&job.PublicacionID,
		&job.Calidad,
		&job.Estado,
		&errorMsg,
		&job.RequesterToken,
		&job.CreatedAt,
		&job.UpdatedAt,
		&finishedAt,
	)
	if err != nil {
		return domain.Job{}, err
	}

	if errorMsg.Valid {
		job.Error = errorMsg.String
	}
	if finishedAt.Valid {
		finished := finishedAt.Time
		job.FinishedAt = &finished
	}

	return job, nil
}

func jobLockKey(publicacionID int64, calidad string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(fmt.Sprintf("%d:%s", publicacionID, calidad)))
	return int64(h.Sum64())
}

func (r *Repository) GetJobByID(ctx context.Context, jobID int64) (domain.Job, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT
			id,
			publicacion_id,
			calidad,
			estado,
			COALESCE(error, ''),
			requester_token,
			created_at,
			updated_at,
			finished_at
		FROM app_job
		WHERE id = $1
	`, jobID)

	return scanJob(row)
}

func (r *Repository) EnsureJob(ctx context.Context, publicacionID int64, calidad, requesterToken string) (domain.Job, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Job{}, false, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, jobLockKey(publicacionID, calidad)); err != nil {
		return domain.Job{}, false, err
	}

	selectRow := tx.QueryRowContext(ctx, `
		SELECT
			id,
			publicacion_id,
			calidad,
			estado,
			COALESCE(error, ''),
			requester_token,
			created_at,
			updated_at,
			finished_at
		FROM app_job
		WHERE publicacion_id = $1 AND calidad = $2
	`, publicacionID, calidad)

	job, err := scanJob(selectRow)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return domain.Job{}, false, err
	}

	if errors.Is(err, sql.ErrNoRows) {
		insertRow := tx.QueryRowContext(ctx, `
			INSERT INTO app_job (
				publicacion_id,
				calidad,
				estado,
				error,
				requester_token
			)
			VALUES ($1, $2, $3, '', $4)
			RETURNING
				id,
				publicacion_id,
				calidad,
				estado,
				COALESCE(error, ''),
				requester_token,
				created_at,
				updated_at,
				finished_at
		`, publicacionID, calidad, domain.JobQueued, requesterToken)

		job, err = scanJob(insertRow)
		if err != nil {
			return domain.Job{}, false, err
		}

		if err := tx.Commit(); err != nil {
			return domain.Job{}, false, err
		}
		return job, true, nil
	}

	if job.Estado.IsActive() {
		if err := tx.Commit(); err != nil {
			return domain.Job{}, false, err
		}
		return job, false, nil
	}

	updateRow := tx.QueryRowContext(ctx, `
		UPDATE app_job
		SET
			estado = $1,
			error = '',
			requester_token = $2,
			updated_at = NOW(),
			finished_at = NULL
		WHERE id = $3
		RETURNING
			id,
			publicacion_id,
			calidad,
			estado,
			COALESCE(error, ''),
			requester_token,
			created_at,
			updated_at,
			finished_at
	`, domain.JobQueued, requesterToken, job.ID)

	job, err = scanJob(updateRow)
	if err != nil {
		return domain.Job{}, false, err
	}

	if err := tx.Commit(); err != nil {
		return domain.Job{}, false, err
	}

	return job, true, nil
}

func (r *Repository) UpdateJobStatus(ctx context.Context, jobID int64, status domain.JobStatus, errorMessage string) error {
	finishedAt := "NULL"
	if status.IsTerminal() {
		finishedAt = "NOW()"
	}

	query := fmt.Sprintf(`
		UPDATE app_job
		SET
			estado = $1,
			error = $2,
			updated_at = NOW(),
			finished_at = %s
		WHERE id = $3
	`, finishedAt)

	_, err := r.db.ExecContext(ctx, query, status, errorMessage, jobID)
	return err
}
