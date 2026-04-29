package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// using a typed string so we don't accidentally stomp on other context keys
type contextKey string

// key we use to stash the principal in request context
const principalContextKey contextKey = "principal"

// these are the two errors the bearer parser can throw, exported so callers can check them
var (
	ErrMissingAuthHeader = errors.New("missing Authorization header")
	ErrInvalidAuthHeader = errors.New("Authorization header must be Bearer <token>")
)

// Principal is basically "who is this user" - we pull this out of the jwt and pass it around
type Principal struct {
	UserID    int64    `json:"user_id"`
	Sub       string   `json:"sub"`
	Email     string   `json:"email,omitempty"`
	Roles     []string `json:"roles,omitempty"`
	SessionID string   `json:"session_id,omitempty"`
	ExpiresAt int64    `json:"expires_at,omitempty"`
}

// Validator is just an interface so we can swap in a fake for tests
type Validator interface {
	ValidateToken(ctx context.Context, rawToken string) (Principal, error)
}

// HMACValidator is the real one - it validates tokens using a shared hmac secret
type HMACValidator struct {
	Issuer   string
	Audience string
	Secret   []byte
}

// TokenIssuer is what we use to mint new session tokens after login
type TokenIssuer struct {
	Issuer   string
	Audience string
	Secret   []byte
	TTL      time.Duration
}

// ValidateToken parses and validates a jwt token string - returns the principal if its good
func (v HMACValidator) ValidateToken(_ context.Context, rawToken string) (Principal, error) {
	// no secret means we cant validate anything
	if len(v.Secret) == 0 {
		return Principal{}, errors.New("jwt secret is not configured")
	}

	// only add issuer/audience checks if those values are actually configured
	parseOptions := []jwt.ParserOption{}
	if strings.TrimSpace(v.Issuer) != "" {
		parseOptions = append(parseOptions, jwt.WithIssuer(v.Issuer))
	}
	if strings.TrimSpace(v.Audience) != "" {
		parseOptions = append(parseOptions, jwt.WithAudience(v.Audience))
	}

	// parse and verify the token signature - also checks expiry automatically
	tok, err := jwt.Parse(rawToken, func(token *jwt.Token) (any, error) {
		// make sure nobody slipped in a different signing algorithm
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method %q", token.Method.Alg())
		}
		return v.Secret, nil
	}, parseOptions...)
	if err != nil {
		return Principal{}, err
	}

	claims, ok := tok.Claims.(jwt.MapClaims)
	if !ok {
		return Principal{}, errors.New("invalid JWT claims")
	}

	// pull each field out of the claims map - these can be empty strings and thats ok
	sub, _ := claims["sub"].(string)
	email, _ := claims["email"].(string)
	sessionID, _ := claims["session_id"].(string)
	expiresAt := extractExpiry(claims)

	userID, err := extractUserID(claims)
	if err != nil {
		return Principal{}, err
	}

	return Principal{
		UserID:    userID,
		Sub:       strings.TrimSpace(sub),
		Email:     strings.TrimSpace(email),
		Roles:     extractRoles(claims),
		SessionID: strings.TrimSpace(sessionID),
		ExpiresAt: expiresAt,
	}, nil
}

// MintSessionToken creates a signed jwt token for the given user - called right after login
func (i TokenIssuer) MintSessionToken(userID int64, sub, email string, roles []string, sessionID string) (string, int64, error) {
	if len(i.Secret) == 0 {
		return "", 0, errors.New("token issuer secret is not configured")
	}
	// user_id zero or negative means something went wrong upstream
	if userID <= 0 {
		return "", 0, errors.New("user_id must be positive")
	}

	// default to 12 hours if no ttl was set in config
	ttlDuration := i.TTL
	if ttlDuration <= 0 {
		ttlDuration = 12 * time.Hour
	}
	expiresAt := time.Now().Add(ttlDuration).Unix()

	// build the claims map - iat is issued-at, exp is expiry
	claims := jwt.MapClaims{
		"iss":        i.Issuer,
		"aud":        i.Audience,
		"sub":        strings.TrimSpace(sub),
		"user_id":    userID,
		"email":      strings.TrimSpace(email),
		"roles":      roles,
		"session_id": strings.TrimSpace(sessionID),
		"iat":        time.Now().Unix(),
		"exp":        expiresAt,
	}

	signedToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	rawSignedString, err := signedToken.SignedString(i.Secret)
	if err != nil {
		return "", 0, err
	}
	return rawSignedString, expiresAt, nil
}

// GenerateSessionID makes a random hex string to use as a session id
// falls back to timestamp based id if the random read somehow fails
func GenerateSessionID() string {
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return fmt.Sprintf("sess-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(randomBytes)
}

// RequireAuth is a middleware that blocks requests with no valid bearer token
func RequireAuth(v Validator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// grab the token from the authorization header
			rawToken, err := ParseBearerToken(r.Header.Get("Authorization"))
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}

			// validate the token and get the principal out of it
			principal, err := v.ValidateToken(r.Context(), rawToken)
			if err != nil {
				http.Error(w, "invalid auth token", http.StatusUnauthorized)
				return
			}

			// stash the principal in context so handlers downstream can use it
			next.ServeHTTP(w, r.WithContext(withPrincipal(r.Context(), principal)))
		})
	}
}

// RequireAnyRole blocks requests where the user doesn't have at least one of the given roles
func RequireAnyRole(roles ...string) func(http.Handler) http.Handler {
	// precompute the allowed set so we don't re-build it on every request
	allowedRoles := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		normalizedRole := strings.TrimSpace(strings.ToLower(role))
		if normalizedRole == "" {
			continue
		}
		allowedRoles[normalizedRole] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := PrincipalFromContext(r.Context())
			if !ok {
				http.Error(w, "missing auth principal", http.StatusUnauthorized)
				return
			}

			// check if any of the user's roles match what we need
			for _, role := range principal.Roles {
				if _, exists := allowedRoles[strings.ToLower(strings.TrimSpace(role))]; exists {
					next.ServeHTTP(w, r)
					return
				}
			}

			http.Error(w, "insufficient role", http.StatusForbidden)
		})
	}
}

// PrincipalFromContext pulls the principal back out of context - returns false if its not there
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalContextKey).(Principal)
	return principal, ok
}

// WithPrincipal is the exported version - used by tests and other packages
func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return withPrincipal(ctx, principal)
}

// withPrincipal is the internal version - stuffs the principal into context
func withPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalContextKey, principal)
}

// ParseBearerToken extracts the token string from an authorization header value
// expects "Bearer <token>" format
func ParseBearerToken(headerValue string) (string, error) {
	trimmedHeader := strings.TrimSpace(headerValue)
	if trimmedHeader == "" {
		return "", ErrMissingAuthHeader
	}
	// split on first space to get ["Bearer", "<token>"]
	headerParts := strings.SplitN(trimmedHeader, " ", 2)
	if len(headerParts) != 2 || !strings.EqualFold(headerParts[0], "Bearer") {
		return "", ErrInvalidAuthHeader
	}
	tokenValue := strings.TrimSpace(headerParts[1])
	if tokenValue == "" {
		return "", ErrInvalidAuthHeader
	}
	return tokenValue, nil
}

// extractUserID pulls user_id out of jwt claims - handles float64, int64, and string types
// jwt lib decodes numbers as float64 by default which is annoying
func extractUserID(claims jwt.MapClaims) (int64, error) {
	rawUserID, ok := claims["user_id"]
	if !ok {
		// no user_id claim at all is fine - just return 0
		return 0, nil
	}

	switch typedValue := rawUserID.(type) {
	case float64:
		// json numbers come in as float64 from jwt library
		if typedValue <= 0 {
			return 0, errors.New("user_id claim must be positive")
		}
		return int64(typedValue), nil
	case int64:
		if typedValue <= 0 {
			return 0, errors.New("user_id claim must be positive")
		}
		return typedValue, nil
	case string:
		// sometimes ids come in as strings, handle that too
		parsedID, convErr := parsePositiveInt(typedValue)
		if convErr != nil {
			return 0, errors.New("user_id claim must be numeric")
		}
		return parsedID, nil
	default:
		return 0, errors.New("user_id claim has unsupported type")
	}
}

// extractExpiry pulls the exp claim out - returns 0 if its missing or unreadable
func extractExpiry(claims jwt.MapClaims) int64 {
	rawExp, ok := claims["exp"]
	if !ok {
		return 0
	}
	switch typedExp := rawExp.(type) {
	case float64:
		return int64(typedExp)
	case int64:
		return typedExp
	case string:
		parsedExp, err := parsePositiveInt(typedExp)
		if err != nil {
			return 0
		}
		return parsedExp
	default:
		return 0
	}
}

// parsePositiveInt parses a string as a positive int64 without using strconv
// we do this manually to avoid import and handle the edge cases ourselves
func parsePositiveInt(rawValue string) (int64, error) {
	trimmedValue := strings.TrimSpace(rawValue)
	if trimmedValue == "" {
		return 0, errors.New("invalid integer")
	}
	var parsedResult int64
	for _, characterByte := range trimmedValue {
		if characterByte < '0' || characterByte > '9' {
			return 0, errors.New("invalid integer")
		}
		parsedResult = parsedResult*10 + int64(characterByte-'0')
	}
	if parsedResult <= 0 {
		return 0, errors.New("must be positive")
	}
	return parsedResult, nil
}

// extractRoles pulls the roles array out of claims - handles multiple formats because jwts are inconsistent
func extractRoles(claims jwt.MapClaims) []string {
	roles := make([]string, 0)
	appendRoleIfNotEmpty := func(roleValue string) {
		trimmedRole := strings.TrimSpace(roleValue)
		if trimmedRole == "" {
			return
		}
		roles = append(roles, trimmedRole)
	}

	if rawRoles, ok := claims["roles"]; ok {
		switch typedRoles := rawRoles.(type) {
		case []any:
			// most common case - array of interface{} from json decode
			for _, item := range typedRoles {
				if stringItem, ok := item.(string); ok {
					appendRoleIfNotEmpty(stringItem)
				}
			}
		case []string:
			for _, item := range typedRoles {
				appendRoleIfNotEmpty(item)
			}
		case string:
			// single role as a string - uncommon but handle it
			appendRoleIfNotEmpty(typedRoles)
		}
	}

	return roles
}