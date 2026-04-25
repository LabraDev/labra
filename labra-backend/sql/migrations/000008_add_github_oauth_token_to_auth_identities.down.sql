ALTER TABLE auth_identities
  DROP COLUMN token_updated_at;

ALTER TABLE auth_identities
  DROP COLUMN access_token;
