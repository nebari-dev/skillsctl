package main

import (
	"log"
	"net/http"
	"os"

	skillctlv1 "github.com/openteams-ai/skill-share/gen/go/skillctl/v1"

	"github.com/openteams-ai/skill-share/backend/internal/server"
	"github.com/openteams-ai/skill-share/backend/internal/store"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Seed with sample data for local development.
	// Will be replaced by PostgreSQL store in a later slice.
	seedSkills := []*skillctlv1.Skill{
		{
			Name:          "data-pipeline",
			Description:   "Data pipeline utilities for Spark and batch processing",
			Owner:         "data-eng",
			Tags:          []string{"data", "spark"},
			LatestVersion: "1.3.0",
			InstallCount:  47,
			Source:        skillctlv1.SkillSource_SKILL_SOURCE_INTERNAL,
		},
		{
			Name:          "code-review",
			Description:   "Code review helpers for Go and Python",
			Owner:         "platform",
			Tags:          []string{"review", "go", "python"},
			LatestVersion: "0.9.1",
			InstallCount:  23,
			Source:        skillctlv1.SkillSource_SKILL_SOURCE_INTERNAL,
		},
	}

	skillStore := store.NewMemory(seedSkills)
	srv := server.New(skillStore)

	log.Printf("starting server on :%s", port)
	if err := http.ListenAndServe(":"+port, srv.Handler()); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
