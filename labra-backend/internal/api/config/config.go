package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

var allowedEnvironments = map[string]struct{}{
	"local": {},
	"dev":   {},
	"stage": {},
	"prod":  {},
}

type Config struct {
	Environment            string
	Host                   string
	Port                   int
	DBURL                  string
	GHClientID             string
	GHClientSecret         string
	GitHubOAuthRedirectURL string
	GitHubWebhookSecret    string
	JWTIssuer              string
	JWTAudience            string
	JWTSigningSecret       string
	AIFeatureEnabled       bool
	AIKillSwitchEnabled    bool
	AIPromptVersion        string
	AIProviderModel        string
	AIProviderTimeoutMS    int
	AIProviderRetries      int
	LogLevel               slog.Level
}

func (c Config) ListenAddress() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func LoadFromEnv() (Config, error) {
	return Load(func(key string) string {
		return os.Getenv(key)
	})
}

func Load(getenv func(string) string) (Config, error) {
	environment := normalize(getenv("APP_ENV"))
	if environment == "" {
		environment = "local"
	}
	if _, ok := allowedEnvironments[environment]; !ok {
		return Config{}, fmt.Errorf("APP_ENV must be one of local/dev/stage/prod")
	}

	host := normalize(getenv("API_HOST"))
	if host == "" {
		host = "localhost"
	}

	port := 8080
	if rawPort := normalize(getenv("API_PORT")); rawPort != "" {
		parsed, err := strconv.Atoi(rawPort)
		if err != nil || parsed < 1 || parsed > 65535 {
			return Config{}, fmt.Errorf("API_PORT must be an integer between 1 and 65535")
		}
		port = parsed
	}

	dbURL := normalize(getenv("DB_URL"))
	if dbURL == "" {
		return Config{}, fmt.Errorf("DB_URL is required")
	}

	logLevel, err := parseLogLevel(normalize(getenv("LOG_LEVEL")))
	if err != nil {
		return Config{}, err
	}

	jwtIssuer := normalize(getenv("JWT_ISSUER"))
	jwtAudience := normalize(getenv("JWT_AUDIENCE"))
	jwtSecret := normalize(getenv("JWT_SIGNING_SECRET"))

	if jwtSecret != "" && (jwtIssuer == "" || jwtAudience == "") {
		return Config{}, fmt.Errorf("JWT_ISSUER and JWT_AUDIENCE are required when JWT_SIGNING_SECRET is set")
	}

	aiFeatureEnabled, err := parseBoolWithDefault(normalize(getenv("AI_FEATURE_ENABLED")), true)
	if err != nil {
		return Config{}, fmt.Errorf("AI_FEATURE_ENABLED must be true or false")
	}
	aiKillSwitchEnabled, err := parseBoolWithDefault(normalize(getenv("AI_KILL_SWITCH_ENABLED")), false)
	if err != nil {
		return Config{}, fmt.Errorf("AI_KILL_SWITCH_ENABLED must be true or false")
	}
	aiPromptVersion := normalize(getenv("AI_PROMPT_VERSION"))
	if aiPromptVersion == "" {
		aiPromptVersion = "phase7-v1"
	}
	aiProviderModel := normalize(getenv("AI_PROVIDER_MODEL"))
	if aiProviderModel == "" {
		aiProviderModel = "mock-ops-v1"
	}

	aiProviderTimeoutMS := 1800
	if raw := normalize(getenv("AI_PROVIDER_TIMEOUT_MS")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			return Config{}, fmt.Errorf("AI_PROVIDER_TIMEOUT_MS must be a positive integer")
		}
		aiProviderTimeoutMS = parsed
	}

	aiProviderRetries := 2
	if raw := normalize(getenv("AI_PROVIDER_RETRIES")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 || parsed > 10 {
			return Config{}, fmt.Errorf("AI_PROVIDER_RETRIES must be between 0 and 10")
		}
		aiProviderRetries = parsed
	}

	return Config{
		Environment:            environment,
		Host:                   host,
		Port:                   port,
		DBURL:                  dbURL,
		GHClientID:             normalize(getenv("GH_CLIENT_ID")),
		GHClientSecret:         normalize(getenv("GH_CLIENT_SECRET")),
		GitHubOAuthRedirectURL: normalize(getenv("GITHUB_OAUTH_REDIRECT_URL")),
		GitHubWebhookSecret:    normalize(getenv("GITHUB_WEBHOOK_SECRET")),
		JWTIssuer:              jwtIssuer,
		JWTAudience:            jwtAudience,
		JWTSigningSecret:       jwtSecret,
		AIFeatureEnabled:       aiFeatureEnabled,
		AIKillSwitchEnabled:    aiKillSwitchEnabled,
		AIPromptVersion:        aiPromptVersion,
		AIProviderModel:        aiProviderModel,
		AIProviderTimeoutMS:    aiProviderTimeoutMS,
		AIProviderRetries:      aiProviderRetries,
		LogLevel:               logLevel,
	}, nil
}

func normalize(v string) string {
	return strings.TrimSpace(v)
}

func parseLogLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("LOG_LEVEL must be one of debug/info/warn/error")
	}
}

func parseBoolWithDefault(raw string, def bool) (bool, error) {
	if strings.TrimSpace(raw) == "" {
		return def, nil
	}
	v, err := strconv.ParseBool(strings.ToLower(strings.TrimSpace(raw)))
	if err != nil {
		return false, err
	}
	return v, nil
}
