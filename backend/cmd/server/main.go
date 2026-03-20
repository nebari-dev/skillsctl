package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/nebari-dev/skillsctl/backend/internal/auth"
	"github.com/nebari-dev/skillsctl/backend/internal/server"
	sqlitestore "github.com/nebari-dev/skillsctl/backend/internal/store/sqlite"
	"github.com/nebari-dev/skillsctl/backend/internal/store/sqlite/migrations"
)

func main() {
	port := envOr("PORT", "8080")
	dbPath := envOr("DB_PATH", "skillsctl.db")

	db, err := sqlitestore.Open(dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := migrations.Run(context.Background(), db); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	authCfg := auth.Config{
		IssuerURL:   envOr("OIDC_ISSUER_URL", ""),
		ClientID:    envOr("OIDC_CLIENT_ID", ""),
		AdminGroup:  envOr("OIDC_ADMIN_GROUP", "skillsctl-admins"),
		GroupsClaim: envOr("OIDC_GROUPS_CLAIM", "groups"),
	}

	if (authCfg.IssuerURL == "") != (authCfg.ClientID == "") {
		log.Fatalf("OIDC_ISSUER_URL and OIDC_CLIENT_ID must both be set or both be empty")
	}

	var validator auth.TokenValidator
	if authCfg.IssuerURL != "" {
		v, err := auth.NewValidator(context.Background(), authCfg)
		if err != nil {
			log.Fatalf("init auth: %v", err)
		}
		validator = v
		log.Printf("auth enabled (issuer: %s)", authCfg.IssuerURL)
	} else if isDevMode() {
		log.Println("WARNING: running in dev mode with authentication disabled")
	} else {
		log.Fatalf("OIDC_ISSUER_URL is required. Set DEV_MODE=true to run without authentication.")
	}

	repo := sqlitestore.New(db)
	handler := server.New(repo, validator, authCfg)

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handler.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	log.Printf("starting server on :%s (db: %s)", port, dbPath)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func isDevMode() bool {
	v := strings.ToLower(os.Getenv("DEV_MODE"))
	return v == "1" || v == "true" || v == "yes"
}
