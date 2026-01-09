package storage

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"

	_ "github.com/lib/pq"
)

type Storage interface {
}

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore() (*PostgresStore, error) {
	dbPassword := os.Getenv("DB_PASSWORD")
	connStr := fmt.Sprintf("host=127.0.0.1 port=5433 user=postgres dbname=memory password=%s sslmode=disable", dbPassword)
	// ... rest of code
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		slog.Info("Got this error while trying to open a connection to the database ", "error", err)
		return nil, err
	}
	if err = db.Ping(); err != nil {
		slog.Info("Got this error while trying to ping the database ", "error", err)
		return nil, err
	}
	ps := &PostgresStore{
		db: db,
	}
	return ps, nil
}
