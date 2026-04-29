package handlers

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"labra-backend/internal/api/store"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// these are the timeouts for each phase of the pipeline
// checkout and build are generous because repos can be big and builds can be slow
const (
	defaultBuildOutputDir       = "dist"
	checkoutTimeout             = 3 * time.Minute
	installTimeout              = 8 * time.Minute
	buildTimeout                = 8 * time.Minute
	siteProbeTimeout            = 8 * time.Second
	awaitingSiteMonitorTimeout  = 12 * time.Minute
	awaitingSiteMonitorInterval = 15 * time.Second
)

// deploymentPipelineResult carries what we got out of a pipeline run
type deploymentPipelineResult struct {
	SiteURL string
	Status  string
}

// deploymentRuntime holds the credentials and aws config we need for a single deploy
// built once at the start of each deployment run
type deploymentRuntime struct {
	InstallationToken string
	AWSConnection     store.AWSConnection
	AWSConfig         aws.Config
}

// executeDeploymentPipeline runs the full build and deploy sequence for a static app
// this is the big one - checkout -> install -> build -> upload to s3 -> invalidate cloudfront
func executeDeploymentPipeline(
	ctx context.Context,
	deploymentID int64,
	app store.App,
	branch string,
) (deploymentPipelineResult, error) {
	// build the runtime first - this fetches github install token and aws credentials
	deployRuntime, err := buildDeploymentRuntime(ctx, app.UserID)
	if err != nil {
		return deploymentPipelineResult{}, err
	}

	// make sure the s3 bucket and cloudfront distribution exist before we try to upload anything
	infraOutputData, err := ensureDeploymentInfra(ctx, app)
	if err != nil {
		return deploymentPipelineResult{}, fmt.Errorf("failed to ensure deployment infrastructure: %w", err)
	}

	// validate we got all the infra fields we need
	s3BucketName := strings.TrimSpace(infraOutputData.BucketName)
	cloudfrontDistributionID := strings.TrimSpace(infraOutputData.DistributionID)
	cloudfrontSiteURL := strings.TrimSpace(infraOutputData.SiteURL)
	if s3BucketName == "" {
		return deploymentPipelineResult{}, fmt.Errorf("deployment infrastructure missing bucket output")
	}
	if cloudfrontDistributionID == "" {
		return deploymentPipelineResult{}, fmt.Errorf("deployment infrastructure missing cloudfront distribution output")
	}
	if cloudfrontSiteURL == "" {
		return deploymentPipelineResult{}, fmt.Errorf("deployment infrastructure missing cloudfront site URL")
	}
	_ = appStore.CreateDeploymentLog(
		ctx,
		deploymentID,
		"info",
		fmt.Sprintf("deployment infra ready (bucket=%s distribution=%s)", s3BucketName, cloudfrontDistributionID),
	)

	// make a temp directory to clone the repo into - cleaned up on exit
	workspaceTempDir, err := os.MkdirTemp("", fmt.Sprintf("labra-app-deploy-%d-", deploymentID))
	if err != nil {
		return deploymentPipelineResult{}, fmt.Errorf("create deployment workspace: %w", err)
	}
	defer func() { _ = os.RemoveAll(workspaceTempDir) }()

	// clone the repo at the given branch into the workspace
	clonedRepoDir, resolvedCommitSHA, err := checkoutGitHubRepository(
		ctx,
		deploymentID,
		workspaceTempDir,
		deployRuntime.InstallationToken,
		app.RepoFullName,
		branch,
	)
	if err != nil {
		return deploymentPipelineResult{}, err
	}
	if resolvedCommitSHA != "" {
		_ = appStore.CreateDeploymentLog(ctx, deploymentID, "info", "checked out commit "+resolvedCommitSHA)
	}

	// resolve the build root - this is usually the repo root but can be a subdirectory
	buildRootDir, err := resolveSafeSubdirectory(clonedRepoDir, app.RootDir, "root_dir")
	if err != nil {
		return deploymentPipelineResult{}, err
	}

	// inspect the repo to see what kind of build we're dealing with
	hasPackageJSONFile, hasBuildScriptDefined, err := inspectBuildPlan(buildRootDir)
	if err != nil {
		return deploymentPipelineResult{}, err
	}
	if hasBuildScriptDefined {
		// standard node project - install then build
		if err := installDependenciesIfNeeded(ctx, deploymentID, buildRootDir); err != nil {
			return deploymentPipelineResult{}, err
		}
		if err := runBuildIfNeeded(ctx, deploymentID, buildRootDir); err != nil {
			return deploymentPipelineResult{}, err
		}
	} else if hasPackageJSONFile {
		// has a package.json but no build script - might still have a dist folder
		_ = appStore.CreateDeploymentLog(
			ctx,
			deploymentID,
			"warn",
			"package.json has no build script; skipping build and attempting static artifact detection",
		)
	} else {
		// no package.json at all - pure static site, just look for files
		_ = appStore.CreateDeploymentLog(
			ctx,
			deploymentID,
			"info",
			"package.json not found; attempting static artifact detection without build step",
		)
	}

	// figure out where the build artifacts are
	configuredOutputDirValue := strings.TrimSpace(app.OutputDir)
	if configuredOutputDirValue == "" {
		configuredOutputDirValue = defaultBuildOutputDir
	}
	resolvedArtifactDir, usedFallbackDir, err := resolveArtifactDirectory(buildRootDir, configuredOutputDirValue)
	if err != nil {
		return deploymentPipelineResult{}, err
	}
	if usedFallbackDir {
		_ = appStore.CreateDeploymentLog(
			ctx,
			deploymentID,
			"warn",
			fmt.Sprintf("configured output directory %q not found; using %q", configuredOutputDirValue, relativeOrDot(buildRootDir, resolvedArtifactDir)),
		)
	}

	// sync all the build artifacts to s3
	uploadedFileCount, deletedStaleFileCount, err := syncStaticArtifactsToS3(
		ctx,
		deployRuntime.AWSConfig,
		deployRuntime.AWSConnection.Region,
		s3BucketName,
		resolvedArtifactDir,
	)
	if err != nil {
		return deploymentPipelineResult{}, err
	}
	_ = appStore.CreateDeploymentLog(
		ctx,
		deploymentID,
		"info",
		fmt.Sprintf("uploaded %d files to s3 (deleted %d stale files)", uploadedFileCount, deletedStaleFileCount),
	)

	// invalidate cloudfront so the new files get served right away
	cfInvalidationID, err := invalidateCloudFrontDistribution(ctx, deployRuntime.AWSConfig, cloudfrontDistributionID)
	if err != nil {
		return deploymentPipelineResult{}, err
	}
	_ = appStore.CreateDeploymentLog(ctx, deploymentID, "info", "cloudfront invalidation created: "+cfInvalidationID)

	// check if the site is actually reachable yet
	siteIsUp := isSiteReachable(cloudfrontSiteURL)
	if siteIsUp {
		return deploymentPipelineResult{
			SiteURL: cloudfrontSiteURL,
			Status:  "succeeded",
		}, nil
	}

	// cloudfront takes time to propagate - return awaiting_site_url and poll in background
	_ = appStore.CreateDeploymentLog(
		ctx,
		deploymentID,
		"warn",
		"artifacts deployed, but cloudfront URL is not reachable yet; keeping status as awaiting_site_url",
	)
	return deploymentPipelineResult{
		SiteURL: cloudfrontSiteURL,
		Status:  "awaiting_site_url",
	}, nil
}