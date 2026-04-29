package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

func (s *Store) CreateApp(ctx context.Context, in CreateAppInput) (App, error) {
	// insert and return in one round trip so caller gets db defaults instantly
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO apps (
			user_id, name, repo_full_name, branch, build_type, output_dir, root_dir, site_url, auto_deploy_enabled, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, unixepoch(), unixepoch())
		RETURNING id, user_id, name, repo_full_name, branch, build_type, output_dir, COALESCE(root_dir, ''), COALESCE(site_url, ''), auto_deploy_enabled, created_at, updated_at
	`, in.UserID, in.Name, in.RepoFullName, in.Branch, in.BuildType, in.OutputDir, nullIfEmpty(in.RootDir), nullIfEmpty(in.SiteURL), boolToInt(in.AutoDeployEnabled))

	var app App
	var autoDeployInt int
	if err := row.Scan(&app.ID, &app.UserID, &app.Name, &app.RepoFullName, &app.Branch, &app.BuildType, &app.OutputDir, &app.RootDir, &app.SiteURL, &autoDeployInt, &app.CreatedAt, &app.UpdatedAt); err != nil {
		return App{}, err
	}
	app.AutoDeployEnabled = autoDeployInt == 1
	return app, nil
}

func (s *Store) ListAppsByUser(ctx context.Context, userID int64) ([]App, error) {
	// default apps list sorted by freshest updates first
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, name, repo_full_name, branch, build_type, output_dir, COALESCE(root_dir, ''), COALESCE(site_url, ''), auto_deploy_enabled, created_at, updated_at
		FROM apps
		WHERE user_id = ?
		ORDER BY updated_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]App, 0)
	for rows.Next() {
		var app App
		var autoDeployInt int
		if err := rows.Scan(&app.ID, &app.UserID, &app.Name, &app.RepoFullName, &app.Branch, &app.BuildType, &app.OutputDir, &app.RootDir, &app.SiteURL, &autoDeployInt, &app.CreatedAt, &app.UpdatedAt); err != nil {
			return nil, err
		}
		app.AutoDeployEnabled = autoDeployInt == 1
		out = append(out, app)
	}
	return out, rows.Err()
}

func (s *Store) GetAppByIDForUser(ctx context.Context, appID, userID int64) (App, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, name, repo_full_name, branch, build_type, output_dir, COALESCE(root_dir, ''), COALESCE(site_url, ''), auto_deploy_enabled, created_at, updated_at
		FROM apps
		WHERE id = ? AND user_id = ?
	`, appID, userID)

	var app App
	var autoDeployInt int
	if err := row.Scan(&app.ID, &app.UserID, &app.Name, &app.RepoFullName, &app.Branch, &app.BuildType, &app.OutputDir, &app.RootDir, &app.SiteURL, &autoDeployInt, &app.CreatedAt, &app.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return App{}, ErrNotFound
		}
		return App{}, err
	}
	app.AutoDeployEnabled = autoDeployInt == 1
	return app, nil
}

func (s *Store) UpdateAppForUser(ctx context.Context, appID, userID int64, in UpdateAppInput) (App, error) {
	// user id is in where clause so users can only update their own app rows
	row := s.db.QueryRowContext(ctx, `
		UPDATE apps
		SET name = ?, branch = ?, build_type = ?, output_dir = ?, root_dir = ?, site_url = ?, auto_deploy_enabled = ?, updated_at = unixepoch()
		WHERE id = ? AND user_id = ?
		RETURNING id, user_id, name, repo_full_name, branch, build_type, output_dir, COALESCE(root_dir, ''), COALESCE(site_url, ''), auto_deploy_enabled, created_at, updated_at
	`, in.Name, in.Branch, in.BuildType, in.OutputDir, nullIfEmpty(in.RootDir), nullIfEmpty(in.SiteURL), boolToInt(in.AutoDeployEnabled), appID, userID)

	var app App
	var autoDeployInt int
	if err := row.Scan(&app.ID, &app.UserID, &app.Name, &app.RepoFullName, &app.Branch, &app.BuildType, &app.OutputDir, &app.RootDir, &app.SiteURL, &autoDeployInt, &app.CreatedAt, &app.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return App{}, ErrNotFound
		}
		return App{}, err
	}
	app.AutoDeployEnabled = autoDeployInt == 1
	return app, nil
}

func (s *Store) DeleteAppForUser(ctx context.Context, appID, userID int64) error {
	// we do manual cascade in a tx because sqlite schema can vary by migration state
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	deploymentIDs := make([]int64, 0)
	rows, err := tx.QueryContext(ctx, `
		SELECT id
		FROM deployments
		WHERE app_id = ? AND user_id = ?
	`, appID, userID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var depID int64
		if scanErr := rows.Scan(&depID); scanErr != nil {
			_ = rows.Close()
			err = scanErr
			return err
		}
		deploymentIDs = append(deploymentIDs, depID)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		_ = rows.Close()
		err = rowsErr
		return err
	}
	_ = rows.Close()

	for _, depID := range deploymentIDs {
		// clean child rows first so deploy history is fully removed
		if _, err = tx.ExecContext(ctx, `
			DELETE FROM deployment_logs
			WHERE deployment_id = ?
		`, depID); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `
			DELETE FROM ai_request_logs
			WHERE deployment_id = ?
		`, depID); err != nil {
			// old local db files may not have this table yet
			if !strings.Contains(strings.ToLower(err.Error()), "no such table") {
				return err
			}
		}
	}

	if _, err = tx.ExecContext(ctx, `
		DELETE FROM deployments
		WHERE app_id = ? AND user_id = ?
	`, appID, userID); err != nil {
		return err
	}

	if _, err = tx.ExecContext(ctx, `
		DELETE FROM webhook_deliveries
		WHERE app_id = ?
	`, appID); err != nil {
		// old local db files may not have this table yet
		if !strings.Contains(strings.ToLower(err.Error()), "no such table") {
			return err
		}
	}

	if _, err = tx.ExecContext(ctx, `
		DELETE FROM app_config_versions
		WHERE app_id = ? AND user_id = ?
	`, appID, userID); err != nil {
		return err
	}

	if _, err = tx.ExecContext(ctx, `
		DELETE FROM app_infra_outputs
		WHERE app_id = ? AND user_id = ?
	`, appID, userID); err != nil {
		return err
	}

	res, err := tx.ExecContext(ctx, `
		DELETE FROM apps
		WHERE id = ? AND user_id = ?
	`, appID, userID)
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

	err = tx.Commit()
	if err == nil {
		committed = true
	}
	return err
}

func (s *Store) CreateAppConfigVersion(ctx context.Context, in CreateAppConfigVersionInput) (AppConfigVersion, error) {
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO app_config_versions (app_id, user_id, source, config_json, created_at)
		VALUES (?, ?, ?, ?, unixepoch())
		RETURNING id, app_id, user_id, source, config_json, created_at
	`, in.AppID, in.UserID, strings.TrimSpace(in.Source), strings.TrimSpace(in.ConfigJSON))

	var out AppConfigVersion
	if err := row.Scan(&out.ID, &out.AppID, &out.UserID, &out.Source, &out.ConfigJSON, &out.CreatedAt); err != nil {
		return AppConfigVersion{}, err
	}
	return out, nil
}

func (s *Store) ListAppConfigVersionsByAppForUser(ctx context.Context, appID, userID int64) ([]AppConfigVersion, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, app_id, user_id, source, config_json, created_at
		FROM app_config_versions
		WHERE app_id = ? AND user_id = ?
		ORDER BY created_at DESC, id DESC
	`, appID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]AppConfigVersion, 0)
	for rows.Next() {
		var cfg AppConfigVersion
		if err := rows.Scan(&cfg.ID, &cfg.AppID, &cfg.UserID, &cfg.Source, &cfg.ConfigJSON, &cfg.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, cfg)
	}
	return out, rows.Err()
}

func (s *Store) UpsertAppInfraOutput(ctx context.Context, in UpsertAppInfraOutputInput) (AppInfraOutput, error) {
	// one row per app so upsert is the cleanest write model here
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO app_infra_outputs (app_id, user_id, bucket_name, distribution_id, site_url, updated_at)
		VALUES (?, ?, ?, ?, ?, unixepoch())
		ON CONFLICT(app_id) DO UPDATE SET
			user_id = excluded.user_id,
			bucket_name = excluded.bucket_name,
			distribution_id = excluded.distribution_id,
			site_url = excluded.site_url,
			updated_at = unixepoch()
		RETURNING app_id, user_id, bucket_name, distribution_id, COALESCE(site_url, ''), updated_at
	`, in.AppID, in.UserID, strings.TrimSpace(in.BucketName), strings.TrimSpace(in.DistributionID), nullIfEmpty(in.SiteURL))

	var out AppInfraOutput
	if err := row.Scan(&out.AppID, &out.UserID, &out.BucketName, &out.DistributionID, &out.SiteURL, &out.UpdatedAt); err != nil {
		return AppInfraOutput{}, err
	}
	return out, nil
}

func (s *Store) GetAppInfraOutputByAppForUser(ctx context.Context, appID, userID int64) (AppInfraOutput, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT app_id, user_id, bucket_name, distribution_id, COALESCE(site_url, ''), updated_at
		FROM app_infra_outputs
		WHERE app_id = ? AND user_id = ?
	`, appID, userID)

	var out AppInfraOutput
	if err := row.Scan(&out.AppID, &out.UserID, &out.BucketName, &out.DistributionID, &out.SiteURL, &out.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AppInfraOutput{}, ErrNotFound
		}
		return AppInfraOutput{}, err
	}
	return out, nil
}

func (s *Store) ListAutoDeployAppsByRepo(ctx context.Context, repoFullName string) ([]App, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, name, repo_full_name, branch, build_type, output_dir, COALESCE(root_dir, ''), COALESCE(site_url, ''), auto_deploy_enabled, created_at, updated_at
		FROM apps
		WHERE lower(repo_full_name) = lower(?) AND auto_deploy_enabled = 1
		ORDER BY id ASC
	`, repoFullName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]App, 0)
	for rows.Next() {
		var app App
		var autoDeployInt int
		if err := rows.Scan(&app.ID, &app.UserID, &app.Name, &app.RepoFullName, &app.Branch, &app.BuildType, &app.OutputDir, &app.RootDir, &app.SiteURL, &autoDeployInt, &app.CreatedAt, &app.UpdatedAt); err != nil {
			return nil, err
		}
		app.AutoDeployEnabled = autoDeployInt == 1
		out = append(out, app)
	}
	return out, rows.Err()
}
