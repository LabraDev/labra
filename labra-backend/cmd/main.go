package main

import (
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"labra-backend/internal/api/auth"
	awsverify "labra-backend/internal/api/aws"
	"labra-backend/internal/api/config"
	"labra-backend/internal/api/handlers"
	"labra-backend/internal/api/middleware"
	"labra-backend/internal/api/routes"
	"labra-backend/internal/api/services"

	"github.com/go-fuego/fuego"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/lpernett/godotenv"
)

func main() {
	_ = godotenv.Load("../.env", ".env")

	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	if cfg.GHClientID != "" && cfg.GHClientSecret != "" {
		services.InitOauth(cfg.GHClientID, cfg.GHClientSecret, cfg.GitHubOAuthRedirectURL)
	} else {
		logger.Warn("GitHub OAuth is not configured; /v1/login and /v1/callback will not work")
	}
	handlers.InitGitHubAppRuntime(cfg.GHAppID, cfg.GHAppPrivateKeyPEM, cfg.GHAppSlug)

	if err := ensureSQLiteDir(cfg.DBURL); err != nil {
		log.Fatalf("prepare db path: %v", err)
	}

	db, err := sql.Open("sqlite3", cfg.DBURL)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	migrationSource, err := resolveMigrationSource()
	if err != nil {
		log.Fatalf("resolve migrations: %v", err)
	}

	if err := runMigrations(db, migrationSource); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	handlers.InitAppStore(db)
	handlers.InitWebhook(cfg.GitHubWebhookSecret)
	handlers.InitReadiness(db.PingContext)
	if cfg.Environment == "local" {
		handlers.InitAssumeRoleVerifier(awsverify.LocalAssumeRoleVerifier{})
	} else {
		handlers.InitAssumeRoleVerifier(awsverify.NewSTSAssumeRoleVerifier())
	}
	handlers.InitAIRuntime(handlers.AIRuntimeConfig{
		AIEnabled: cfg.AIEnabled,
		DisableAI: cfg.DisableAI,
		PromptVersion:   cfg.AIPromptVersion,
		ProviderModel:   cfg.AIProviderModel,
		BedrockRegion:   cfg.AIBedrockRegion,
		ProviderTimeout: time.Duration(cfg.AIProviderTimeoutMS) * time.Millisecond,
		ProviderRetries: cfg.AIProviderRetries,
		OpenAIAPIKey:    cfg.OpenAIAPIKey,
		OpenAIBaseURL:   cfg.OpenAIBaseURL,
	})

	validator := &auth.HMACValidator{
		Issuer:   strings.TrimSpace(cfg.JWTIssuer),
		Audience: strings.TrimSpace(cfg.JWTAudience),
		Secret:   []byte(strings.TrimSpace(cfg.JWTSigningSecret)),
	}
	routes.InitAuthMiddleware(validator)
	handlers.InitAuthRuntime(auth.TokenIssuer{
		Issuer:   strings.TrimSpace(cfg.JWTIssuer),
		Audience: strings.TrimSpace(cfg.JWTAudience),
		Secret:   []byte(strings.TrimSpace(cfg.JWTSigningSecret)),
		TTL:      12 * time.Hour,
	})

	s := fuego.NewServer(
		fuego.WithAddr(cfg.ListenAddress()),
	)
	fuego.Use(s, middleware.RequestContext(logger))

	routes.HealthRoute(s)
	routes.Oauth(s)
	routes.GitHubRoutes(s)
	routes.AuthRoutes(s)
	routes.AWSConnections(s)
	routes.Apps(s)
	routes.Deploy(s)
	routes.AIRoutes(s)
	routes.SystemRoutes(s)
	routes.Webhooks(s)

	logger.Info("server starting", "addr", cfg.ListenAddress(), "env", cfg.Environment)
	s.Run()
}

func runMigrations(db *sql.DB, migrationSource string) error {
	driver, err := sqlite3.WithInstance(db, &sqlite3.Config{})
	if err != nil {
		return err
	}

	m, err := migrate.NewWithDatabaseInstance(
		migrationSource,
		"sqlite3",
		driver,
	)
	if err != nil {
		return err
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}

func resolveMigrationSource() (string, error) {
	candidates := []string{
		"../sql/migrations",
		"./sql/migrations",
		"/app/sql/migrations",
	}

	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && info.IsDir() {
			return "file://" + candidate, nil
		}
	}

	return "", fmt.Errorf("sql migrations directory not found in known paths: %s", strings.Join(candidates, ", "))
}

func ensureSQLiteDir(dbURL string) error {
	trimmed := strings.TrimSpace(dbURL)
	if trimmed == "" || trimmed == ":memory:" || strings.HasPrefix(trimmed, "file:") {
		return nil
	}

	dbPath := strings.SplitN(trimmed, "?", 2)[0]
	dir := filepath.Dir(dbPath)
	if dir == "" || dir == "." {
		return nil
	}

	return os.MkdirAll(dir, 0o755)
}
