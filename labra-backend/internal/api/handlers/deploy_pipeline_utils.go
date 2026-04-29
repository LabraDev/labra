package handlers

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// runCommand runs a shell command with a timeout and captured output
// merges stdout and stderr so we get all output in one place for deployment logs
func runCommand(
	ctx context.Context,
	commandTimeout time.Duration,
	workingDir string,
	extraEnvVars map[string]string,
	commandName string,
	commandArgs ...string,
) (string, error) {
	// wrap the parent context with a per-command timeout
	commandCtx, cancelCommand := context.WithTimeout(ctx, commandTimeout)
	defer cancelCommand()

	execCmd := exec.CommandContext(commandCtx, commandName, commandArgs...)
	execCmd.Dir = workingDir
	// merge extra env vars on top of the current process environment
	execCmd.Env = mergeEnvs(os.Environ(), extraEnvVars)

	// capture both stdout and stderr into a single bounded buffer
	var capturedOutput boundedOutput
	execCmd.Stdout = &capturedOutput
	execCmd.Stderr = &capturedOutput

	commandRunErr := execCmd.Run()
	if commandCtx.Err() == context.DeadlineExceeded {
		return capturedOutput.String(), fmt.Errorf("command timed out after %s", commandTimeout)
	}
	if commandRunErr != nil {
		outputSnippet := strings.TrimSpace(capturedOutput.String())
		if outputSnippet != "" {
			return capturedOutput.String(), fmt.Errorf("command failed: %w (output: %s)", commandRunErr, outputSnippet)
		}
		return capturedOutput.String(), fmt.Errorf("command failed: %w", commandRunErr)
	}
	return capturedOutput.String(), nil
}

// boundedOutput is a writer that caps how much we keep in memory
// build output can be huge and we don't need all of it for logs
type boundedOutput struct {
	buf bytes.Buffer
}

func (b *boundedOutput) Write(rawBytes []byte) (int, error) {
	// cap at 16kb - enough to see what went wrong without eating too much ram
	const maxOutputBytes = 16 * 1024
	remainingCapacity := maxOutputBytes - b.buf.Len()
	if remainingCapacity > 0 {
		if len(rawBytes) > remainingCapacity {
			b.buf.Write(rawBytes[:remainingCapacity])
		} else {
			b.buf.Write(rawBytes)
		}
	}
	return len(rawBytes), nil
}

func (b *boundedOutput) String() string {
	return strings.TrimSpace(b.buf.String())
}

// mergeEnvs combines a base env slice with a map of extra key=value pairs
func mergeEnvs(baseEnv []string, extraEnvMap map[string]string) []string {
	if len(extraEnvMap) == 0 {
		return baseEnv
	}
	mergedEnv := make([]string, 0, len(baseEnv)+len(extraEnvMap))
	mergedEnv = append(mergedEnv, baseEnv...)
	for envKey, envValue := range extraEnvMap {
		mergedEnv = append(mergedEnv, envKey+"="+envValue)
	}
	return mergedEnv
}

// resolveSafeSubdirectory resolves a relative path against a base dir
// rejects absolute paths and anything that would escape the base dir with ../
func resolveSafeSubdirectory(baseDir, rawRelativePath, fieldName string) (string, error) {
	trimmedRelPath := strings.TrimSpace(rawRelativePath)
	// empty or dot means just use the base dir as-is
	if trimmedRelPath == "" || trimmedRelPath == "." {
		return baseDir, nil
	}
	if filepath.IsAbs(trimmedRelPath) {
		return "", fmt.Errorf("%s must be a relative path", fieldName)
	}

	resolvedPath := filepath.Clean(filepath.Join(baseDir, trimmedRelPath))
	relativeFromBase, err := filepath.Rel(baseDir, resolvedPath)
	if err != nil {
		return "", fmt.Errorf("invalid %s path: %w", fieldName, err)
	}
	// check for path traversal attempts
	normalizedRelative := filepath.ToSlash(relativeFromBase)
	if normalizedRelative == ".." || strings.HasPrefix(normalizedRelative, "../") {
		return "", fmt.Errorf("%s cannot escape repository root", fieldName)
	}
	return resolvedPath, nil
}

// fileExists returns true if a regular file exists at the given path
func fileExists(filePath string) bool {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return false
	}
	return !fileInfo.IsDir()
}

// directoryExists returns true if a directory exists at the given path
func directoryExists(dirPath string) bool {
	dirInfo, err := os.Stat(dirPath)
	if err != nil {
		return false
	}
	return dirInfo.IsDir()
}

// hasIndexHTML checks if a directory contains an index.html file
func hasIndexHTML(dirPath string) bool {
	return fileExists(filepath.Join(dirPath, "index.html"))
}

// uniqueStrings deduplicates a slice of strings while preserving order
func uniqueStrings(inputStrings []string) []string {
	seenStrings := make(map[string]struct{}, len(inputStrings))
	uniqueResult := make([]string, 0, len(inputStrings))
	for _, stringItem := range inputStrings {
		trimmedItem := strings.TrimSpace(stringItem)
		if trimmedItem == "" {
			continue
		}
		if _, alreadySeen := seenStrings[trimmedItem]; alreadySeen {
			continue
		}
		seenStrings[trimmedItem] = struct{}{}
		uniqueResult = append(uniqueResult, trimmedItem)
	}
	return uniqueResult
}

// relativeOrDot returns a path relative to baseDir, or "." if it equals the base
func relativeOrDot(baseDir string, targetPath string) string {
	relPath, err := filepath.Rel(baseDir, targetPath)
	if err != nil {
		return targetPath
	}
	normalizedRelPath := filepath.ToSlash(strings.TrimSpace(relPath))
	if normalizedRelPath == "" || normalizedRelPath == "." {
		return "."
	}
	return normalizedRelPath
}
