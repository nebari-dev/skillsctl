package main

import (
	"context"
	"log"
	"net/http"
	"os"

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
	defer db.Close()

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
	} else {
		log.Println("auth disabled (no OIDC_ISSUER_URL)")
	}

	repo := sqlitestore.New(db)
	srv := server.New(repo, validator, authCfg)

	log.Printf("starting server on :%s (db: %s)", port, dbPath)
	if err := http.ListenAndServe(":"+port, srv.Handler()); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
