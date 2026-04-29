package store

import "context"

func (s *Store) CreateAuditEvent(ctx context.Context, in AuditEventInput) error {
	// audit writes are fire and forget from caller perspective
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO audit_events (
			actor_user_id, event_type, target_type, target_id, status, message, metadata_json, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, unixepoch())
	`, in.ActorUserID, in.EventType, in.TargetType, nullIfEmpty(in.TargetID), in.Status, nullIfEmpty(in.Message), nullIfEmpty(in.Metadata))
	return err
}
