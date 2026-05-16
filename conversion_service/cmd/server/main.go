package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dario61k/conversion-service/cmd/helpers"

	"github.com/gin-gonic/gin"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/dario61k/conversion-service/internal/config"
	"github.com/dario61k/conversion-service/internal/cron"
	"github.com/dario61k/conversion-service/internal/cron/jobs"
	"github.com/dario61k/conversion-service/internal/db"
	"github.com/dario61k/conversion-service/internal/handlers"
	"github.com/dario61k/conversion-service/internal/middlewares"
	"github.com/dario61k/conversion-service/internal/queue"
	"github.com/dario61k/conversion-service/internal/services"
	"github.com/dario61k/conversion-service/internal/storage"
	"github.com/dario61k/conversion-service/internal/worker"
)

func main() {
	cfg := config.Load()

	// Almacenamiento S3 compatible
	store, err := storage.New(cfg.MinioEndpoint, cfg.MinioAccessKey, cfg.MinioSecretKey, cfg.MinioUseSSL)
	if err != nil {
		log.Fatalf("minio: %v", err)
	}

	// Dependencias de dominio
	dbPool := db.NewDBPool(cfg.PGDSN, 5, 10)
	repository := db.NewRepository(dbPool)

	queueClient, err := queue.NewClient(cfg)
	if err != nil {
		log.Fatalf("rabbitmq: %v", err)
	}
	defer queueClient.Close()

	downloaderService := services.NewDowloaderService(cfg, repository, store, queueClient)
	handler := handlers.NewHandler(downloaderService, cfg)

	cp := jobs.CronParams{
		Repo:  repository,
		Store: store,
		Cfg:   &cfg,
	}

	cron := cron.Start(&cp)
	defer cron.Stop()

	workerCtx, workerCancel := context.WithCancel(context.Background())
	workerRunner := worker.Start(workerCtx, cfg, repository, store, downloaderService, queueClient)
	go func() {
		for err := range workerRunner.Errors() {
			log.Printf("worker error: %v", err)
		}
	}()

	// Router HTTP
	gin.SetMode(gin.ReleaseMode) // Prod Mode
	if cfg.Debug {
		gin.SetMode(gin.DebugMode) // Dev Mode
	}

	// Logging
	gin.DefaultWriter = io.MultiWriter(helpers.BuildLogs())
	gin.DefaultErrorWriter = io.MultiWriter(helpers.BuildErrorLogs())

	// Gin Init
	r := gin.New()

	// Midlewares
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	r.GET("/download/:id/:quality", middlewares.VerifyAccess(cfg.AuthEndpoint), handler.GetVideo)
	r.GET("/download/:id/subtitle/:lang", middlewares.VerifyAccess(cfg.AuthEndpoint), handler.GetSubtitle)
	r.GET("/jobs/:job_id", handler.GetJob)
	r.GET("/buckets", handler.GetBucketList)

	srv := &http.Server{
		Addr:    ":" + cfg.HTTPPort,
		Handler: r.Handler(),
	}

	go func() {
		// service connections
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	log.Printf("🚀  escuchando en :%s", cfg.HTTPPort)

	// Shutdown ( Este proceso permite que antes de apagarse el sistema se terminen todas las tareas en ejecucion )
	quitSignal := make(chan os.Signal, 1)
	signal.Notify(quitSignal, syscall.SIGINT, syscall.SIGTERM)
	<-quitSignal
	log.Println("🛑  servidor detenido")

	workerCancel()
	workerRunner.Wait()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Println("Server Shutdown:", err)
	}
	<-ctx.Done()

	log.Println("Saliendo del servidor...")

}
