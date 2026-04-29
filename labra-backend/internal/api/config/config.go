package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

// these are the only envs we allow - anything else is a misconfiguration we catch early
var allowedEnvironments = map[string]struct{}{
	"local": {},
	"dev":   {},
	"stage": {},
	"prod":  {},
}

// Config holds everything the app needs to boot - loaded once at startup from env vars
type Config struct {
	Environment            string
	Host                   string
	Port                   int
	DBURL                  string
	GHClientID             string
	GHClientSecret         string
	GHAppID                string
	GHAppPrivateKeyPEM     string
	GHAppSlug              string
	GitHubOAuthRedirectURL string
	GitHubWebhookSecret    string
	JWTIssuer              string
	JWTAudience            string
	JWTSigningSecret       string
	AIEnabled       bool
	DisableAI       bool
	AIPromptVersion string
	AIProviderModel        string
	AIBedrockRegion        string
	AIProviderTimeoutMS    int
	AIProviderRetries      int
	OpenAIAPIKey           string
	OpenAIBaseURL          string
	LogLevel               slog.Level
}

// ListenAddress returns the host:port string the server should bind to
func (c Config) ListenAddress() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// LoadFromEnv is the production entry point - reads config from real env vars
func LoadFromEnv() (Config, error) {
	return Load(func(key string) string {
		return os.Getenv(key)
	})
}

// Load is the testable version - accepts a getter func so tests can inject fake envs
func Load(getenv func(string) string) (Config, error) {
	// default to local if APP_ENV is not set
	environmentName := normalize(getenv("APP_ENV"))
	if environmentName == "" {
		environmentName = "local"
	}
	if _, ok := allowedEnvironments[environmentName]; !ok {
		return Config{}, fmt.Errorf("APP_ENV must be one of local/dev/stage/prod")
	}

	// default host is localhost for local dev
	apiHost := normalize(getenv("API_HOST"))
	if apiHost == "" {
		apiHost = "localhost"
	}

	// 8080 is our default port
	apiPort := 8080
	if rawPortString := normalize(getenv("API_PORT")); rawPortString != "" {
		parsedPort, err := strconv.Atoi(rawPortString)
		if err != nil || parsedPort < 1 || parsedPort > 65535 {
			return Config{}, fmt.Errorf("API_PORT must be an integer between 1 and 65535")
		}
		apiPort = parsedPort
	}

	// db url is required - no fallback makes sense here
	dbConnectionURL := normalize(getenv("DB_URL"))
	if dbConnectionURL == "" {
		return Config{}, fmt.Errorf("DB_URL is required")
	}

	logLevel, err := parseLogLevel(normalize(getenv("LOG_LEVEL")))
	if err != nil {
		return Config{}, err
	}

	// jwt needs all three or none - partial config is an error
	jwtIssuerValue := normalize(getenv("JWT_ISSUER"))
	jwtAudienceValue := normalize(getenv("JWT_AUDIENCE"))
	jwtSigningSecretValue := normalize(getenv("JWT_SIGNING_SECRET"))

	if jwtSigningSecretValue != "" && (jwtIssuerValue == "" || jwtAudienceValue == "") {
		return Config{}, fmt.Errorf("JWT_ISSUER and JWT_AUDIENCE are required when JWT_SIGNING_SECRET is set")
	}

	// ai is on by default, disabled flag is false by default
	aiEnabledFlag, err := parseBoolWithDefault(normalize(getenv("AI_ENABLED")), true)
	if err != nil {
		return Config{}, fmt.Errorf("AI_ENABLED must be true or false")
	}
	aiDisabledFlag, err := parseBoolWithDefault(normalize(getenv("AI_DISABLED")), false)
	if err != nil {
		return Config{}, fmt.Errorf("AI_DISABLED must be true or false")
	}

	aiPromptVersionValue := normalize(getenv("AI_PROMPT_VERSION"))
	if aiPromptVersionValue == "" {
		aiPromptVersionValue = "v1"
	}

	aiProviderModelName := normalize(getenv("AI_PROVIDER_MODEL"))
	if aiProviderModelName == "" {
		// nova lite is cheap and fast enough for our insight generation
		aiProviderModelName = "us.amazon.nova-lite-v1:0"
	}

	// bedrock region check - try a few env var names before falling back to us-west-1
	aiBedrockRegionValue := normalize(getenv("AI_BEDROCK_REGION"))
	if aiBedrockRegionValue == "" {
		aiBedrockRegionValue = normalize(getenv("AWS_REGION"))
	}
	if aiBedrockRegionValue == "" {
		aiBedrockRegionValue = normalize(getenv("AWS_DEFAULT_REGION"))
	}
	if aiBedrockRegionValue == "" {
		aiBedrockRegionValue = "us-west-1"
	}

	// 1800ms is the default timeout for ai provider calls
	aiProviderTimeoutMSValue := 1800
	if rawTimeoutString := normalize(getenv("AI_PROVIDER_TIMEOUT_MS")); rawTimeoutString != "" {
		parsedTimeout, err := strconv.Atoi(rawTimeoutString)
		if err != nil || parsedTimeout <= 0 {
			return Config{}, fmt.Errorf("AI_PROVIDER_TIMEOUT_MS must be a positive integer")
		}
		aiProviderTimeoutMSValue = parsedTimeout
	}

	// 2 retries by default - more than that and we're probably just hammering a broken provider
	aiProviderRetriesValue := 2
	if rawRetriesString := normalize(getenv("AI_PROVIDER_RETRIES")); rawRetriesString != "" {
		parsedRetries, err := strconv.Atoi(rawRetriesString)
		if err != nil || parsedRetries < 0 || parsedRetries > 10 {
			return Config{}, fmt.Errorf("AI_PROVIDER_RETRIES must be between 0 and 10")
		}
		aiProviderRetriesValue = parsedRetries
	}

	return Config{
		Environment:            environmentName,
		Host:                   apiHost,
		Port:                   apiPort,
		DBURL:                  dbConnectionURL,
		GHClientID:             normalize(getenv("GH_CLIENT_ID")),
		GHClientSecret:         normalize(getenv("GH_CLIENT_SECRET")),
		GHAppID:                normalize(getenv("GH_APP_ID")),
		GHAppPrivateKeyPEM:     normalize(getenv("GH_APP_PRIVATE_KEY_PEM")),
		GHAppSlug:              normalize(getenv("GH_APP_SLUG")),
		GitHubOAuthRedirectURL: normalize(getenv("GITHUB_OAUTH_REDIRECT_URL")),
		GitHubWebhookSecret:    normalize(getenv("GITHUB_WEBHOOK_SECRET")),
		JWTIssuer:              jwtIssuerValue,
		JWTAudience:            jwtAudienceValue,
		JWTSigningSecret:       jwtSigningSecretValue,
		AIEnabled:       aiEnabledFlag,
		DisableAI:       aiDisabledFlag,
		AIPromptVersion:        aiPromptVersionValue,
		AIProviderModel:        aiProviderModelName,
		AIBedrockRegion:        aiBedrockRegionValue,
		AIProviderTimeoutMS:    aiProviderTimeoutMSValue,
		AIProviderRetries:      aiProviderRetriesValue,
		OpenAIAPIKey:           normalize(getenv("OPENAI_API_KEY")),
		OpenAIBaseURL:          normalize(getenv("OPENAI_BASE_URL")),
		LogLevel:               logLevel,
	}, nil
}

// normalize just trims whitespace from env var values - we do this everywhere
func normalize(rawValue string) string {
	return strings.TrimSpace(rawValue)
}

// parseLogLevel converts a string log level to slog.Level
// defaults to info if empty or unrecognized
func parseLogLevel(rawLevelString string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(rawLevelString)) {
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

// parseBoolWithDefault parses a bool env var - returns the default if the string is empty
func parseBoolWithDefault(rawBoolString string, defaultValue bool) (bool, error) {
	if strings.TrimSpace(rawBoolString) == "" {
		return defaultValue, nil
	}
	parsedBool, err := strconv.ParseBool(strings.ToLower(strings.TrimSpace(rawBoolString)))
	if err != nil {
		return false, err
	}
	return parsedBool, nil
}