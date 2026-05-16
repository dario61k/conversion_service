package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPPort         string
	PGDSN            string
	MinioEndpoint    string
	MinioAccessKey   string
	MinioSecretKey   string
	MinioUseSSL      bool
	DownloadsBucket  string
	VideosBucket     string
	SubsBucket       string
	FFmpegPath       string
	Debug            bool
	TTL              int
	AuthEndpoint     string
	AMQPURL          string
	AMQPExchange     string
	AMQPQueueBuild   string
	AMQPQueueRetry   string
	AMQPQueueDLQ     string
	AMQPPrefetch     int
	AMQPRetryTTLMS   int
	AMQPMaxAttempts  int
	LongPollTimeout  int
	LongPollInterval int
	WorkerCount      int
}

func Load() Config {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	ttl := getenv("TTL", "4320")
	ttlInt, err := strconv.Atoi(ttl)
	if err != nil {
		log.Fatal("Invalid TTL value")
	}

	return Config{
		HTTPPort:         getenv("HTTP_PORT", "8080"),
		PGDSN:            mustenv("PG_DSN"),
		MinioEndpoint:    mustenv("MINIO_ENDPOINT"),
		MinioAccessKey:   mustenv("MINIO_ACCESS_KEY"),
		MinioSecretKey:   mustenv("MINIO_SECRET_KEY"),
		MinioUseSSL:      mustenv("MINIO_USE_SSL") == "true",
		DownloadsBucket:  mustenv("DOWNLOADS_BUCKET"),
		VideosBucket:     mustenv("VIDEOS_BUCKET"),
		SubsBucket:       getenv("SUBS_BUCKET", "subtitulos"),
		FFmpegPath:       mustenv("FFMPEG_PATH"),
		Debug:            mustenv("DEBUG") == "true",
		TTL:              ttlInt,
		AuthEndpoint:     mustenv("AUTH_ENDPOINT"),
		AMQPURL:          mustenv("AMQP_URL"),
		AMQPExchange:     getenv("AMQP_EXCHANGE", "conversion.direct"),
		AMQPQueueBuild:   getenv("AMQP_QUEUE_BUILD", "video.build.request"),
		AMQPQueueRetry:   getenv("AMQP_QUEUE_RETRY", "video.build.retry"),
		AMQPQueueDLQ:     getenv("AMQP_QUEUE_DLQ", "video.build.dlq"),
		AMQPPrefetch:     mustInt(getenv("AMQP_PREFETCH", "2"), "AMQP_PREFETCH"),
		AMQPRetryTTLMS:   mustInt(getenv("AMQP_RETRY_TTL_MS", "30000"), "AMQP_RETRY_TTL_MS"),
		AMQPMaxAttempts:  mustInt(getenv("AMQP_MAX_ATTEMPTS", "5"), "AMQP_MAX_ATTEMPTS"),
		LongPollTimeout:  mustInt(getenv("LONG_POLL_TIMEOUT_SEC", "25"), "LONG_POLL_TIMEOUT_SEC"),
		LongPollInterval: mustInt(getenv("LONG_POLL_INTERVAL_MS", "1000"), "LONG_POLL_INTERVAL_MS"),
		WorkerCount:      mustInt(getenv("WORKER_COUNT", "2"), "WORKER_COUNT"),
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

func mustInt(v string, key string) int {
	n, err := strconv.Atoi(v)
	if err != nil {
		log.Fatalf("Invalid %s value", key)
	}
	return n
}
