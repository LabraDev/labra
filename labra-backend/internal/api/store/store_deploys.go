package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

func (s *Store) CreateDeployment(ctx context.Context, in CreateDeploymentInput) (Deployment, error) {
	// started and finished can come later so they stay nullable on insert
	var startedAt any
	var finishedAt any
	if in.StartedAt > 0 {
		startedAt = in.StartedAt
	}
	if in.FinishedAt > 0 {
		finishedAt = in.FinishedAt
	}

	row := s.db.QueryRowContext(ctx, `
		INSERT INTO deployments (
			app_id, user_id, status, trigger_type, commit_sha, commit_message, commit_author, branch, site_url, failure_reason, correlation_id,
			created_at, updated_at, started_at, finished_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, unixepoch(), unixepoch(), ?, ?)
		RETURNING id, app_id, user_id, status, trigger_type, COALESCE(commit_sha, ''), COALESCE(commit_message, ''), COALESCE(commit_author, ''),
			COALESCE(branch, ''), COALESCE(site_url, ''), COALESCE(failure_reason, ''), COALESCE(correlation_id, ''), created_at, updated_at,
			COALESCE(started_at, 0), COALESCE(finished_at, 0)
	`, in.AppID, in.UserID, in.Status, in.TriggerType, nullIfEmpty(in.CommitSHA), nullIfEmpty(in.CommitMessage), nullIfEmpty(in.CommitAuthor),
		nullIfEmpty(in.Branch), nullIfEmpty(in.SiteURL), nullIfEmpty(in.FailureReason), nullIfEmpty(in.CorrelationID), startedAt, finishedAt)

	var dep Deployment
	if err := row.Scan(&dep.ID, &dep.AppID, &dep.UserID, &dep.Status, &dep.TriggerType, &dep.CommitSHA, &dep.CommitMessage, &dep.CommitAuthor,
		&dep.Branch, &dep.SiteURL, &dep.FailureReason, &dep.CorrelationID, &dep.CreatedAt, &dep.UpdatedAt, &dep.StartedAt, &dep.FinishedAt); err != nil {
		return Deployment{}, err
	}
	return dep, nil
}

func (s *Store) GetDeploymentByIDForUser(ctx context.Context, deploymentID, userID int64) (Deployment, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, app_id, user_id, status, trigger_type, COALESCE(commit_sha, ''), COALESCE(commit_message, ''), COALESCE(commit_author, ''),
			COALESCE(branch, ''), COALESCE(site_url, ''), COALESCE(failure_reason, ''), COALESCE(correlation_id, ''), created_at, updated_at,
			COALESCE(started_at, 0), COALESCE(finished_at, 0)
		FROM deployments
		WHERE id = ? AND user_id = ?
	`, deploymentID, userID)

	var dep Deployment
	if err := row.Scan(&dep.ID, &dep.AppID, &dep.UserID, &dep.Status, &dep.TriggerType, &dep.CommitSHA, &dep.CommitMessage, &dep.CommitAuthor,
		&dep.Branch, &dep.SiteURL, &dep.FailureReason, &dep.CorrelationID, &dep.CreatedAt, &dep.UpdatedAt, &dep.StartedAt, &dep.FinishedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Deployment{}, ErrNotFound
		}
		return Deployment{}, err
	}
	return dep, nil
}

func (s *Store) ListDeploymentsByAppForUser(ctx context.Context, appID, userID int64) ([]Deployment, error) {
	// timeline is newest first so ui can treat index zero as latest deploy
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, app_id, user_id, status, trigger_type, COALESCE(commit_sha, ''), COALESCE(commit_message, ''), COALESCE(commit_author, ''),
			COALESCE(branch, ''), COALESCE(site_url, ''), COALESCE(failure_reason, ''), COALESCE(correlation_id, ''), created_at, updated_at,
			COALESCE(started_at, 0), COALESCE(finished_at, 0)
		FROM deployments
		WHERE app_id = ? AND user_id = ?
		ORDER BY created_at DESC
	`, appID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Deployment, 0)
	for rows.Next() {
		var dep Deployment
		if err := rows.Scan(&dep.ID, &dep.AppID, &dep.UserID, &dep.Status, &dep.TriggerType, &dep.CommitSHA, &dep.CommitMessage, &dep.CommitAuthor,
			&dep.Branch, &dep.SiteURL, &dep.FailureReason, &dep.CorrelationID, &dep.CreatedAt, &dep.UpdatedAt, &dep.StartedAt, &dep.FinishedAt); err != nil {
			return nil, err
		}
		out = append(out, dep)
	}
	return out, rows.Err()
}

func (s *Store) UpdateDeploymentStatus(ctx context.Context, deploymentID int64, status, reason, siteURL string, startedAt, finishedAt int64) (Deployment, error) {
	// coalesce lets us patch only fields provided by caller
	var started any
	var finished any
	if startedAt > 0 {
		started = startedAt
	}
	if finishedAt > 0 {
		finished = finishedAt
	}

	row := s.db.QueryRowContext(ctx, `
		UPDATE deployments
		SET status = ?, failure_reason = ?, site_url = ?, updated_at = unixepoch(), started_at = COALESCE(?, started_at), finished_at = COALESCE(?, finished_at)
		WHERE id = ?
		RETURNING id, app_id, user_id, status, trigger_type, COALESCE(commit_sha, ''), COALESCE(commit_message, ''), COALESCE(commit_author, ''),
			COALESCE(branch, ''), COALESCE(site_url, ''), COALESCE(failure_reason, ''), COALESCE(correlation_id, ''), created_at, updated_at,
			COALESCE(started_at, 0), COALESCE(finished_at, 0)
	`, status, nullIfEmpty(reason), nullIfEmpty(siteURL), started, finished, deploymentID)

	var dep Deployment
	if err := row.Scan(&dep.ID, &dep.AppID, &dep.UserID, &dep.Status, &dep.TriggerType, &dep.CommitSHA, &dep.CommitMessage, &dep.CommitAuthor,
		&dep.Branch, &dep.SiteURL, &dep.FailureReason, &dep.CorrelationID, &dep.CreatedAt, &dep.UpdatedAt, &dep.StartedAt, &dep.FinishedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Deployment{}, ErrNotFound
		}
		return Deployment{}, err
	}
	return dep, nil
}

func (s *Store) CreateDeploymentLog(ctx context.Context, deploymentID int64, level, message string) error {
	// logs append only and keep db write path simple
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO deployment_logs (deployment_id, log_level, message, created_at)
		VALUES (?, ?, ?, unixepoch())
	`, deploymentID, level, message)
	return err
}

func (s *Store) ListDeploymentLogs(ctx context.Context, deploymentID int64) ([]DeploymentLog, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, deployment_id, log_level, message, created_at
		FROM deployment_logs
		WHERE deployment_id = ?
		ORDER BY created_at ASC, id ASC
	`, deploymentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]DeploymentLog, 0)
	for rows.Next() {
		var l DeploymentLog
		if err := rows.Scan(&l.ID, &l.DeploymentID, &l.LogLevel, &l.Message, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (s *Store) CreateAIRequestLog(ctx context.Context, in CreateAIRequestLogInput) (AIRequestLog, error) {
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO ai_request_logs (
			user_id, deployment_id, prompt_version, provider, model, input_redacted, fallback_used, status, input_excerpt, output_excerpt, output_text, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, unixepoch())
		RETURNING id, user_id, deployment_id, prompt_version, provider, model, input_redacted, fallback_used, status,
			COALESCE(input_excerpt, ''), COALESCE(output_excerpt, ''), COALESCE(output_text, ''), created_at
	`, in.UserID, in.DeploymentID, strings.TrimSpace(in.PromptVersion), strings.TrimSpace(in.Provider), strings.TrimSpace(in.Model),
		boolToInt(in.InputRedacted), boolToInt(in.FallbackUsed), strings.TrimSpace(in.Status), nullIfEmpty(in.InputExcerpt), nullIfEmpty(in.OutputExcerpt), nullIfEmpty(in.OutputText))

	var out AIRequestLog
	var inputRedactedInt int
	var fallbackUsedInt int
	if err := row.Scan(&out.ID, &out.UserID, &out.DeploymentID, &out.PromptVersion, &out.Provider, &out.Model, &inputRedactedInt, &fallbackUsedInt, &out.Status, &out.InputExcerpt, &out.OutputExcerpt, &out.OutputText, &out.CreatedAt); err != nil {
		return AIRequestLog{}, err
	}
	out.InputRedacted = inputRedactedInt == 1
	out.FallbackUsed = fallbackUsedInt == 1
	return out, nil
}

func (s *Store) ListAIRequestLogsByUser(ctx context.Context, userID int64, limit int) ([]AIRequestLog, error) {
	// clamp limits so one request cant ask for too much history
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, deployment_id, prompt_version, provider, model, input_redacted, fallback_used, status,
			COALESCE(input_excerpt, ''), COALESCE(output_excerpt, ''), COALESCE(output_text, ''), created_at
		FROM ai_request_logs
		WHERE user_id = ?
		ORDER BY created_at DESC, id DESC
		LIMIT ?
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]AIRequestLog, 0)
	for rows.Next() {
		var item AIRequestLog
		var inputRedactedInt int
		var fallbackUsedInt int
		if err := rows.Scan(&item.ID, &item.UserID, &item.DeploymentID, &item.PromptVersion, &item.Provider, &item.Model, &inputRedactedInt, &fallbackUsedInt, &item.Status, &item.InputExcerpt, &item.OutputExcerpt, &item.OutputText, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.InputRedacted = inputRedactedInt == 1
		item.FallbackUsed = fallbackUsedInt == 1
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) ListAIRequestLogsByUserForDeployment(ctx context.Context, userID int64, deploymentID int64, limit int) ([]AIRequestLog, error) {
	// same clamp rules as global ai log list
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, deployment_id, prompt_version, provider, model, input_redacted, fallback_used, status,
			COALESCE(input_excerpt, ''), COALESCE(output_excerpt, ''), COALESCE(output_text, ''), created_at
		FROM ai_request_logs
		WHERE user_id = ? AND deployment_id = ?
		ORDER BY created_at DESC, id DESC
		LIMIT ?
	`, userID, deploymentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]AIRequestLog, 0)
	for rows.Next() {
		var item AIRequestLog
		var inputRedactedInt int
		var fallbackUsedInt int
		if err := rows.Scan(&item.ID, &item.UserID, &item.DeploymentID, &item.PromptVersion, &item.Provider, &item.Model, &inputRedactedInt, &fallbackUsedInt, &item.Status, &item.InputExcerpt, &item.OutputExcerpt, &item.OutputText, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.InputRedacted = inputRedactedInt == 1
		item.FallbackUsed = fallbackUsedInt == 1
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) ClaimWebhookDelivery(ctx context.Context, appID int64, deliveryID, eventType, commitSHA string) (bool, error) {
	// unique key on delivery id makes this idempotent for webhook retries
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO webhook_deliveries (app_id, delivery_id, event_type, commit_sha, received_at)
		VALUES (?, ?, ?, ?, unixepoch())
	`, appID, deliveryID, eventType, nullIfEmpty(commitSHA))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique constraint failed") {
			return false, nil
		}
		return false, fmt.Errorf("insert webhook delivery: %w", err)
	}
	return true, nil
}
