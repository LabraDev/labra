package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

func (s *Store) CreatePlatformUser(ctx context.Context, in CreatePlatformUserInput) (PlatformUser, error) {
	// keep default status simple so caller can omit it
	status := strings.TrimSpace(in.Status)
	if status == "" {
		status = "active"
	}

	row := s.db.QueryRowContext(ctx, `
		INSERT INTO platform_users (email, status, created_at, updated_at)
		VALUES (?, ?, unixepoch(), unixepoch())
		RETURNING id, COALESCE(email, ''), status, created_at, updated_at
	`, nullIfEmpty(in.Email), status)

	var out PlatformUser
	if err := row.Scan(&out.ID, &out.Email, &out.Status, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return PlatformUser{}, err
	}
	return out, nil
}

func (s *Store) GetPlatformUserByID(ctx context.Context, userID int64) (PlatformUser, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, COALESCE(email, ''), status, created_at, updated_at
		FROM platform_users
		WHERE id = ?
	`, userID)

	var out PlatformUser
	if err := row.Scan(&out.ID, &out.Email, &out.Status, &out.CreatedAt, &out.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PlatformUser{}, ErrNotFound
		}
		return PlatformUser{}, err
	}
	return out, nil
}

func (s *Store) GetPlatformUserByIdentity(ctx context.Context, provider, subject string) (PlatformUser, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT u.id, COALESCE(u.email, ''), u.status, u.created_at, u.updated_at
		FROM platform_users u
		INNER JOIN auth_identities ai ON ai.user_id = u.id
		WHERE ai.provider = ? AND ai.subject = ?
	`, strings.TrimSpace(provider), strings.TrimSpace(subject))

	var out PlatformUser
	if err := row.Scan(&out.ID, &out.Email, &out.Status, &out.CreatedAt, &out.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PlatformUser{}, ErrNotFound
		}
		return PlatformUser{}, err
	}
	return out, nil
}

func (s *Store) UpsertAuthIdentity(ctx context.Context, in UpsertAuthIdentityInput) (AuthIdentity, error) {
	// token only updates when caller sends a fresh one
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO auth_identities (
			user_id, provider, subject, email, access_token, token_updated_at, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, CASE WHEN ? IS NULL THEN NULL ELSE unixepoch() END, unixepoch(), unixepoch())
		ON CONFLICT(provider, subject) DO UPDATE SET
			user_id = excluded.user_id,
			email = excluded.email,
			access_token = COALESCE(excluded.access_token, auth_identities.access_token),
			token_updated_at = CASE
				WHEN excluded.access_token IS NULL THEN auth_identities.token_updated_at
				ELSE unixepoch()
			END,
			updated_at = unixepoch()
		RETURNING id, user_id, provider, subject, COALESCE(email, ''), COALESCE(access_token, ''), COALESCE(token_updated_at, 0), created_at, updated_at
	`, in.UserID, strings.TrimSpace(in.Provider), strings.TrimSpace(in.Subject), nullIfEmpty(in.Email), nullIfEmpty(in.AccessToken), nullIfEmpty(in.AccessToken))

	var out AuthIdentity
	if err := row.Scan(
		&out.ID,
		&out.UserID,
		&out.Provider,
		&out.Subject,
		&out.Email,
		&out.AccessToken,
		&out.TokenUpdatedAt,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		return AuthIdentity{}, err
	}
	return out, nil
}

func (s *Store) GetAuthIdentityByUserProvider(ctx context.Context, userID int64, provider string) (AuthIdentity, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, provider, subject, COALESCE(email, ''), COALESCE(access_token, ''), COALESCE(token_updated_at, 0), created_at, updated_at
		FROM auth_identities
		WHERE user_id = ? AND provider = ?
		ORDER BY updated_at DESC, id DESC
		LIMIT 1
	`, userID, strings.TrimSpace(provider))

	var out AuthIdentity
	if err := row.Scan(
		&out.ID,
		&out.UserID,
		&out.Provider,
		&out.Subject,
		&out.Email,
		&out.AccessToken,
		&out.TokenUpdatedAt,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AuthIdentity{}, ErrNotFound
		}
		return AuthIdentity{}, err
	}
	return out, nil
}

func (s *Store) UpsertGitHubInstallation(ctx context.Context, in UpsertGitHubInstallationInput) (GitHubInstallation, error) {
	// one install record per user keeps app access checks straightforward
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO github_installations (
			user_id, installation_id, account_login, target_type, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, unixepoch(), unixepoch())
		ON CONFLICT(user_id) DO UPDATE SET
			installation_id = excluded.installation_id,
			account_login = excluded.account_login,
			target_type = excluded.target_type,
			updated_at = unixepoch()
		RETURNING id, user_id, installation_id, COALESCE(account_login, ''), COALESCE(target_type, ''), created_at, updated_at
	`, in.UserID, in.InstallationID, nullIfEmpty(in.AccountLogin), nullIfEmpty(in.TargetType))

	var out GitHubInstallation
	if err := row.Scan(
		&out.ID,
		&out.UserID,
		&out.InstallationID,
		&out.AccountLogin,
		&out.TargetType,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		return GitHubInstallation{}, err
	}
	return out, nil
}

func (s *Store) GetGitHubInstallationByUserID(ctx context.Context, userID int64) (GitHubInstallation, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, installation_id, COALESCE(account_login, ''), COALESCE(target_type, ''), created_at, updated_at
		FROM github_installations
		WHERE user_id = ?
	`, userID)

	var out GitHubInstallation
	if err := row.Scan(
		&out.ID,
		&out.UserID,
		&out.InstallationID,
		&out.AccountLogin,
		&out.TargetType,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return GitHubInstallation{}, ErrNotFound
		}
		return GitHubInstallation{}, err
	}
	return out, nil
}

func (s *Store) CreateAuthSession(ctx context.Context, in CreateAuthSessionInput) (AuthSession, error) {
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO auth_sessions (session_id, user_id, expires_at, created_at)
		VALUES (?, ?, ?, unixepoch())
		RETURNING session_id, user_id, expires_at, created_at, COALESCE(revoked_at, 0)
	`, in.SessionID, in.UserID, in.ExpiresAt)

	var out AuthSession
	if err := row.Scan(&out.SessionID, &out.UserID, &out.ExpiresAt, &out.CreatedAt, &out.RevokedAt); err != nil {
		return AuthSession{}, err
	}
	return out, nil
}

func (s *Store) GetAuthSessionByID(ctx context.Context, sessionID string) (AuthSession, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT session_id, user_id, expires_at, created_at, COALESCE(revoked_at, 0)
		FROM auth_sessions
		WHERE session_id = ?
	`, strings.TrimSpace(sessionID))

	var out AuthSession
	if err := row.Scan(&out.SessionID, &out.UserID, &out.ExpiresAt, &out.CreatedAt, &out.RevokedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AuthSession{}, ErrNotFound
		}
		return AuthSession{}, err
	}
	return out, nil
}

func (s *Store) RevokeAuthSession(ctx context.Context, sessionID string) error {
	// soft revoke lets us audit old sessions later
	res, err := s.db.ExecContext(ctx, `
		UPDATE auth_sessions
		SET revoked_at = unixepoch()
		WHERE session_id = ? AND revoked_at IS NULL
	`, strings.TrimSpace(sessionID))
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}
