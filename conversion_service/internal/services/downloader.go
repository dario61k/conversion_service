package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dario61k/conversion-service/internal/config"
	"github.com/dario61k/conversion-service/internal/db"
	"github.com/dario61k/conversion-service/internal/domain"
	"github.com/dario61k/conversion-service/internal/queue"
	"github.com/dario61k/conversion-service/internal/storage"
)

type Downloader struct {
	cfg     config.Config
	repo    *db.Repository
	storage *storage.S3
	queue   *queue.Client
}

type AccessResult struct {
	Status      string `json:"status"`
	DownloadURL string `json:"download_url,omitempty"`
	JobID       int64  `json:"job_id,omitempty"`
	Error       string `json:"error,omitempty"`
}

func NewDowloaderService(cfg config.Config, repo *db.Repository, s3 *storage.S3, q *queue.Client) *Downloader {
	return &Downloader{cfg: cfg, repo: repo, storage: s3, queue: q}
}

// RequestVideo resuelve el acceso al recurso: devuelve URL inmediata si ya existe o un job si debe generarse.
func (d *Downloader) RequestVideo(ctx context.Context, requesterToken, id, quality string) (AccessResult, error) {
	manifest, err := d.repo.PublicationManifestAndQuality(ctx, id, quality)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, db.ErrQualityUnavailable) {
			return AccessResult{}, ErrNotFound
		}
		return AccessResult{}, err
	}

	objectName := fmt.Sprintf("%s/%s.mp4", manifest, quality)

	if d.storage.Exists(ctx, d.cfg.DownloadsBucket, objectName) {
		if err := d.repo.UpdateLRU(ctx, id, quality); err != nil {
			return AccessResult{}, err
		}
		url, err := d.storage.Presign(ctx, d.cfg.DownloadsBucket, objectName, 48*time.Hour)
		if err != nil {
			return AccessResult{}, err
		}
		return AccessResult{Status: string(domain.JobCompleted), DownloadURL: url}, nil
	}

	publicationID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return AccessResult{}, ErrInvalidRequest
	}

	job, shouldPublish, err := d.repo.EnsureJob(ctx, publicationID, quality, requesterToken)
	if err != nil {
		return AccessResult{}, err
	}

	if shouldPublish {
		msg := queue.VideoBuildRequested{
			JobID:          job.ID,
			PublicationID:  publicationID,
			Quality:        quality,
			Manifest:       manifest,
			ObjectKey:      objectName,
			RequestedAt:    time.Now().UTC(),
			Attempt:        1,
			IdempotencyKey: fmt.Sprintf("%d:%s", publicationID, quality),
			RequesterToken: requesterToken,
		}
		if err := d.queue.PublishBuildRequest(ctx, msg); err != nil {
			return AccessResult{}, err
		}
	}

	return AccessResult{Status: string(job.Estado), JobID: job.ID}, nil
}

// VideoURL devuelve la URL de descarga (48 h) de un vídeo en la calidad solicitada.
func (d *Downloader) VideoURL(ctx context.Context, id, quality string) (string, error) {
	manifest, err := d.repo.PublicationManifestAndQuality(ctx, id, quality)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", ErrNotFound
		}
		return "", err
	}

	objectName := fmt.Sprintf("%s/%s.mp4", manifest, quality)

	// 1) Ya existe ensamblado → presign
	if d.storage.Exists(ctx, d.cfg.DownloadsBucket, objectName) {
		if err := d.repo.UpdateLRU(ctx, id, quality); err != nil {
			return "", err
		}
		return d.storage.Presign(ctx, d.cfg.DownloadsBucket, objectName, 48*time.Hour)
	}

	// 2) Construir
	if err := d.build(ctx, manifest, quality, objectName); err != nil {
		return "", err
	}

	if err := d.repo.UpdateLRU(ctx, id, quality); err != nil {
		return "", err
	}

	return d.storage.Presign(ctx, d.cfg.DownloadsBucket, objectName, 48*time.Hour)

}

// SubtitleURL devuelve la URL de descarga (48 h) de los subtítulos.
func (d *Downloader) SubtitleURL(ctx context.Context, id, lang string) (string, error) {
	manifest, err := d.repo.PublicationManifest(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", ErrNotFound
		}
		return "", err
	}

	objectName := fmt.Sprintf("%s/%s.vtt", manifest, lang)

	if !d.storage.Exists(ctx, d.cfg.SubsBucket, objectName) {
		return "", ErrNotFound
	}
	return d.storage.Presign(ctx, d.cfg.SubsBucket, objectName, 48*time.Hour)
}

// BuildVideo expone el proceso de muxing para el worker.
func (d *Downloader) BuildVideo(ctx context.Context, manifest, quality, target string) error {
	return d.build(ctx, manifest, quality, target)
}

// JobStatus devuelve el estado de un job y, si ya terminó, la URL temporal del archivo generado.
func (d *Downloader) JobStatus(ctx context.Context, requesterToken string, jobID int64) (AccessResult, error) {
	job, err := d.repo.GetJobByID(ctx, jobID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AccessResult{}, ErrNotFound
		}
		return AccessResult{}, err
	}

	result := AccessResult{Status: string(job.Estado), JobID: job.ID}

	switch job.Estado {
	case domain.JobCompleted:
		manifest, err := d.repo.PublicationManifest(ctx, fmt.Sprint(job.PublicacionID))
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return AccessResult{}, ErrNotFound
			}
			return AccessResult{}, err
		}

		objectName := fmt.Sprintf("%s/%s.mp4", manifest, job.Calidad)
		if !d.storage.Exists(ctx, d.cfg.DownloadsBucket, objectName) {
			return AccessResult{}, ErrNotFound
		}

		if err := d.repo.UpdateLRU(ctx, fmt.Sprint(job.PublicacionID), job.Calidad); err != nil {
			return AccessResult{}, err
		}

		url, err := d.storage.Presign(ctx, d.cfg.DownloadsBucket, objectName, 48*time.Hour)
		if err != nil {
			return AccessResult{}, err
		}
		result.DownloadURL = url
	case domain.JobFailed:
		result.Error = job.Error
	case domain.JobExpired:
		result.Error = "job expirado"
	}

	return result, nil
}

// build crea el MP4 juntando audio & vídeo sin transcodificar.
func (d *Downloader) build(ctx context.Context, manifest, quality, target string) error {
    prefix := fmt.Sprintf("%s/%s/", manifest, quality)
    var videoKey, audioKey string
    for obj := range d.storage.List(ctx, d.cfg.VideosBucket, prefix) {
        if obj.Err != nil {
            return obj.Err
        }
        switch {
        case strings.HasSuffix(obj.Key, ".m4a") || strings.HasSuffix(obj.Key, ".aac"):
            audioKey = obj.Key
        case strings.HasSuffix(obj.Key, ".mp4") || strings.HasSuffix(obj.Key, ".mkv"):
            videoKey = obj.Key
        }
    }
    if videoKey == "" || audioKey == "" {
        return ErrNotFound
    }

    tmpDir, err := os.MkdirTemp("", "conv-"+manifest+"-"+quality+"-")
    if err != nil {
        return err
    }
    defer os.RemoveAll(tmpDir)

    vidExt := filepath.Ext(videoKey)
    audExt := filepath.Ext(audioKey)
    vidFile := filepath.Join(tmpDir, "video"+vidExt)
    audFile := filepath.Join(tmpDir, "audio"+audExt)
    outFile := filepath.Join(tmpDir, "out.mp4")

    if err := d.storage.Get(ctx, d.cfg.VideosBucket, videoKey, vidFile); err != nil {
        return err
    }
    if err := d.storage.Get(ctx, d.cfg.VideosBucket, audioKey, audFile); err != nil {
        return err
    }

    cmd := exec.CommandContext(ctx, d.cfg.FFmpegPath, "-y", "-i", vidFile, "-i", audFile, "-c", "copy", outFile)
    if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Sprintf("ffmpg failed: %w: %s", err, out)
        return err
    }

    if info, err := os.Stat(outFile); err != nil {
        return err
    } else if info.Size() == 0 {
        return fmt.Errorf("ffmpeg produced empty output: %s", outFile)
    }

    return d.storage.Put(ctx, d.cfg.DownloadsBucket, target, outFile, "video/mp4")
}

func (d *Downloader) ListBuckets(ctx context.Context) ([]storage.BucketsInfo, error) {
	buckets, err := d.storage.ListBuckets(ctx)
	if err != nil {
		return nil, err
	}
	return buckets, nil
}
