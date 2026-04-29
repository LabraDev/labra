package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"labra-backend/internal/api/store"
)

// AIRuntimeConfig holds all the config for the ai feature - loaded from env at startup
type AIRuntimeConfig struct {
	AIEnabled     bool
	DisableAI     bool
	PromptVersion string
	ProviderModel   string
	BedrockRegion   string
	ProviderTimeout time.Duration
	ProviderRetries int
	OpenAIAPIKey    string
	OpenAIBaseURL   string
	Provider        AIProvider
}

// AIProviderInput is what we send to the ai provider
type AIProviderInput struct {
	Prompt        string
	PromptVersion string
	Model         string
}

// AIProviderOutput is what we get back from the ai provider
type AIProviderOutput struct {
	Text       string
	Provider   string
	Model      string
	Confidence string
}

// AIProvider is an interface so we can swap between bedrock and openai without changing call sites
type AIProvider interface {
	Generate(ctx context.Context, in AIProviderInput) (AIProviderOutput, error)
}

// these are the runtime state vars for ai - set once at startup via InitAIRuntime
var (
	aiEnabled       = true
	aiDisabled      = false
	aiPromptVersion = "v1"
	aiProviderModel   = "us.amazon.nova-lite-v1:0"
	aiBedrockRegion   = "us-west-1"
	aiProviderTimeout = 1800 * time.Millisecond
	aiProviderRetries = 2
	aiProvider        AIProvider
)

// these regexes strip sensitive data from prompts before we send them to the ai provider
var (
	emailRegex    = regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`)
	awsAKRegex    = regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)
	secretKVRegex = regexp.MustCompile(`(?i)(aws_secret_access_key|api[_-]?key|token|password)\s*[:=]\s*([^\s,;]+)`)
	bearerRegex   = regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9\-._~+/]+=*`)
)

// aiDeployInsightRequest is the body the frontend posts to get ai insights for a deployment
type aiDeployInsightRequest struct {
	DeploymentID int64  `json:"deployment_id"`
	Prompt       string `json:"prompt"`
}

// InitAIRuntime sets up the ai provider and config from the runtime config struct
// called once at startup after config is loaded
func InitAIRuntime(cfg AIRuntimeConfig) {
	aiEnabled = cfg.AIEnabled
	aiDisabled = cfg.DisableAI

	// only override defaults if the config has actual values
	if strings.TrimSpace(cfg.PromptVersion) != "" {
		aiPromptVersion = strings.TrimSpace(cfg.PromptVersion)
	}
	if strings.TrimSpace(cfg.ProviderModel) != "" {
		aiProviderModel = strings.TrimSpace(cfg.ProviderModel)
	}
	if strings.TrimSpace(cfg.BedrockRegion) != "" {
		aiBedrockRegion = strings.TrimSpace(cfg.BedrockRegion)
	}
	if cfg.ProviderTimeout > 0 {
		aiProviderTimeout = cfg.ProviderTimeout
	}
	if cfg.ProviderRetries >= 0 {
		aiProviderRetries = cfg.ProviderRetries
	}

	// if a provider was injected directly (e.g. in tests) use that
	if cfg.Provider != nil {
		aiProvider = cfg.Provider
		return
	}

	// if openai key is set, use openai instead of bedrock
	if strings.TrimSpace(cfg.OpenAIAPIKey) != "" {
		aiProvider = newOpenAIResponsesProvider(strings.TrimSpace(cfg.OpenAIAPIKey), strings.TrimSpace(cfg.OpenAIBaseURL))
		// switch to a gpt model if the model was still set to an amazon model
		if strings.TrimSpace(aiProviderModel) == "" || strings.Contains(strings.ToLower(aiProviderModel), "amazon.") {
			aiProviderModel = "gpt-4.1-mini"
		}
		return
	}

	// default to bedrock
	aiProvider = newBedrockConverseProvider(aiBedrockRegion)
}

// PostAIDeployInsightsHandler handles POST /v1/ai/deploy-insights
// takes a deployment id and optional user prompt, returns ai-generated insights about the deploy
func PostAIDeployInsightsHandler(w http.ResponseWriter, r *http.Request) {
	if appStore == nil {
		writeJSONError(w, http.StatusInternalServerError, "store not initialized")
		return
	}

	userID, ok := readUserID(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "missing auth principal")
		return
	}

	var requestBody aiDeployInsightRequest
	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if requestBody.DeploymentID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "deployment_id must be a positive integer")
		return
	}

	// load the deployment to make sure it exists and belongs to this user
	targetDeployment, err := appStore.GetDeploymentByIDForUser(r.Context(), requestBody.DeploymentID, userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSONError(w, http.StatusNotFound, "deployment not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to load deployment")
		return
	}

	// load logs to give the ai context about what happened
	deploymentLogs, err := appStore.ListDeploymentLogs(r.Context(), targetDeployment.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to load deployment logs")
		return
	}

	// optionally load the app record for extra context
	var associatedApp *store.App
	if appValue, appErr := appStore.GetAppByIDForUser(r.Context(), targetDeployment.AppID, userID); appErr == nil {
		associatedApp = &appValue
	}

	// build the prompt from all the context we have
	builtPromptText := buildDeployPrompt(targetDeployment, associatedApp, deploymentLogs, strings.TrimSpace(requestBody.Prompt))
	// always redact before sending - even internally we don't want sensitive data in ai calls
	redactedPromptText, promptWasRedacted := redactSensitive(builtPromptText)

	// these track what we ended up returning
	resultInsightText := ""
	resultProviderName := "fallback"
	resultModelName := "n/a"
	resultConfidenceLevel := "low"
	fallbackWasUsed := true
	aiCallStatus := "fallback"
	aiCallFailureReason := ""

	switch {
	case !aiEnabled || aiDisabled:
		// ai is off - return a fallback response immediately
		disabledReason := "AI feature is disabled"
		if aiDisabled {
			disabledReason = "AI kill switch enabled"
		}
		resultInsightText = fallbackInsight(targetDeployment, deploymentLogs, disabledReason)
		aiCallStatus = "disabled_fallback"
	default:
		// try to get a real response from the ai provider
		aiProviderResponse, aiCallErr := invokeAIWithRetries(r.Context(), AIProviderInput{
			Prompt:        redactedPromptText,
			PromptVersion: aiPromptVersion,
			Model:         aiProviderModel,
		})
		if aiCallErr != nil {
			// ai failed - use fallback and keep the error reason for the audit log
			aiCallFailureReason = aiCallErr.Error()
			resultInsightText = fallbackInsight(targetDeployment, deploymentLogs, "AI provider unavailable")
			aiCallStatus = "fallback"
		} else {
			// ai succeeded - use the response
			resultInsightText = strings.TrimSpace(aiProviderResponse.Text)
			resultProviderName = strings.TrimSpace(aiProviderResponse.Provider)
			if resultProviderName == "" {
				resultProviderName = "provider"
			}
			resultModelName = strings.TrimSpace(aiProviderResponse.Model)
			if resultModelName == "" {
				resultModelName = aiProviderModel
			}
			resultConfidenceLevel = strings.TrimSpace(aiProviderResponse.Confidence)
			if resultConfidenceLevel == "" {
				resultConfidenceLevel = "medium"
			}
			fallbackWasUsed = false
			aiCallStatus = "succeeded"
		}
	}

	// cap the output length so we don't store huge strings
	resultInsightText = limitLen(strings.TrimSpace(resultInsightText), 1500)
	if resultInsightText == "" {
		// empty response from ai - fall back
		resultInsightText = fallbackInsight(targetDeployment, deploymentLogs, "empty AI response")
		fallbackWasUsed = true
		aiCallStatus = "fallback"
		resultProviderName = "fallback"
		resultModelName = "n/a"
		resultConfidenceLevel = "low"
	}

	// persist a log entry for this ai request so we can audit it later
	savedLogEntry, logSaveErr := appStore.CreateAIRequestLog(r.Context(), store.CreateAIRequestLogInput{
		UserID:        userID,
		DeploymentID:  targetDeployment.ID,
		PromptVersion: aiPromptVersion,
		Provider:      resultProviderName,
		Model:         resultModelName,
		InputRedacted: promptWasRedacted,
		FallbackUsed:  fallbackWasUsed,
		Status:        aiCallStatus,
		InputExcerpt:  limitLen(redactedPromptText, 500),
		OutputExcerpt: limitLen(resultInsightText, 500),
		OutputText:    limitLen(resultInsightText, 4000),
	})
	if logSaveErr != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to persist ai request log")
		return
	}

	// audit event for tracking who called the ai feature and with what result
	auditMetadataJSON, _ := json.Marshal(map[string]any{
		"deployment_id": targetDeployment.ID,
		"provider":      resultProviderName,
		"model":         resultModelName,
		"fallback_used": fallbackWasUsed,
		"status":        aiCallStatus,
	})
	_ = appStore.CreateAuditEvent(r.Context(), store.AuditEventInput{
		ActorUserID: userID,
		EventType:   "ai.deploy_insight",
		TargetType:  "deployment",
		TargetID:    strconv.FormatInt(targetDeployment.ID, 10),
		Status:      aiCallStatus,
		Message:     strings.TrimSpace(aiCallFailureReason),
		Metadata:    string(auditMetadataJSON),
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"deployment_id":  targetDeployment.ID,
		"insight":        resultInsightText,
		"source":         resultProviderName,
		"model":          resultModelName,
		"prompt_version": aiPromptVersion,
		"fallback_used":  fallbackWasUsed,
		"confidence":     resultConfidenceLevel,
		"request_log":    savedLogEntry,
	})
}

// GetAIRequestLogsHandler handles GET /v1/ai/logs
// returns recent ai request logs for the current user, optionally filtered by deployment
func GetAIRequestLogsHandler(w http.ResponseWriter, r *http.Request) {
	if appStore == nil {
		writeJSONError(w, http.StatusInternalServerError, "store not initialized")
		return
	}

	userID, ok := readUserID(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "missing auth principal")
		return
	}

	// parse optional limit query param
	pageLimit := 20
	if rawLimitString := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimitString != "" {
		parsedLimit, err := strconv.Atoi(rawLimitString)
		if err != nil || parsedLimit <= 0 {
			writeJSONError(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		pageLimit = parsedLimit
	}

	// parse optional deployment_id filter
	filterDeploymentID := int64(0)
	if rawDeployIDString := strings.TrimSpace(r.URL.Query().Get("deployment_id")); rawDeployIDString != "" {
		parsedDeployID, err := strconv.ParseInt(rawDeployIDString, 10, 64)
		if err != nil || parsedDeployID <= 0 {
			writeJSONError(w, http.StatusBadRequest, "deployment_id must be a positive integer")
			return
		}
		// verify the deployment belongs to this user before filtering by it
		if _, err := appStore.GetDeploymentByIDForUser(r.Context(), parsedDeployID, userID); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeJSONError(w, http.StatusNotFound, "deployment not found")
				return
			}
			writeJSONError(w, http.StatusInternalServerError, "failed to validate deployment")
			return
		}
		filterDeploymentID = parsedDeployID
	}

	// either filter by deployment or return all for this user
	var aiRequestLogs []store.AIRequestLog
	var logsLoadErr error
	if filterDeploymentID > 0 {
		aiRequestLogs, logsLoadErr = appStore.ListAIRequestLogsByUserForDeployment(r.Context(), userID, filterDeploymentID, pageLimit)
	} else {
		aiRequestLogs, logsLoadErr = appStore.ListAIRequestLogsByUser(r.Context(), userID, pageLimit)
	}
	if logsLoadErr != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to load ai request logs")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"logs":  aiRequestLogs,
		"count": len(aiRequestLogs),
	})
}

// invokeAIWithRetries calls the ai provider with automatic retries on failure
// respects aiProviderRetries and aiProviderTimeout settings
func invokeAIWithRetries(ctx context.Context, providerInput AIProviderInput) (AIProviderOutput, error) {
	if aiProvider == nil {
		return AIProviderOutput{}, fmt.Errorf("ai provider is not configured")
	}

	// total attempts = retries + 1 (the first try counts)
	totalAttempts := aiProviderRetries + 1
	if totalAttempts < 1 {
		totalAttempts = 1
	}

	var lastAttemptError error
	for attemptIndex := 0; attemptIndex < totalAttempts; attemptIndex++ {
		// fresh timeout context per attempt
		attemptCtx, cancelAttempt := context.WithTimeout(ctx, aiProviderTimeout)
		providerResult, attemptErr := aiProvider.Generate(attemptCtx, providerInput)
		cancelAttempt()
		if attemptErr == nil {
			// got a response - check it's not empty before accepting it
			if strings.TrimSpace(providerResult.Text) == "" {
				lastAttemptError = fmt.Errorf("ai provider returned empty output")
				continue
			}
			return providerResult, nil
		}
		lastAttemptError = attemptErr
	}

	if lastAttemptError == nil {
		lastAttemptError = fmt.Errorf("ai provider failed")
	}
	return AIProviderOutput{}, lastAttemptError
}

// buildDeployPrompt constructs the prompt we send to the ai from a deployment + logs
func buildDeployPrompt(dep store.Deployment, app *store.App, logs []store.DeploymentLog, userPromptText string) string {
	promptParts := []string{
		"You are a deployment assistant for Labra. Use only the provided context, and if data is missing say so clearly.",
		fmt.Sprintf("Deployment id: %d", dep.ID),
		fmt.Sprintf("Status: %s", dep.Status),
		fmt.Sprintf("Trigger: %s", dep.TriggerType),
		fmt.Sprintf("Branch: %s", dep.Branch),
		fmt.Sprintf("Failure reason: %s", dep.FailureReason),
	}
	// include app context if we have it
	if app != nil {
		promptParts = append(promptParts,
			fmt.Sprintf("App id: %d", app.ID),
			fmt.Sprintf("App name: %s", app.Name),
			fmt.Sprintf("Repository: %s", app.RepoFullName),
			fmt.Sprintf("Configured app branch: %s", app.Branch),
			fmt.Sprintf("Auto deploy enabled: %t", app.AutoDeployEnabled),
			fmt.Sprintf("Current site URL: %s", app.SiteURL),
		)
	}
	if dep.CommitSHA != "" {
		promptParts = append(promptParts, fmt.Sprintf("Commit: %s", dep.CommitSHA))
	}

	// include up to 10 log entries - more than that gets noisy
	maxLogEntries := len(logs)
	if maxLogEntries > 10 {
		maxLogEntries = 10
	}
	for logIndex := 0; logIndex < maxLogEntries; logIndex++ {
		logMessageText := strings.TrimSpace(logs[logIndex].Message)
		if logMessageText == "" {
			continue
		}
		promptParts = append(promptParts, fmt.Sprintf("Log %d (%s): %s", logIndex+1, logs[logIndex].LogLevel, limitLen(logMessageText, 180)))
	}

	// tack on the user's own question if they provided one
	trimmedUserPrompt := strings.TrimSpace(userPromptText)
	if trimmedUserPrompt != "" {
		promptParts = append(promptParts, "User ask: "+limitLen(trimmedUserPrompt, 500))
	}

	return strings.Join(promptParts, "\n")
}

// fallbackInsight generates a simple text insight when the ai provider isn't available
func fallbackInsight(dep store.Deployment, logs []store.DeploymentLog, reasonForFallback string) string {
	deploymentStatusValue := strings.TrimSpace(dep.Status)
	if deploymentStatusValue == "" {
		deploymentStatusValue = "unknown"
	}
	baseInsightText := fmt.Sprintf("Fallback insight (%s): deployment status is %s.", strings.TrimSpace(reasonForFallback), deploymentStatusValue)
	// add the last log message for extra context
	if len(logs) > 0 {
		lastLogMessage := strings.TrimSpace(logs[len(logs)-1].Message)
		if lastLogMessage != "" {
			return baseInsightText + " Latest log: " + limitLen(lastLogMessage, 180)
		}
	}
	if strings.TrimSpace(dep.FailureReason) != "" {
		return baseInsightText + " Failure reason: " + limitLen(dep.FailureReason, 180)
	}
	return baseInsightText + " Review deployment logs for detailed diagnostics."
}

// redactSensitive strips emails, aws keys, bearer tokens, and secret key=value pairs from text
// returns the cleaned text and whether anything was actually redacted
func redactSensitive(inputText string) (string, bool) {
	redactedText := inputText
	redactedText = emailRegex.ReplaceAllString(redactedText, "[REDACTED_EMAIL]")
	redactedText = awsAKRegex.ReplaceAllString(redactedText, "[REDACTED_AWS_ACCESS_KEY]")
	redactedText = bearerRegex.ReplaceAllString(redactedText, "Bearer [REDACTED_TOKEN]")
	redactedText = secretKVRegex.ReplaceAllString(redactedText, "$1=[REDACTED]")
	return redactedText, redactedText != inputText
}

// limitLen truncates a string to max characters, appending "..." if truncated
func limitLen(inputString string, maxLength int) string {
	if maxLength <= 0 {
		return ""
	}
	trimmedInput := strings.TrimSpace(inputString)
	if len(trimmedInput) <= maxLength {
		return trimmedInput
	}
	if maxLength <= 3 {
		return trimmedInput[:maxLength]
	}
	return trimmedInput[:maxLength-3] + "..."
}
