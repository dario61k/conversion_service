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
	"github.com/dario61k/conversion-service/internal/db"
	"github.com/dario61k/conversion-service/internal/domain"
	"github.com/dario61k/conversion-service/internal/handlers"
	"github.com/dario61k/conversion-service/internal/services"
	"github.com/dario61k/conversion-service/internal/storage"
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
	downloaderService := services.NewDowloaderService(cfg, repository, store)
	handler := handlers.NewHandler(downloaderService)

	cp := domain.CronParams{
		Repo : repository,
		Store : store,
		Cfg: &cfg,
	}

	cron := cron.Start(&cp)
	defer cron.Stop()

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

	r.GET("/download/:id/:quality", handler.GetVideo)
	r.GET("/download/:id/subtitle/:lang", handler.GetSubtitle)
	r.GET("/buckets", handler.GetBucketList)

	srv := &http.Server{
		Addr:    ":8080",
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Println("Server Shutdown:", err)
	}
	<-ctx.Done()

	log.Println("Saliendo del servidor...")

}
