package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	awsverify "labra-backend/internal/api/aws"
	"labra-backend/internal/api/store"
)

// regionPattern validates aws region strings like us-west-2
var (
	regionPattern                                   = regexp.MustCompile(`^[a-z]{2}(-[a-z]+)+-[0-9]+$`)
	assumeRoleVerifier awsverify.AssumeRoleVerifier = awsverify.LocalAssumeRoleVerifier{}
)

// upsertAWSConnectionRequest is what the frontend sends to add or update an aws connection
type upsertAWSConnectionRequest struct {
	RoleARN    string `json:"role_arn"`
	ExternalID string `json:"external_id"`
	Region     string `json:"region"`
	AccountID  string `json:"account_id,omitempty"`
}

// InitAssumeRoleVerifier swaps in a fake verifier for testing - use real one in production
func InitAssumeRoleVerifier(v awsverify.AssumeRoleVerifier) {
	if v == nil {
		assumeRoleVerifier = awsverify.LocalAssumeRoleVerifier{}
		return
	}
	assumeRoleVerifier = v
}

// UpsertAWSConnectionHandler handles POST /v1/aws-connections
// validates the role arn can actually be assumed before storing anything
func UpsertAWSConnectionHandler(w http.ResponseWriter, r *http.Request) {
	if appStore == nil {
		writeJSONError(w, http.StatusInternalServerError, "store not initialized")
		return
	}

	userID, ok := readUserID(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "missing auth principal")
		return
	}

	var requestBody upsertAWSConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	// normalize and validate the input before we make any aws calls
	normalizedConnectionInput, err := normalizeAWSConnection(requestBody)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		_ = appStore.CreateAuditEvent(r.Context(), store.AuditEventInput{
			ActorUserID: userID,
			EventType:   "aws_connection.upsert",
			TargetType:  "aws_connection",
			Status:      "failed",
			Message:     err.Error(),
		})
		return
	}

	// actually try to assume the role - this catches bad arns and wrong external ids early
	verifiedAWSAccountID, err := assumeRoleVerifier.Verify(r.Context(), awsverify.AssumeRoleInput{
		RoleARN:    normalizedConnectionInput.RoleARN,
		ExternalID: normalizedConnectionInput.ExternalID,
		Region:     normalizedConnectionInput.Region,
	})
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("unable to validate AssumeRole configuration: %v", err))
		_ = appStore.CreateAuditEvent(r.Context(), store.AuditEventInput{
			ActorUserID: userID,
			EventType:   "aws_connection.assume_role_verify",
			TargetType:  "aws_connection",
			Status:      "failed",
			Message:     err.Error(),
		})
		return
	}

	// if user didn't provide an account id, use the one we got from sts
	if normalizedConnectionInput.AccountID == "" {
		normalizedConnectionInput.AccountID = verifiedAWSAccountID
	}
	// if they did provide one, make sure it matches what sts says
	if normalizedConnectionInput.AccountID != verifiedAWSAccountID {
		writeJSONError(w, http.StatusBadRequest, "account_id does not match role ARN account")
		_ = appStore.CreateAuditEvent(r.Context(), store.AuditEventInput{
			ActorUserID: userID,
			EventType:   "aws_connection.assume_role_verify",
			TargetType:  "aws_connection",
			Status:      "failed",
			Message:     "account_id does not match role ARN account",
		})
		return
	}

	// set the fields that we control server-side
	normalizedConnectionInput.UserID = userID
	normalizedConnectionInput.Status = "validated"
	normalizedConnectionInput.LastValidatedAt = store.UnixNow()

	// save the connection - upsert means update if already exists
	savedConnection, err := appStore.UpsertAWSConnection(r.Context(), normalizedConnectionInput)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to save aws connection")
		_ = appStore.CreateAuditEvent(r.Context(), store.AuditEventInput{
			ActorUserID: userID,
			EventType:   "aws_connection.upsert",
			TargetType:  "aws_connection",
			Status:      "failed",
			Message:     "failed to save aws connection",
		})
		return
	}

	// audit log the successful connection
	_ = appStore.CreateAuditEvent(r.Context(), store.AuditEventInput{
		ActorUserID: userID,
		EventType:   "aws_connection.upsert",
		TargetType:  "aws_connection",
		TargetID:    fmt.Sprintf("%d", savedConnection.ID),
		Status:      "success",
		Message:     "aws connection validated and saved",
	})

	writeJSON(w, http.StatusCreated, map[string]any{
		"connection": savedConnection,
	})
}

// ListAWSConnectionsHandler handles GET /v1/aws-connections
// returns all aws connections for the current user
func ListAWSConnectionsHandler(w http.ResponseWriter, r *http.Request) {
	if appStore == nil {
		writeJSONError(w, http.StatusInternalServerError, "store not initialized")
		return
	}

	userID, ok := readUserID(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "missing auth principal")
		return
	}

	allUserConnections, err := appStore.ListAWSConnectionsByUser(r.Context(), userID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to load aws connections")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"aws_connections": allUserConnections,
	})
}

// DeleteAWSConnectionHandler handles DELETE /v1/aws-connections/:id
// removes an aws connection for the current user
func DeleteAWSConnectionHandler(w http.ResponseWriter, r *http.Request) {
	if appStore == nil {
		writeJSONError(w, http.StatusInternalServerError, "store not initialized")
		return
	}

	userID, ok := readUserID(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "missing auth principal")
		return
	}

	connectionID, err := readIDFromPathOrQuery(r, "aws-connections")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	// delete returns false if the connection wasn't found (or didn't belong to this user)
	wasDeleted, err := appStore.DeleteAWSConnectionForUser(r.Context(), connectionID, userID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to delete aws connection")
		return
	}
	if !wasDeleted {
		writeJSONError(w, http.StatusNotFound, "aws connection not found")
		return
	}

	// audit log the deletion
	_ = appStore.CreateAuditEvent(r.Context(), store.AuditEventInput{
		ActorUserID: userID,
		EventType:   "aws_connection.delete",
		TargetType:  "aws_connection",
		TargetID:    fmt.Sprintf("%d", connectionID),
		Status:      "success",
		Message:     "aws connection deleted",
	})

	// 204 means success with no response body
	w.WriteHeader(http.StatusNoContent)
}

// normalizeAWSConnection validates and trims the incoming aws connection request
func normalizeAWSConnection(requestData upsertAWSConnectionRequest) (store.UpsertAWSConnectionInput, error) {
	trimmedRoleARN := strings.TrimSpace(requestData.RoleARN)
	trimmedExternalID := strings.TrimSpace(requestData.ExternalID)
	trimmedRegion := strings.TrimSpace(requestData.Region)
	trimmedAccountID := strings.TrimSpace(requestData.AccountID)

	if trimmedRoleARN == "" {
		return store.UpsertAWSConnectionInput{}, fmt.Errorf("role_arn is required")
	}
	if trimmedExternalID == "" {
		return store.UpsertAWSConnectionInput{}, fmt.Errorf("external_id is required")
	}
	// external id needs to be long enough to be meaningful as a secret
	if len(trimmedExternalID) < 8 || len(trimmedExternalID) > 128 {
		return store.UpsertAWSConnectionInput{}, fmt.Errorf("external_id must be between 8 and 128 characters")
	}
	// region needs to look like a real aws region
	if !regionPattern.MatchString(trimmedRegion) {
		return store.UpsertAWSConnectionInput{}, fmt.Errorf("region must look like us-west-2")
	}
	// account id if provided must be exactly 12 digits
	if trimmedAccountID != "" && (len(trimmedAccountID) != 12 || strings.Trim(trimmedAccountID, "0123456789") != "") {
		return store.UpsertAWSConnectionInput{}, fmt.Errorf("account_id must be a 12-digit AWS account number")
	}

	return store.UpsertAWSConnectionInput{
		RoleARN:    trimmedRoleARN,
		ExternalID: trimmedExternalID,
		Region:     trimmedRegion,
		AccountID:  trimmedAccountID,
	}, nil
}
