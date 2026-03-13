package main

import (
	"context"
	"log"
	"net/http"
	"os"

	_ "modernc.org/sqlite"

	"github.com/nebari-dev/skillctl/backend/internal/server"
	sqlitestore "github.com/nebari-dev/skillctl/backend/internal/store/sqlite"
	"github.com/nebari-dev/skillctl/backend/internal/store/sqlite/migrations"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "skillctl.db"
	}

	db, err := sqlitestore.Open(dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	if err := migrations.Run(context.Background(), db); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	repo := sqlitestore.New(db)
	srv := server.New(repo)

	log.Printf("starting server on :%s (db: %s)", port, dbPath)
	if err := http.ListenAndServe(":"+port, srv.Handler()); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
