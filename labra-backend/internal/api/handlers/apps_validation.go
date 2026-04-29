package handlers

import (
	"fmt"
	"strings"

	"labra-backend/internal/api/store"
)

func normalizeCreateApp(req createAppRequest) (store.CreateAppInput, error) {
	name := strings.TrimSpace(req.Name)
	repo := strings.TrimSpace(req.RepoFullName)
	branch := strings.TrimSpace(req.Branch)
	buildType := strings.TrimSpace(req.BuildType)
	outputDir := strings.TrimSpace(req.OutputDir)
	rootDir := strings.TrimSpace(req.RootDir)
	siteURL := strings.TrimSpace(req.SiteURL)

	if name == "" {
		return store.CreateAppInput{}, fmt.Errorf("name is required")
	}
	if repo == "" {
		return store.CreateAppInput{}, fmt.Errorf("repo_full_name is required")
	}
	if !repoPattern.MatchString(repo) {
		return store.CreateAppInput{}, fmt.Errorf("repo_full_name must look like owner/repo")
	}
	if branch == "" {
		branch = "main"
	}
	if buildType == "" {
		buildType = "static"
	}
	if buildType != "static" {
		return store.CreateAppInput{}, fmt.Errorf("build_type must be static for MVP")
	}
	if outputDir == "" {
		outputDir = "dist"
	}

	autoDeploy := true
	if req.AutoDeployEnabled != nil {
		autoDeploy = *req.AutoDeployEnabled
	}

	return store.CreateAppInput{
		Name:              name,
		RepoFullName:      strings.ToLower(repo),
		Branch:            branch,
		BuildType:         buildType,
		OutputDir:         outputDir,
		RootDir:           rootDir,
		SiteURL:           siteURL,
		AutoDeployEnabled: autoDeploy,
	}, nil
}

func mergeAppUpdate(current store.App, req updateAppRequest) (store.UpdateAppInput, error) {
	next := store.UpdateAppInput{
		Name:              current.Name,
		Branch:            current.Branch,
		BuildType:         current.BuildType,
		OutputDir:         current.OutputDir,
		RootDir:           current.RootDir,
		SiteURL:           current.SiteURL,
		AutoDeployEnabled: current.AutoDeployEnabled,
	}

	if req.Name != nil {
		next.Name = strings.TrimSpace(*req.Name)
	}
	if req.Branch != nil {
		next.Branch = strings.TrimSpace(*req.Branch)
	}
	if req.BuildType != nil {
		next.BuildType = strings.TrimSpace(*req.BuildType)
	}
	if req.OutputDir != nil {
		next.OutputDir = strings.TrimSpace(*req.OutputDir)
	}
	if req.RootDir != nil {
		next.RootDir = strings.TrimSpace(*req.RootDir)
	}
	if req.SiteURL != nil {
		next.SiteURL = strings.TrimSpace(*req.SiteURL)
	}
	if req.AutoDeployEnabled != nil {
		next.AutoDeployEnabled = *req.AutoDeployEnabled
	}

	if next.Name == "" {
		return store.UpdateAppInput{}, fmt.Errorf("name cannot be empty")
	}
	if next.Branch == "" {
		return store.UpdateAppInput{}, fmt.Errorf("branch cannot be empty")
	}
	if next.BuildType == "" {
		next.BuildType = "static"
	}
	if next.BuildType != "static" {
		return store.UpdateAppInput{}, fmt.Errorf("build_type must be static for MVP")
	}
	if next.OutputDir == "" {
		next.OutputDir = "dist"
	}

	return next, nil
}

func shouldRecordConfigHistoryForPatch(current store.App, next store.UpdateAppInput) bool {
	return current.Name != next.Name ||
		current.Branch != next.Branch ||
		current.BuildType != next.BuildType ||
		current.OutputDir != next.OutputDir ||
		current.RootDir != next.RootDir ||
		current.SiteURL != next.SiteURL
}
