package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// githubWebhookSecret is the shared secret used to verify webhook payloads from github
var githubWebhookSecret string

// webhookMaxSkewSeconds is the replay window - payloads older than this get rejected
var webhookMaxSkewSeconds int64 = 5 * 60

// webhookNowUnix is injectable for tests so we can fake the current time
var webhookNowUnix = func() int64 { return time.Now().Unix() }

// githubPushEvent is the shape of a push webhook payload from github
type githubPushEvent struct {
	Ref        string `json:"ref"`
	After      string `json:"after"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
	HeadCommit struct {
		ID      string `json:"id"`
		Message string `json:"message"`
		Author  struct {
			Name string `json:"name"`
		} `json:"author"`
	} `json:"head_commit"`
}

// webhookResolution holds what we figured out from processing a push event
type webhookResolution struct {
	RepoFullName string
	Branch       string
	MatchedApps  int
	EligibleApps []map[string]any
	Ignored      bool
	Reason       string
}

// InitWebhook sets the github webhook secret at startup
func InitWebhook(webhookSecretValue string) {
	githubWebhookSecret = strings.TrimSpace(webhookSecretValue)
}

// GitHubWebhookHandler handles incoming webhook events from github
// verifies signature, checks freshness, then routes to push event handling
func GitHubWebhookHandler(w http.ResponseWriter, r *http.Request) {
	if appStore == nil {
		writeJSONError(w, http.StatusInternalServerError, "store not initialized")
		return
	}
	if githubWebhookSecret == "" {
		writeJSONError(w, http.StatusInternalServerError, "github webhook secret is not configured")
		return
	}

	// read the full body so we can verify the signature
	rawRequestBody, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "unable to read request body")
		return
	}

	// verify the hmac signature before doing anything else
	webhookSignatureHeader := strings.TrimSpace(r.Header.Get("X-Hub-Signature-256"))
	if !isValidGitHubSignature(githubWebhookSecret, rawRequestBody, webhookSignatureHeader) {
		writeJSONError(w, http.StatusUnauthorized, "invalid webhook signature")
		return
	}
	// check the timestamp isn't too old (replay protection)
	if err := validateWebhookFreshness(r); err != nil {
		writeWebhookError(w, err)
		return
	}

	// grab event type and delivery id from headers
	githubEventType := strings.ToLower(strings.TrimSpace(r.Header.Get("X-GitHub-Event")))
	webhookDeliveryID := strings.TrimSpace(r.Header.Get("X-GitHub-Delivery"))

	// we only handle push events right now - acknowledge everything else without processing
	if githubEventType != "push" {
		writeJSON(w, http.StatusAccepted, map[string]any{
			"accepted":    true,
			"ignored":     true,
			"delivery_id": webhookDeliveryID,
			"event_type":  githubEventType,
			"reason":      "event type is not supported",
		})
		return
	}

	// parse the push event payload
	var pushEventPayload githubPushEvent
	if err := json.Unmarshal(rawRequestBody, &pushEventPayload); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid push payload")
		return
	}

	// figure out which apps match this push event
	pushEventResolution, err := resolvePushEventWithStore(r, pushEventPayload)
	if err != nil {
		writeWebhookError(w, err)
		return
	}

	// deduplicate to avoid triggering multiple deploys for the same delivery
	deduplicatedApps, duplicateDeliveryCount, err := dedupeEligibleAppsWithLedger(r, webhookDeliveryID, githubEventType, pushEventPayload, pushEventResolution.EligibleApps)
	if err != nil {
		writeWebhookError(w, err)
		return
	}

	// queue deployments for all eligible apps
	triggeredDeploymentsList, err := enqueueWebhookDeployments(r, pushEventPayload, deduplicatedApps)
	if err != nil {
		writeWebhookError(w, err)
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"accepted":              true,
		"ignored":               pushEventResolution.Ignored || len(deduplicatedApps) == 0,
		"delivery_id":           webhookDeliveryID,
		"event_type":            githubEventType,
		"repo_full_name":        pushEventResolution.RepoFullName,
		"branch":                pushEventResolution.Branch,
		"commit_sha":            strings.TrimSpace(pushEventPayload.After),
		"commit_message":        strings.TrimSpace(pushEventPayload.HeadCommit.Message),
		"commit_author":         strings.TrimSpace(pushEventPayload.HeadCommit.Author.Name),
		"matched_apps":          pushEventResolution.MatchedApps,
		"eligible_apps":         deduplicatedApps,
		"duplicate_count":       duplicateDeliveryCount,
		"triggered_count":       len(triggeredDeploymentsList),
		"triggered_deployments": triggeredDeploymentsList,
		"reason":                pushEventResolution.Reason,
	})
}

// validateWebhookFreshness checks the X-Labra-Webhook-Timestamp header
// rejects payloads older than webhookMaxSkewSeconds to prevent replay attacks
func validateWebhookFreshness(r *http.Request) error {
	rawTimestampHeader := strings.TrimSpace(r.Header.Get("X-Labra-Webhook-Timestamp"))
	if rawTimestampHeader == "" {
		// no timestamp header is fine - freshness check is optional
		return nil
	}

	parsedTimestamp, err := strconv.ParseInt(rawTimestampHeader, 10, 64)
	if err != nil || parsedTimestamp <= 0 {
		return errWebhook("invalid webhook timestamp header")
	}

	currentTimeUnix := webhookNowUnix()
	timestampDelta := currentTimeUnix - parsedTimestamp
	if timestampDelta < 0 {
		timestampDelta = -timestampDelta
	}
	if timestampDelta > webhookMaxSkewSeconds {
		return errWebhook("webhook timestamp outside allowed replay window")
	}
	return nil
}

// isValidGitHubSignature verifies the hmac-sha256 signature on a github webhook payload
func isValidGitHubSignature(sharedSecret string, requestBody []byte, signatureHeader string) bool {
	if !strings.HasPrefix(signatureHeader, "sha256=") {
		return false
	}

	// decode the expected signature from the header
	signatureHexPart := strings.TrimPrefix(signatureHeader, "sha256=")
	expectedSignatureBytes, err := hex.DecodeString(signatureHexPart)
	if err != nil {
		return false
	}

	// compute the actual hmac signature of the body
	hmacHasher := hmac.New(sha256.New, []byte(sharedSecret))
	_, _ = hmacHasher.Write(requestBody)
	computedSignatureBytes := hmacHasher.Sum(nil)
	// constant time comparison to avoid timing attacks
	return hmac.Equal(computedSignatureBytes, expectedSignatureBytes)
}

// extractBranch parses a git ref like "refs/heads/main" into just "main"
func extractBranch(gitRef string) (string, bool) {
	const refsHeadsPrefix = "refs/heads/"
	if !strings.HasPrefix(gitRef, refsHeadsPrefix) {
		// not a branch ref - could be a tag or something else
		return "", false
	}

	branchName := strings.TrimSpace(strings.TrimPrefix(gitRef, refsHeadsPrefix))
	if branchName == "" {
		return "", false
	}

	return branchName, true
}

// webhookErr is a typed error for webhook-specific bad request errors
type webhookErr string

func (e webhookErr) Error() string { return string(e) }

func errWebhook(message string) error {
	return webhookErr(message)
}

// writeWebhookError sends a 400 for webhook errors, 500 for everything else
func writeWebhookError(w http.ResponseWriter, err error) {
	if _, isWebhookErr := err.(webhookErr); isWebhookErr {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONError(w, http.StatusInternalServerError, err.Error())
}

// mustInt64 converts various numeric types to int64 - needed because json unmarshals numbers differently depending on context
func mustInt64(numericValue any) (int64, error) {
	switch typedNumericValue := numericValue.(type) {
	case int64:
		return typedNumericValue, nil
	case int:
		return int64(typedNumericValue), nil
	case float64:
		// json numbers come in as float64 by default
		return int64(typedNumericValue), nil
	case string:
		parsedInt, err := strconv.ParseInt(strings.TrimSpace(typedNumericValue), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid numeric value %q", typedNumericValue)
		}
		return parsedInt, nil
	default:
		return 0, fmt.Errorf("unsupported numeric type %T", numericValue)
	}
}
