package handlers

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// checkoutGitHubRepository clones a github repo into a temp directory using a github app install token
// uses a git-askpass helper script to pass credentials without them appearing in ps output
func checkoutGitHubRepository(
	ctx context.Context,
	deploymentID int64,
	workspaceDir string,
	installationToken string,
	repoFullName string,
	branch string,
) (string, string, error) {
	// validate the repo name looks like owner/repo before we use it in a url
	repoNameValue := strings.TrimSpace(repoFullName)
	if !repoPattern.MatchString(repoNameValue) {
		return "", "", fmt.Errorf("repo_full_name must look like owner/repo")
	}

	// default to main if no branch was given
	branchNameValue := strings.TrimSpace(branch)
	if branchNameValue == "" {
		branchNameValue = "main"
	}

	// make sure git is actually installed in this container
	if _, err := exec.LookPath("git"); err != nil {
		return "", "", fmt.Errorf("git binary is not available in this runtime")
	}

	// write a tiny shell script that feeds credentials to git when asked
	// this avoids putting the token in the url or in git config
	askPassScriptPath := filepath.Join(workspaceDir, "git-askpass.sh")
	askPassScriptContent := strings.Join([]string{
		"#!/bin/sh",
		"case \"$1\" in",
		"*sername*) echo \"x-access-token\" ;;",
		"*assword*) echo \"$LABRA_GH_INSTALL_TOKEN\" ;;",
		"*) echo \"\" ;;",
		"esac",
		"",
	}, "\n")
	if err := os.WriteFile(askPassScriptPath, []byte(askPassScriptContent), 0o700); err != nil {
		return "", "", fmt.Errorf("failed to prepare git credential helper: %w", err)
	}

	// target directory for the clone
	cloneDestinationDir := filepath.Join(workspaceDir, "repo")
	cloneURL := "https://github.com/" + repoNameValue + ".git"

	// set env vars so git uses our askpass script and doesn't prompt interactively
	gitEnvVars := map[string]string{
		"GIT_ASKPASS":              askPassScriptPath,
		"GIT_TERMINAL_PROMPT":      "0",
		"LABRA_GH_INSTALL_TOKEN":   installationToken,
		"GIT_ASKPASS_REQUIRE_PASS": "force",
	}

	// shallow clone of a single branch - faster than a full clone for large repos
	_, cloneErr := runCommand(
		ctx,
		checkoutTimeout,
		workspaceDir,
		gitEnvVars,
		"git",
		"clone",
		"--depth",
		"1",
		"--single-branch",
		"--branch",
		branchNameValue,
		cloneURL,
		cloneDestinationDir,
	)
	if cloneErr != nil {
		return "", "", fmt.Errorf("failed to clone repository branch %q: %w", branchNameValue, cloneErr)
	}
	_ = appStore.CreateDeploymentLog(ctx, deploymentID, "info", fmt.Sprintf("checked out repository %s (%s)", repoNameValue, branchNameValue))

	// get the commit sha so we can store it on the deployment record
	rawCommitSHAOutput, shaErr := runCommand(ctx, 30*time.Second, cloneDestinationDir, nil, "git", "rev-parse", "HEAD")
	if shaErr != nil {
		// not fatal - we just won't have a commit sha
		return cloneDestinationDir, "", nil
	}
	return cloneDestinationDir, strings.TrimSpace(rawCommitSHAOutput), nil
}