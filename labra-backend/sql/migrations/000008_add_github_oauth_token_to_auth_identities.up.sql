ALTER TABLE auth_identities
  ADD COLUMN access_token TEXT;

ALTER TABLE auth_identities
  ADD COLUMN token_updated_at INTEGER;
