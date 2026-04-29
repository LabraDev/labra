package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

type bedrockConverseProvider struct {
	client   *bedrockruntime.Client
	region   string
	initErr  error
	provider string
}

func newBedrockConverseProvider(region string) AIProvider {
	effectiveRegion := strings.TrimSpace(region)
	if effectiveRegion == "" {
		effectiveRegion = "us-west-1"
	}

	cfg, err := awsconfig.LoadDefaultConfig(context.Background(), awsconfig.WithRegion(effectiveRegion))
	if err != nil {
		return bedrockConverseProvider{
			region:   effectiveRegion,
			provider: "aws-bedrock",
			initErr:  fmt.Errorf("load aws config for bedrock: %w", err),
		}
	}

	return bedrockConverseProvider{
		client:   bedrockruntime.NewFromConfig(cfg),
		region:   effectiveRegion,
		provider: "aws-bedrock",
	}
}

func (p bedrockConverseProvider) Generate(ctx context.Context, in AIProviderInput) (AIProviderOutput, error) {
	if p.initErr != nil {
		return AIProviderOutput{}, p.initErr
	}
	if p.client == nil {
		return AIProviderOutput{}, fmt.Errorf("bedrock runtime client is not initialized")
	}

	prompt := strings.TrimSpace(in.Prompt)
	if prompt == "" {
		return AIProviderOutput{}, fmt.Errorf("empty prompt")
	}

	model := strings.TrimSpace(in.Model)
	if model == "" {
		model = "us.amazon.nova-lite-v1:0"
	}

	userText := types.ContentBlockMemberText{Value: prompt}
	message := types.Message{
		Role:    "user",
		Content: []types.ContentBlock{&userText},
	}

	resp, err := p.client.Converse(ctx, &bedrockruntime.ConverseInput{
		ModelId:  aws.String(model),
		Messages: []types.Message{message},
		InferenceConfig: &types.InferenceConfiguration{
			MaxTokens:   aws.Int32(700),
			Temperature: aws.Float32(0.2),
		},
	})
	if err != nil {
		return AIProviderOutput{}, fmt.Errorf("call bedrock converse: %w", err)
	}

	outputMessage, ok := resp.Output.(*types.ConverseOutputMemberMessage)
	if !ok || outputMessage == nil {
		return AIProviderOutput{}, fmt.Errorf("bedrock response contained no message output")
	}

	segments := make([]string, 0, len(outputMessage.Value.Content))
	for _, block := range outputMessage.Value.Content {
		textBlock, ok := block.(*types.ContentBlockMemberText)
		if !ok {
			continue
		}
		if text := strings.TrimSpace(textBlock.Value); text != "" {
			segments = append(segments, text)
		}
	}

	text := strings.TrimSpace(strings.Join(segments, "\n"))
	if text == "" {
		return AIProviderOutput{}, fmt.Errorf("bedrock response contained no output text")
	}

	return AIProviderOutput{
		Text:       text,
		Provider:   p.provider,
		Model:      model,
		Confidence: "medium",
	}, nil
}
