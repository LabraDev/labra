package store

import "context"

func (s *Store) UpsertAWSConnection(ctx context.Context, in UpsertAWSConnectionInput) (AWSConnection, error) {
	// last validated is optional because first save might be unvalidated
	var lastValidatedAt any
	if in.LastValidatedAt > 0 {
		lastValidatedAt = in.LastValidatedAt
	}

	row := s.db.QueryRowContext(ctx, `
		INSERT INTO aws_connections (
			user_id, role_arn, external_id, region, account_id, status, last_validated_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, unixepoch(), unixepoch())
		ON CONFLICT(user_id, role_arn, region) DO UPDATE SET
			external_id = excluded.external_id,
			account_id = excluded.account_id,
			status = excluded.status,
			last_validated_at = excluded.last_validated_at,
			updated_at = unixepoch()
		RETURNING id, user_id, role_arn, external_id, region, account_id, status, COALESCE(last_validated_at, 0), created_at, updated_at
	`, in.UserID, in.RoleARN, in.ExternalID, in.Region, in.AccountID, in.Status, lastValidatedAt)

	var out AWSConnection
	if err := row.Scan(&out.ID, &out.UserID, &out.RoleARN, &out.ExternalID, &out.Region, &out.AccountID, &out.Status, &out.LastValidatedAt, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return AWSConnection{}, err
	}
	return out, nil
}

func (s *Store) ListAWSConnectionsByUser(ctx context.Context, userID int64) ([]AWSConnection, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, role_arn, external_id, region, account_id, status, COALESCE(last_validated_at, 0), created_at, updated_at
		FROM aws_connections
		WHERE user_id = ?
		ORDER BY updated_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]AWSConnection, 0)
	for rows.Next() {
		var c AWSConnection
		if err := rows.Scan(&c.ID, &c.UserID, &c.RoleARN, &c.ExternalID, &c.Region, &c.AccountID, &c.Status, &c.LastValidatedAt, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) DeleteAWSConnectionForUser(ctx context.Context, connectionID, userID int64) (bool, error) {
	// bool return keeps handler logic simple for not found responses
	res, err := s.db.ExecContext(ctx, `
		DELETE FROM aws_connections
		WHERE id = ? AND user_id = ?
	`, connectionID, userID)
	if err != nil {
		return false, err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}
