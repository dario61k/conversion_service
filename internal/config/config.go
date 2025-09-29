package config

import (
	"github.com/joho/godotenv"
	"log"
	"os"
)

type Config struct {
	HTTPPort        string
	PGDSN           string
	MinioEndpoint   string
	MinioAccessKey  string
	MinioSecretKey  string
	MinioUseSSL     bool
	DownloadsBucket string
	VideosBucket    string
	SubsBucket      string
	FFmpegPath      string
}

func Load() Config {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file")
	}
	return Config{
		HTTPPort:        getenv("HTTP_PORT", "8080"),
		PGDSN:           mustenv("PG_DSN"),
		MinioEndpoint:   mustenv("MINIO_ENDPOINT"),
		MinioAccessKey:  mustenv("MINIO_ACCESS_KEY"),
		MinioSecretKey:  mustenv("MINIO_SECRET_KEY"),
		MinioUseSSL:     mustenv("MINIO_USE_SSL") == "true",
		DownloadsBucket: mustenv("DOWNLOADS_BUCKET"),
		VideosBucket:    mustenv("VIDEOS_BUCKET"),
		SubsBucket:      getenv("SUBS_BUCKET", "subtitulos"),
		FFmpegPath:      mustenv("FFMPEG_PATH"),
	}
}

func getenv(k, def string) string {
	if v, ok := os.LookupEnv(k); ok {
		return v
	}
	return def
}

func mustenv(k string) string {
	v, ok := os.LookupEnv(k)
	if !ok {
		log.Fatalf("variable de entorno faltante: %s", k)
	}
	return v
}
