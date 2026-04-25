package aws

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

type STSAssumeRoleVerifier struct {
	sessionNamePrefix string
	duration          time.Duration
}

func NewSTSAssumeRoleVerifier() STSAssumeRoleVerifier {
	return STSAssumeRoleVerifier{
		sessionNamePrefix: "labra-verify",
		duration:          15 * time.Minute,
	}
}

var sessionNameSanitizer = regexp.MustCompile(`[^A-Za-z0-9+=,.@-]+`)

func (v STSAssumeRoleVerifier) Verify(ctx context.Context, in AssumeRoleInput) (string, error) {
	// Keep strict local format checks first so user errors fail fast and clearly.
	expectedAccountID, err := LocalAssumeRoleVerifier{}.Verify(ctx, in)
	if err != nil {
		return "", err
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(strings.TrimSpace(in.Region)))
	if err != nil {
		return "", fmt.Errorf("load aws config: %w", err)
	}

	client := sts.NewFromConfig(cfg)
	_, err = client.AssumeRole(ctx, &sts.AssumeRoleInput{
		RoleArn:         &in.RoleARN,
		RoleSessionName: awsString(buildSessionName(v.sessionNamePrefix)),
		ExternalId:      &in.ExternalID,
		DurationSeconds: awsInt32(int32(v.duration.Seconds())),
	})
	if err != nil {
		return "", fmt.Errorf("sts assume role failed: %w", err)
	}

	return expectedAccountID, nil
}

func buildSessionName(prefix string) string {
	safePrefix := sessionNameSanitizer.ReplaceAllString(strings.TrimSpace(prefix), "-")
	if safePrefix == "" {
		safePrefix = "labra-verify"
	}
	name := fmt.Sprintf("%s-%d", safePrefix, time.Now().Unix())
	if len(name) > 64 {
		return name[:64]
	}
	return name
}

func awsString(v string) *string {
	return &v
}

func awsInt32(v int32) *int32 {
	return &v
}
