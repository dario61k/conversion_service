package main

import (
	"context"
	"database/sql"
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

	"github.com/dario61k/conversion-service/internal/db"
	"github.com/dario61k/conversion-service/internal/handlers"
	"github.com/dario61k/conversion-service/internal/services/downloader"
	"github.com/dario61k/conversion-service/internal/storage"
	"github.com/dario61k/conversion-service/internal/config"
)

func main() {
	cfg := config.Load()

	// Base de datos
	dbPool, err := sql.Open("pgx", cfg.PGDSN)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	dbPool.SetConnMaxIdleTime(5 * time.Minute)
	dbPool.SetMaxOpenConns(10)

	// Almacenamiento S3 compatible
	store, err := storage.New(cfg.MinioEndpoint, cfg.MinioAccessKey, cfg.MinioSecretKey, cfg.MinioUseSSL)
	if err != nil {
		log.Fatalf("minio: %v", err)
	}

	// Dependencias de dominio
	repo := db.New(dbPool)
	dl := downloader.New(cfg, repo, store)
	h := handlers.New(dl)

	// Router HTTP
	gin.SetMode(gin.ReleaseMode) // Prod Mode
	//gin.SetMode(gin.DebugMode) // Prod Mode

	// Logging
	gin.DefaultWriter = io.MultiWriter(helpers.BuildLogs())
	gin.DefaultErrorWriter = io.MultiWriter(helpers.BuildErrorLogs())

	// Gin Init
	r := gin.New()

	// Midlewares
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	r.GET("/download/:id/:quality", h.GetVideo)
	r.GET("/download/:id/subtitle/:lang", h.GetSubtitle)
	r.GET("/buckets", h.GetBucketList)

	// Tarea de limpieza
	//cron := cleanup.Start(cfg, store)
	//defer cron.Stop()

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
