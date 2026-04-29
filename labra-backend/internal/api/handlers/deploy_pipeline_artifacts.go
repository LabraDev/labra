package handlers

import (
	"context"
	"fmt"
	"io/fs"
	"mime"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// resolveArtifactDirectory finds where the build artifacts actually ended up
// tries the configured output dir first, then falls back to common locations like dist/build/out
func resolveArtifactDirectory(buildRoot string, requestedOutputDir string) (string, bool, error) {
	// try the configured path first
	requestedFullPath, err := resolveSafeSubdirectory(buildRoot, requestedOutputDir, "output_dir")
	if err != nil {
		return "", false, err
	}
	if directoryExists(requestedFullPath) {
		return requestedFullPath, false, nil
	}

	// the configured dir doesn't exist - try common fallback locations
	// prefer ones that have an index.html since that's a static site indicator
	fallbackDirCandidates := uniqueStrings([]string{
		"dist",
		"build",
		"out",
		"public",
	})
	for _, candidateName := range fallbackDirCandidates {
		candidateFullPath, pathErr := resolveSafeSubdirectory(buildRoot, candidateName, "output_dir")
		if pathErr != nil {
			continue
		}
		if !directoryExists(candidateFullPath) {
			continue
		}
		// prefer dirs that have an index.html - more confident they're the right output
		if hasIndexHTML(candidateFullPath) {
			return candidateFullPath, true, nil
		}
	}
	// second pass - accept any existing dir from the fallback list even without index.html
	for _, candidateName := range fallbackDirCandidates {
		candidateFullPath, pathErr := resolveSafeSubdirectory(buildRoot, candidateName, "output_dir")
		if pathErr != nil {
			continue
		}
		if directoryExists(candidateFullPath) {
			return candidateFullPath, true, nil
		}
	}

	// last resort - try the build root itself if it has an index.html
	if hasIndexHTML(buildRoot) {
		return buildRoot, true, nil
	}

	return "", false, fmt.Errorf(
		"build output directory %q does not exist (and no fallback static directory was found)",
		strings.TrimSpace(requestedOutputDir),
	)
}

// syncStaticArtifactsToS3 uploads all files from artifactDir to the s3 bucket
// also deletes any objects in the bucket that are no longer in the artifact dir
func syncStaticArtifactsToS3(
	ctx context.Context,
	cfg aws.Config,
	region string,
	bucketName string,
	artifactDir string,
) (int, int, error) {
	// use the provided region, fall back to config region, then default
	effectiveRegion := strings.TrimSpace(region)
	if effectiveRegion == "" {
		effectiveRegion = cfg.Region
	}
	if effectiveRegion == "" {
		effectiveRegion = "us-west-1"
	}

	s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.Region = effectiveRegion
	})

	// build a list of all files to upload
	allArtifactFiles, err := listArtifactFiles(artifactDir)
	if err != nil {
		return 0, 0, err
	}
	if len(allArtifactFiles) == 0 {
		return 0, 0, fmt.Errorf("build output directory %q is empty", artifactDir)
	}

	// track which keys we're uploading so we can delete stale ones afterward
	expectedS3Keys := make(map[string]struct{}, len(allArtifactFiles))
	for _, artifactItem := range allArtifactFiles {
		expectedS3Keys[artifactItem.Key] = struct{}{}
		detectedContentType := detectContentType(artifactItem.Path)
		appropriateCacheControl := cacheControlForKey(artifactItem.Key)
		if err := uploadArtifactFile(ctx, s3Client, bucketName, artifactItem.Key, artifactItem.Path, detectedContentType, appropriateCacheControl); err != nil {
			return 0, 0, err
		}
	}

	// remove any objects that weren't part of this build
	deletedObjectCount, err := deleteStaleObjects(ctx, s3Client, bucketName, expectedS3Keys)
	if err != nil {
		return 0, 0, err
	}
	return len(allArtifactFiles), deletedObjectCount, nil
}

// artifactFile pairs an s3 key with the local file path it came from
type artifactFile struct {
	Key  string
	Path string
}

// listArtifactFiles walks the artifact directory and returns all files with their s3 keys
func listArtifactFiles(rootDir string) ([]artifactFile, error) {
	collectedFiles := make([]artifactFile, 0)
	err := filepath.WalkDir(rootDir, func(filePath string, dirEntry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		// skip directories - we only want files
		if dirEntry.IsDir() {
			return nil
		}
		// convert to a relative path for use as the s3 key
		relativeFilePath, err := filepath.Rel(rootDir, filePath)
		if err != nil {
			return err
		}
		// use forward slashes for s3 keys regardless of os
		s3ObjectKey := filepath.ToSlash(strings.TrimSpace(relativeFilePath))
		if s3ObjectKey == "" || s3ObjectKey == "." {
			return nil
		}
		collectedFiles = append(collectedFiles, artifactFile{
			Key:  s3ObjectKey,
			Path: filePath,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan build output files: %w", err)
	}

	// sort for deterministic upload order - easier to debug logs
	sort.Slice(collectedFiles, func(i, j int) bool {
		return collectedFiles[i].Key < collectedFiles[j].Key
	})
	return collectedFiles, nil
}

// uploadArtifactFile puts a single file into s3 with the given content type and cache control
func uploadArtifactFile(
	ctx context.Context,
	s3Client *s3.Client,
	bucketName string,
	s3Key string,
	localFilePath string,
	contentType string,
	cacheControl string,
) error {
	openedFile, err := os.Open(localFilePath)
	if err != nil {
		return fmt.Errorf("open artifact file %q: %w", s3Key, err)
	}
	defer openedFile.Close()

	putObjectInput := &s3.PutObjectInput{
		Bucket:       &bucketName,
		Key:          &s3Key,
		Body:         openedFile,
		ContentType:  &contentType,
		CacheControl: &cacheControl,
	}
	if _, err := s3Client.PutObject(ctx, putObjectInput); err != nil {
		return fmt.Errorf("upload artifact %q: %w", s3Key, err)
	}
	return nil
}

// deleteStaleObjects removes s3 objects that are no longer in the expected set
// handles pagination and batches deletes in chunks of 1000 (s3 api limit)
func deleteStaleObjects(
	ctx context.Context,
	s3Client *s3.Client,
	bucketName string,
	expectedKeys map[string]struct{},
) (int, error) {
	staleObjectKeys := make([]string, 0)

	// paginate through all existing objects in the bucket
	listPaginator := s3.NewListObjectsV2Paginator(s3Client, &s3.ListObjectsV2Input{
		Bucket: &bucketName,
	})
	for listPaginator.HasMorePages() {
		listPage, err := listPaginator.NextPage(ctx)
		if err != nil {
			return 0, fmt.Errorf("list existing objects: %w", err)
		}
		for _, existingObject := range listPage.Contents {
			existingKey := strings.TrimSpace(aws.ToString(existingObject.Key))
			if existingKey == "" {
				continue
			}
			// if this key isn't in our expected set, it's stale
			if _, isExpected := expectedKeys[existingKey]; !isExpected {
				staleObjectKeys = append(staleObjectKeys, existingKey)
			}
		}
	}

	if len(staleObjectKeys) == 0 {
		return 0, nil
	}

	// delete in batches of 1000 - that's the s3 api max per delete request
	const deleteBatchSize = 1000
	totalDeletedCount := 0
	for batchStart := 0; batchStart < len(staleObjectKeys); batchStart += deleteBatchSize {
		batchEnd := batchStart + deleteBatchSize
		if batchEnd > len(staleObjectKeys) {
			batchEnd = len(staleObjectKeys)
		}

		currentBatch := staleObjectKeys[batchStart:batchEnd]
		deleteObjectIdentifiers := make([]s3types.ObjectIdentifier, 0, len(currentBatch))
		for _, staleKey := range currentBatch {
			keyCopy := staleKey
			deleteObjectIdentifiers = append(deleteObjectIdentifiers, s3types.ObjectIdentifier{Key: &keyCopy})
		}
		_, err := s3Client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: &bucketName,
			Delete: &s3types.Delete{
				Objects: deleteObjectIdentifiers,
				Quiet:   boolPtr(true),
			},
		})
		if err != nil {
			return totalDeletedCount, fmt.Errorf("delete stale objects: %w", err)
		}
		totalDeletedCount += len(currentBatch)
	}
	return totalDeletedCount, nil
}

// detectContentType guesses the mime type from the file extension
// falls back to octet-stream if we can't figure it out
func detectContentType(filePath string) string {
	fileExtension := strings.ToLower(filepath.Ext(filePath))
	if fileExtension != "" {
		if mimeType := mime.TypeByExtension(fileExtension); strings.TrimSpace(mimeType) != "" {
			return mimeType
		}
	}
	return "application/octet-stream"
}

// cacheControlForKey returns cache control headers based on file type
// html/json/xml get no-cache since they change frequently
// everything else gets a short cache since they're usually hashed
func cacheControlForKey(s3Key string) string {
	fileExtension := strings.ToLower(filepath.Ext(s3Key))
	switch fileExtension {
	case ".html", ".json", ".txt", ".xml":
		// these should never be cached - content changes on deploy
		return "no-cache, no-store, must-revalidate"
	default:
		// short cache for assets - 5 minutes is enough for cloudfront to catch up
		return "public, max-age=300"
	}
}