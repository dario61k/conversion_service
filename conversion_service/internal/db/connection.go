package db

import (
	"database/sql"
	"log"
	"time"
)

func NewDBPool(dns string, connMaxIdle, maxOpenConns int) *sql.DB {

	dbPool, err := sql.Open("pgx", dns)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}

	dbPool.SetConnMaxIdleTime(time.Duration(connMaxIdle) * time.Minute)
	dbPool.SetMaxOpenConns(maxOpenConns)

	return dbPool
}