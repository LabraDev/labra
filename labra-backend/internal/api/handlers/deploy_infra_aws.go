package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"labra-backend/internal/api/store"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// errNoValidatedAWSConnection is returned when the user hasn't connected an aws account yet
var errNoValidatedAWSConnection = errors.New("no validated aws connection configured")

// ensureDeploymentInfra makes sure the s3 bucket and cloudfront distribution exist for this app
// creates them if they don't exist - idempotent so safe to call on every deploy
func ensureDeploymentInfra(ctx context.Context, app store.App) (store.AppInfraOutput, error) {
	// figure out what bucket name we want - reuse existing one if we already created it before
	desiredBucketName := buildBucketName(app)
	existing, existingErr := appStore.GetAppInfraOutputByAppForUser(ctx, app.ID, app.UserID)
	if isMissingTableErr(existingErr) {
		existingErr = store.ErrNotFound
	}
	if existingErr == nil && strings.TrimSpace(existing.BucketName) != "" {
		desiredBucketName = strings.TrimSpace(existing.BucketName)
	}

	connection, err := selectValidatedAWSConnection(ctx, app.UserID)
	if err != nil {
		return store.AppInfraOutput{}, err
	}

	cfg, err := loadAssumedRoleConfig(ctx, connection)
	if err != nil {
		return store.AppInfraOutput{}, err
	}

	if err := ensureS3WebsiteBucket(ctx, cfg, desiredBucketName, connection.Region); err != nil {
		return store.AppInfraOutput{}, err
	}

	distributionID := ""
	if existingErr == nil {
		distributionID = strings.TrimSpace(existing.DistributionID)
	}
	distributionID, siteURL, err := ensureCloudFrontDistribution(ctx, cfg, desiredBucketName, connection.Region, app.ID, distributionID)
	if err != nil {
		return store.AppInfraOutput{}, err
	}

	out, err := appStore.UpsertAppInfraOutput(ctx, store.UpsertAppInfraOutputInput{
		AppID:          app.ID,
		UserID:         app.UserID,
		BucketName:     desiredBucketName,
		DistributionID: distributionID,
		SiteURL:        siteURL,
	})
	if err != nil {
		return store.AppInfraOutput{}, err
	}

	if strings.TrimSpace(app.SiteURL) != strings.TrimSpace(siteURL) {
		_, err = appStore.UpdateAppForUser(ctx, app.ID, app.UserID, store.UpdateAppInput{
			Name:              app.Name,
			Branch:            app.Branch,
			BuildType:         app.BuildType,
			OutputDir:         app.OutputDir,
			RootDir:           app.RootDir,
			SiteURL:           siteURL,
			AutoDeployEnabled: app.AutoDeployEnabled,
		})
		if err != nil {
			return store.AppInfraOutput{}, err
		}
	}

	return out, nil
}

// selectValidatedAWSConnection finds the first validated aws connection for a user
// returns errNoValidatedAWSConnection if they haven't connected an aws account yet
func selectValidatedAWSConnection(ctx context.Context, userID int64) (store.AWSConnection, error) {
	userConnections, err := appStore.ListAWSConnectionsByUser(ctx, userID)
	if err != nil {
		// missing table means the migrations haven't run yet - treat as no connection
		if strings.Contains(strings.ToLower(err.Error()), "no such table") {
			return store.AWSConnection{}, errNoValidatedAWSConnection
		}
		return store.AWSConnection{}, err
	}
	// return the first validated connection - most users only have one
	for _, candidateConnection := range userConnections {
		if strings.EqualFold(strings.TrimSpace(candidateConnection.Status), "validated") {
			return candidateConnection, nil
		}
	}
	return store.AWSConnection{}, errNoValidatedAWSConnection
}

func isMissingTableErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "no such table")
}

// loadAssumedRoleConfig builds an aws config that uses assume role for the given connection
// the session lasts 30 minutes which is plenty for a single deployment
func loadAssumedRoleConfig(ctx context.Context, connection store.AWSConnection) (aws.Config, error) {
	regionValue := strings.TrimSpace(connection.Region)
	if regionValue == "" {
		regionValue = "us-west-1"
	}

	// load the base config using whatever credentials are available in the environment (ec2 role, env vars, etc)
	baseAWSConfig, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(regionValue))
	if err != nil {
		return aws.Config{}, fmt.Errorf("load aws config: %w", err)
	}

	// set up the assume role provider that will swap to the customer's role
	stsServiceClient := sts.NewFromConfig(baseAWSConfig)
	assumeRoleProvider := stscreds.NewAssumeRoleProvider(stsServiceClient, strings.TrimSpace(connection.RoleARN), func(o *stscreds.AssumeRoleOptions) {
		o.RoleSessionName = fmt.Sprintf("labra-deploy-%d", time.Now().Unix())
		externalIDValue := strings.TrimSpace(connection.ExternalID)
		if externalIDValue != "" {
			o.ExternalID = &externalIDValue
		}
		// 30 min should be more than enough for any deploy
		o.Duration = 30 * time.Minute
	})
	// wrap in credentials cache so we don't assume the role on every api call
	baseAWSConfig.Credentials = aws.NewCredentialsCache(assumeRoleProvider)
	return baseAWSConfig, nil
}

func ensureS3WebsiteBucket(ctx context.Context, cfg aws.Config, bucketName, region string) error {
	s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.Region = region
	})

	if _, err := s3Client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: &bucketName}); err != nil {
		input := &s3.CreateBucketInput{Bucket: &bucketName}
		if strings.TrimSpace(region) != "" && region != "us-east-1" {
			input.CreateBucketConfiguration = &s3types.CreateBucketConfiguration{
				LocationConstraint: s3types.BucketLocationConstraint(region),
			}
		}
		if _, createErr := s3Client.CreateBucket(ctx, input); createErr != nil {
			return fmt.Errorf("create s3 bucket %q: %w", bucketName, createErr)
		}
	}

	_, _ = s3Client.PutPublicAccessBlock(ctx, &s3.PutPublicAccessBlockInput{
		Bucket: &bucketName,
		PublicAccessBlockConfiguration: &s3types.PublicAccessBlockConfiguration{
			BlockPublicAcls:       boolPtr(false),
			IgnorePublicAcls:      boolPtr(false),
			BlockPublicPolicy:     boolPtr(false),
			RestrictPublicBuckets: boolPtr(false),
		},
	})

	_, _ = s3Client.PutBucketOwnershipControls(ctx, &s3.PutBucketOwnershipControlsInput{
		Bucket: &bucketName,
		OwnershipControls: &s3types.OwnershipControls{
			Rules: []s3types.OwnershipControlsRule{
				{ObjectOwnership: s3types.ObjectOwnershipBucketOwnerPreferred},
			},
		},
	})

	_, _ = s3Client.PutBucketWebsite(ctx, &s3.PutBucketWebsiteInput{
		Bucket: &bucketName,
		WebsiteConfiguration: &s3types.WebsiteConfiguration{
			IndexDocument: &s3types.IndexDocument{Suffix: strPtr("index.html")},
			ErrorDocument: &s3types.ErrorDocument{Key: strPtr("index.html")},
		},
	})

	policy, _ := json.Marshal(map[string]any{
		"Version": "2012-10-17",
		"Statement": []map[string]any{
			{
				"Sid":       "PublicReadForWebsite",
				"Effect":    "Allow",
				"Principal": "*",
				"Action":    []string{"s3:GetObject"},
				"Resource":  []string{fmt.Sprintf("arn:aws:s3:::%s/*", bucketName)},
			},
		},
	})
	_, _ = s3Client.PutBucketPolicy(ctx, &s3.PutBucketPolicyInput{
		Bucket: &bucketName,
		Policy: strPtr(string(policy)),
	})

	_, _ = s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      &bucketName,
		Key:         strPtr("index.html"),
		ContentType: strPtr("text/html; charset=utf-8"),
		Body: strings.NewReader(
			"<!doctype html><html><head><meta charset=\"utf-8\"><title>Labra App</title></head><body><h1>Labra deployment initialized</h1></body></html>",
		),
	})

	return nil
}

func ensureCloudFrontDistribution(ctx context.Context, cfg aws.Config, bucketName, region string, appID int64, existingDistributionID string) (distributionID string, siteURL string, err error) {
	cfClient := cloudfront.NewFromConfig(cfg)

	if strings.TrimSpace(existingDistributionID) != "" {
		getResp, getErr := cfClient.GetDistribution(ctx, &cloudfront.GetDistributionInput{
			Id: &existingDistributionID,
		})
		if getErr == nil && getResp.Distribution != nil {
			domain := strings.TrimSpace(strVal(getResp.Distribution.DomainName))
			if domain != "" {
				return strings.TrimSpace(existingDistributionID), "https://" + domain, nil
			}
		}
	}

	websiteDomain := fmt.Sprintf("%s.s3-website-%s.amazonaws.com", bucketName, region)
	originID := fmt.Sprintf("labra-app-%d-origin", appID)
	callerReference := fmt.Sprintf("labra-app-%d-%d", appID, time.Now().UnixNano())
	comment := fmt.Sprintf("labra app %d static site", appID)

	resp, err := cfClient.CreateDistribution(ctx, &cloudfront.CreateDistributionInput{
		DistributionConfig: &cftypes.DistributionConfig{
			CallerReference:   &callerReference,
			Comment:           &comment,
			Enabled:           boolPtr(true),
			DefaultRootObject: strPtr("index.html"),
			PriceClass:        cftypes.PriceClassPriceClass100,
			Origins: &cftypes.Origins{
				Quantity: int32Ptr(1),
				Items: []cftypes.Origin{
					{
						Id:         &originID,
						DomainName: &websiteDomain,
						CustomOriginConfig: &cftypes.CustomOriginConfig{
							HTTPPort:             int32Ptr(80),
							HTTPSPort:            int32Ptr(443),
							OriginProtocolPolicy: cftypes.OriginProtocolPolicyHttpOnly,
						},
					},
				},
			},
			DefaultCacheBehavior: &cftypes.DefaultCacheBehavior{
				TargetOriginId:       &originID,
				ViewerProtocolPolicy: cftypes.ViewerProtocolPolicyRedirectToHttps,
				AllowedMethods: &cftypes.AllowedMethods{
					Quantity: int32Ptr(2),
					Items:    []cftypes.Method{cftypes.MethodGet, cftypes.MethodHead},
					CachedMethods: &cftypes.CachedMethods{
						Quantity: int32Ptr(2),
						Items:    []cftypes.Method{cftypes.MethodGet, cftypes.MethodHead},
					},
				},
				ForwardedValues: &cftypes.ForwardedValues{
					QueryString: boolPtr(false),
					Cookies: &cftypes.CookiePreference{
						Forward: cftypes.ItemSelectionNone,
					},
				},
				Compress: boolPtr(true),
				MinTTL:   int64Ptr(0),
			},
			Restrictions: &cftypes.Restrictions{
				GeoRestriction: &cftypes.GeoRestriction{
					RestrictionType: cftypes.GeoRestrictionTypeNone,
					Quantity:        int32Ptr(0),
				},
			},
			ViewerCertificate: &cftypes.ViewerCertificate{
				CloudFrontDefaultCertificate: boolPtr(true),
			},
		},
	})
	if err != nil {
		return "", "", fmt.Errorf("create cloudfront distribution: %w", err)
	}

	if resp.Distribution == nil {
		return "", "", fmt.Errorf("create cloudfront distribution: empty response")
	}

	domain := strings.TrimSpace(strVal(resp.Distribution.DomainName))
	id := strings.TrimSpace(strVal(resp.Distribution.Id))
	if domain == "" || id == "" {
		return "", "", fmt.Errorf("create cloudfront distribution: missing id or domain")
	}

	return id, "https://" + domain, nil
}

func strPtr(v string) *string { return &v }

func strVal(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func boolPtr(v bool) *bool { return &v }

func int32Ptr(v int32) *int32 { return &v }

func int64Ptr(v int64) *int64 { return &v }
