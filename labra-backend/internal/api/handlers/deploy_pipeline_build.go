package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// installDependenciesIfNeeded runs npm/yarn/pnpm install if there's a package.json
// detects which package manager to use based on lock file presence
func installDependenciesIfNeeded(ctx context.Context, deploymentID int64, buildRoot string) error {
	packageJSONPath := filepath.Join(buildRoot, "package.json")
	if _, err := os.Stat(packageJSONPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			_ = appStore.CreateDeploymentLog(ctx, deploymentID, "info", "package.json not found; skipping dependency install")
			return nil
		}
		return fmt.Errorf("failed to inspect package.json: %w", err)
	}

	// figure out which package manager this project uses
	detectedManagerName, installCommandArgs, err := detectInstallCommand(buildRoot)
	if err != nil {
		return err
	}
	if len(installCommandArgs) == 0 {
		return nil
	}

	_ = appStore.CreateDeploymentLog(ctx, deploymentID, "info", "install dependencies ("+detectedManagerName+")")
	// CI=true suppresses interactive prompts in some tools
	_, err = runCommand(ctx, installTimeout, buildRoot, map[string]string{"CI": "true"}, installCommandArgs[0], installCommandArgs[1:]...)
	if err != nil {
		return fmt.Errorf("dependency install failed: %w", err)
	}
	return nil
}

// runBuildIfNeeded runs the build script from package.json if one exists
func runBuildIfNeeded(ctx context.Context, deploymentID int64, buildRoot string) error {
	packageJSONPath := filepath.Join(buildRoot, "package.json")
	if _, err := os.Stat(packageJSONPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			_ = appStore.CreateDeploymentLog(ctx, deploymentID, "info", "package.json not found; skipping build command")
			return nil
		}
		return fmt.Errorf("failed to inspect package.json: %w", err)
	}

	// check if there's actually a build script before trying to run it
	hasBuildScript, err := packageHasScript(buildRoot, "build")
	if err != nil {
		return err
	}
	if !hasBuildScript {
		_ = appStore.CreateDeploymentLog(ctx, deploymentID, "warn", "package.json has no build script; skipping build command")
		return nil
	}

	detectedManagerName, buildCommandArgs, err := detectBuildCommand(buildRoot)
	if err != nil {
		return err
	}
	if len(buildCommandArgs) == 0 {
		return nil
	}

	_ = appStore.CreateDeploymentLog(ctx, deploymentID, "info", "run build ("+detectedManagerName+")")
	_, err = runCommand(ctx, buildTimeout, buildRoot, map[string]string{"CI": "true"}, buildCommandArgs[0], buildCommandArgs[1:]...)
	if err != nil {
		return fmt.Errorf("build failed: %w", err)
	}
	return nil
}

// detectInstallCommand picks the right install command based on lock file presence
// pnpm > yarn > npm, falling back to npm install if no lock file found
func detectInstallCommand(projectDir string) (string, []string, error) {
	// pnpm lock file takes top priority
	if fileExists(filepath.Join(projectDir, "pnpm-lock.yaml")) {
		if _, err := exec.LookPath("pnpm"); err != nil {
			return "", nil, fmt.Errorf("pnpm-lock.yaml detected but pnpm is not installed in this runtime")
		}
		return "pnpm", []string{"pnpm", "install", "--frozen-lockfile"}, nil
	}
	// yarn lock file next
	if fileExists(filepath.Join(projectDir, "yarn.lock")) {
		if _, err := exec.LookPath("yarn"); err != nil {
			return "", nil, fmt.Errorf("yarn.lock detected but yarn is not installed in this runtime")
		}
		return "yarn", []string{"yarn", "install", "--frozen-lockfile", "--non-interactive"}, nil
	}
	// npm is the default - use ci if there's a lock file, install otherwise
	if _, err := exec.LookPath("npm"); err != nil {
		return "", nil, fmt.Errorf("npm is not installed in this runtime")
	}
	if fileExists(filepath.Join(projectDir, "package-lock.json")) {
		// npm ci is faster and more reliable than npm install when a lock file exists
		return "npm", []string{"npm", "ci", "--no-audit", "--no-fund"}, nil
	}
	return "npm", []string{"npm", "install", "--no-audit", "--no-fund"}, nil
}

// detectBuildCommand picks the right build command based on lock file presence
func detectBuildCommand(projectDir string) (string, []string, error) {
	if fileExists(filepath.Join(projectDir, "pnpm-lock.yaml")) {
		if _, err := exec.LookPath("pnpm"); err != nil {
			return "", nil, fmt.Errorf("pnpm-lock.yaml detected but pnpm is not installed in this runtime")
		}
		return "pnpm", []string{"pnpm", "run", "build"}, nil
	}
	if fileExists(filepath.Join(projectDir, "yarn.lock")) {
		if _, err := exec.LookPath("yarn"); err != nil {
			return "", nil, fmt.Errorf("yarn.lock detected but yarn is not installed in this runtime")
		}
		return "yarn", []string{"yarn", "build"}, nil
	}
	if _, err := exec.LookPath("npm"); err != nil {
		return "", nil, fmt.Errorf("npm is not installed in this runtime")
	}
	return "npm", []string{"npm", "run", "build"}, nil
}

// inspectBuildPlan reads the package.json to determine if a build step is needed
// returns hasPackageJSON, hasBuildScript, and any error
func inspectBuildPlan(buildRoot string) (hasPackageJSON bool, hasBuildScript bool, err error) {
	packageJSONPath := filepath.Join(buildRoot, "package.json")
	if _, statErr := os.Stat(packageJSONPath); statErr != nil {
		if errors.Is(statErr, os.ErrNotExist) {
			// no package.json means pure static - no build needed
			return false, false, nil
		}
		return false, false, fmt.Errorf("failed to inspect package.json: %w", statErr)
	}
	// package.json exists - check if it has a build script
	hasBuildScript, err = packageHasScript(buildRoot, "build")
	if err != nil {
		return true, false, err
	}
	return true, hasBuildScript, nil
}

// packageHasScript reads package.json and checks if the named script exists
func packageHasScript(projectDir string, scriptName string) (bool, error) {
	packageJSONFullPath := filepath.Join(projectDir, "package.json")
	rawFileContents, err := os.ReadFile(packageJSONFullPath)
	if err != nil {
		return false, fmt.Errorf("read package.json: %w", err)
	}

	// only unmarshal the scripts field - we don't need the whole file
	var packageJSONStructure struct {
		Scripts map[string]any `json:"scripts"`
	}
	if err := json.Unmarshal(rawFileContents, &packageJSONStructure); err != nil {
		return false, fmt.Errorf("parse package.json: %w", err)
	}
	scriptValue, scriptExists := packageJSONStructure.Scripts[strings.TrimSpace(scriptName)]
	if !scriptExists {
		return false, nil
	}
	// handle both string values and non-string values (like objects in some tooling)
	switch typedScriptValue := scriptValue.(type) {
	case string:
		return strings.TrimSpace(typedScriptValue) != "", nil
	default:
		return true, nil
	}
}