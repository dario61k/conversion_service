package downloader

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/dario61k/conversion-service/internal/config"
	"github.com/dario61k/conversion-service/internal/db"
	"github.com/dario61k/conversion-service/internal/storage"
)

type Downloader struct {
	cfg     config.Config
	repo    *db.Repository
	storage *storage.S3
}

func New(cfg config.Config, repo *db.Repository, s3 *storage.S3) *Downloader {
	return &Downloader{cfg: cfg, repo: repo, storage: s3}
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
		return d.storage.Presign(ctx, d.cfg.DownloadsBucket, objectName, 48*time.Hour)
	}

	// 2) Construir
	if err := d.build(ctx, manifest, quality, objectName); err != nil {
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

	tmpDir := os.TempDir()
	vidFile := filepath.Join(tmpDir, filepath.Base(videoKey))
	audFile := filepath.Join(tmpDir, filepath.Base(audioKey))
	outFile := filepath.Join(tmpDir, filepath.Base(target))

	if err := d.storage.Get(ctx, d.cfg.VideosBucket, videoKey, vidFile); err != nil {
		return err
	}
	if err := d.storage.Get(ctx, d.cfg.VideosBucket, audioKey, audFile); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, d.cfg.FFmpegPath, "-y", "-i", vidFile, "-i", audFile, "-c", "copy", outFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("ffmpeg output: %s\n", string(out))
		return err
	} else if len(out) > 0 {
		fmt.Printf("ffmpeg: %s\n", strings.TrimSpace(string(out)))
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
