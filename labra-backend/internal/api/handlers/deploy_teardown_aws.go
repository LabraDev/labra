package handlers

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"labra-backend/internal/api/store"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// cloudfront distribution ids are all uppercase alphanumeric, at least 8 chars
var cloudFrontDistributionIDPattern = regexp.MustCompile(`^[A-Z0-9]{8,}$`)

// teardownDeploymentInfra tears down the s3 bucket and cloudfront distribution for an app
// called before deleting the app record so we don't leave orphaned aws resources
func teardownDeploymentInfra(ctx context.Context, app store.App) error {
	if appStore == nil {
		return fmt.Errorf("store not initialized")
	}

	infra, err := appStore.GetAppInfraOutputByAppForUser(ctx, app.ID, app.UserID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || isMissingTableErr(err) {
			return nil
		}
		return fmt.Errorf("load app infra outputs: %w", err)
	}

	bucketName := strings.TrimSpace(infra.BucketName)
	distributionID := strings.TrimSpace(infra.DistributionID)
	if bucketName == "" && distributionID == "" {
		return nil
	}

	connection, err := selectValidatedAWSConnection(ctx, app.UserID)
	if err != nil {
		return fmt.Errorf("load validated aws connection: %w", err)
	}
	cfg, err := loadAssumedRoleConfig(ctx, connection)
	if err != nil {
		return fmt.Errorf("load assumed-role aws config: %w", err)
	}

	if distributionID != "" && isLikelyCloudFrontDistributionID(distributionID) {
		if err := disableAndDeleteCloudFrontDistribution(ctx, cfg, distributionID); err != nil {
			return fmt.Errorf("delete cloudfront distribution %q: %w", distributionID, err)
		}
	}

	if bucketName != "" {
		if err := emptyAndDeleteBucket(ctx, cfg, connection.Region, bucketName); err != nil {
			return fmt.Errorf("delete s3 bucket %q: %w", bucketName, err)
		}
	}

	return nil
}

func isLikelyCloudFrontDistributionID(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	if strings.Contains(trimmed, "-") {
		return false
	}
	return cloudFrontDistributionIDPattern.MatchString(trimmed)
}

// disableAndDeleteCloudFrontDistribution disables and then deletes a cloudfront distribution
// cloudfront requires you to disable first and wait for it to propagate before you can delete
// this can take several minutes so we loop with retries
func disableAndDeleteCloudFrontDistribution(ctx context.Context, cfg aws.Config, distributionID string) error {
	client := cloudfront.NewFromConfig(cfg)

	// loop up to 3 times because cloudfront propagation can take a while
	for attempt := 0; attempt < 3; attempt++ {
		getResp, err := client.GetDistributionConfig(ctx, &cloudfront.GetDistributionConfigInput{
			Id: &distributionID,
		})
		if err != nil {
			if awsErrorCode(err) == "NoSuchDistribution" {
				return nil
			}
			return err
		}
		if getResp == nil || getResp.DistributionConfig == nil {
			return nil
		}

		etag := strings.TrimSpace(aws.ToString(getResp.ETag))
		if etag == "" {
			return fmt.Errorf("cloudfront distribution etag missing")
		}

		if aws.ToBool(getResp.DistributionConfig.Enabled) {
			getResp.DistributionConfig.Enabled = boolPtr(false)
			_, err = client.UpdateDistribution(ctx, &cloudfront.UpdateDistributionInput{
				Id:                 &distributionID,
				IfMatch:            &etag,
				DistributionConfig: getResp.DistributionConfig,
			})
			if err != nil {
				return err
			}
			if err := waitForCloudFrontDeployed(ctx, client, distributionID, 15*time.Minute); err != nil {
				return fmt.Errorf("wait for cloudfront disable propagation: %w", err)
			}
			continue
		}

		_, err = client.DeleteDistribution(ctx, &cloudfront.DeleteDistributionInput{
			Id:      &distributionID,
			IfMatch: &etag,
		})
		if err == nil {
			return nil
		}
		code := awsErrorCode(err)
		if code == "NoSuchDistribution" {
			return nil
		}
		if code == "DistributionNotDisabled" {
			if waitErr := waitForCloudFrontDeployed(ctx, client, distributionID, 5*time.Minute); waitErr != nil {
				return fmt.Errorf("distribution not disabled yet: %w", waitErr)
			}
			continue
		}
		return err
	}

	return fmt.Errorf("cloudfront distribution is still disabling; retry delete in a few minutes")
}

func waitForCloudFrontDeployed(ctx context.Context, client *cloudfront.Client, distributionID string, timeout time.Duration) error {
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	waiter := cloudfront.NewDistributionDeployedWaiter(client)
	return waiter.Wait(waitCtx, &cloudfront.GetDistributionInput{
		Id: &distributionID,
	}, timeout)
}

func emptyAndDeleteBucket(ctx context.Context, cfg aws.Config, region string, bucketName string) error {
	regionValue := strings.TrimSpace(region)
	if regionValue == "" {
		regionValue = strings.TrimSpace(cfg.Region)
	}
	if regionValue == "" {
		regionValue = "us-west-1"
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.Region = regionValue
	})

	for attempt := 0; attempt < 3; attempt++ {
		if err := purgeBucketMultipartUploads(ctx, client, bucketName); err != nil {
			return err
		}
		if err := purgeBucketVersionedObjects(ctx, client, bucketName); err != nil {
			return err
		}
		if err := purgeBucketCurrentObjects(ctx, client, bucketName); err != nil {
			return err
		}

		_, err := client.DeleteBucket(ctx, &s3.DeleteBucketInput{
			Bucket: &bucketName,
		})
		if err == nil {
			return nil
		}

		code := awsErrorCode(err)
		if code == "NoSuchBucket" {
			return nil
		}
		if code == "BucketNotEmpty" {
			time.Sleep(2 * time.Second)
			continue
		}
		return err
	}

	return fmt.Errorf("bucket is still not empty after cleanup")
}

func purgeBucketMultipartUploads(ctx context.Context, client *s3.Client, bucketName string) error {
	paginator := s3.NewListMultipartUploadsPaginator(client, &s3.ListMultipartUploadsInput{
		Bucket: &bucketName,
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			code := awsErrorCode(err)
			if code == "NoSuchBucket" {
				return nil
			}
			return err
		}
		for _, upload := range page.Uploads {
			key := strings.TrimSpace(aws.ToString(upload.Key))
			uploadID := strings.TrimSpace(aws.ToString(upload.UploadId))
			if key == "" || uploadID == "" {
				continue
			}
			_, err := client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
				Bucket:   &bucketName,
				Key:      &key,
				UploadId: &uploadID,
			})
			if err != nil && awsErrorCode(err) != "NoSuchBucket" {
				return err
			}
		}
	}
	return nil
}

func purgeBucketVersionedObjects(ctx context.Context, client *s3.Client, bucketName string) error {
	paginator := s3.NewListObjectVersionsPaginator(client, &s3.ListObjectVersionsInput{
		Bucket: &bucketName,
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			code := awsErrorCode(err)
			if code == "NoSuchBucket" {
				return nil
			}
			return err
		}

		objects := make([]s3types.ObjectIdentifier, 0, len(page.Versions)+len(page.DeleteMarkers))
		for _, v := range page.Versions {
			key := strings.TrimSpace(aws.ToString(v.Key))
			versionID := strings.TrimSpace(aws.ToString(v.VersionId))
			if key == "" || versionID == "" {
				continue
			}
			objects = append(objects, s3types.ObjectIdentifier{
				Key:       &key,
				VersionId: &versionID,
			})
		}
		for _, marker := range page.DeleteMarkers {
			key := strings.TrimSpace(aws.ToString(marker.Key))
			versionID := strings.TrimSpace(aws.ToString(marker.VersionId))
			if key == "" || versionID == "" {
				continue
			}
			objects = append(objects, s3types.ObjectIdentifier{
				Key:       &key,
				VersionId: &versionID,
			})
		}
		if err := deleteS3ObjectsBatch(ctx, client, bucketName, objects); err != nil {
			return err
		}
	}
	return nil
}

func purgeBucketCurrentObjects(ctx context.Context, client *s3.Client, bucketName string) error {
	paginator := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
		Bucket: &bucketName,
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			code := awsErrorCode(err)
			if code == "NoSuchBucket" {
				return nil
			}
			return err
		}
		objects := make([]s3types.ObjectIdentifier, 0, len(page.Contents))
		for _, item := range page.Contents {
			key := strings.TrimSpace(aws.ToString(item.Key))
			if key == "" {
				continue
			}
			objects = append(objects, s3types.ObjectIdentifier{
				Key: &key,
			})
		}
		if err := deleteS3ObjectsBatch(ctx, client, bucketName, objects); err != nil {
			return err
		}
	}
	return nil
}

func deleteS3ObjectsBatch(ctx context.Context, client *s3.Client, bucketName string, objects []s3types.ObjectIdentifier) error {
	if len(objects) == 0 {
		return nil
	}

	const batchSize = 1000
	for start := 0; start < len(objects); start += batchSize {
		end := start + batchSize
		if end > len(objects) {
			end = len(objects)
		}
		chunk := objects[start:end]
		_, err := client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: &bucketName,
			Delete: &s3types.Delete{
				Objects: chunk,
				Quiet:   boolPtr(true),
			},
		})
		if err != nil {
			code := awsErrorCode(err)
			if code == "NoSuchBucket" {
				return nil
			}
			return err
		}
	}
	return nil
}

func awsErrorCode(err error) string {
	if err == nil {
		return ""
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		return strings.TrimSpace(apiErr.ErrorCode())
	}
	return ""
}
