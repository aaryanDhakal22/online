// Package message wraps the AWS SQS publisher. The queue URL is taken from
// config (no GetQueueUrl round-trip, no STS lookup); otelaws is wired so
// every send becomes a span and the W3C traceparent rides along inside SQS
// MessageAttributes — main/'s consumer extracts it automatically and the
// trace chains end-to-end without any custom plumbing.
//
// Every outbound message carries string MessageAttributes:
//   - source       = "online"                  (routes main/'s caster registry)
//   - tenant_id    = ScopeConfig.TenantID      (optional; only when set — main/
//   - location_id  = ScopeConfig.LocationID     ignores both for source=online)
//
// The wire-level contract — what main/ actually receives — lives in
// online/SQS_CONTRACT.md. If you change the SendMessageInput shape below,
// update that doc at the same time so the two stay in sync.
package message

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws"
)

type MessageBroker struct {
	queueURL   string
	tenantID   string
	locationID string
	client     *sqs.Client
	logger     zerolog.Logger
}

// NewMessageBroker constructs the SQS publisher. endpoint is optional —
// when non-empty (dev/staging pointing at ElasticMQ), it overrides the
// AWS service endpoint. Prod leaves it empty so the SDK targets real AWS.
//
// tenantID and locationID are optional: each is stamped on outbound messages
// only when non-empty. main/ routes online orders by the body's store_id /
// store_name and doesn't require them for this source.
func NewMessageBroker(ctx context.Context, region, queueURL, endpoint, tenantID, locationID string, logger zerolog.Logger) (*MessageBroker, error) {
	if queueURL == "" {
		return nil, errors.New("SQS queue URL is required")
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}
	otelaws.AppendMiddlewares(&awsCfg.APIOptions)

	var clientOpts []func(*sqs.Options)
	if endpoint != "" {
		clientOpts = append(clientOpts, func(o *sqs.Options) {
			o.BaseEndpoint = aws.String(endpoint)
		})
	}

	return &MessageBroker{
		queueURL:   queueURL,
		tenantID:   tenantID,
		locationID: locationID,
		client:     sqs.NewFromConfig(awsCfg, clientOpts...),
		logger:     logger.With().Str("module", "message").Logger(),
	}, nil
}

// Publish sends the body verbatim to SQS on a standard (non-FIFO) queue.
// orderID is the SHA-256 hex digest of body, computed by the caller; it
// is used only for log/trace correlation here. Cross-retry idempotency
// is the consumer's responsibility — main/ can hash the body itself if
// it needs to dedupe. The actual SQS span is created by otelaws on the
// SDK middleware chain; we don't add our own.
func (mb *MessageBroker) Publish(ctx context.Context, orderID string, body []byte) error {
	mb.logger.Debug().Ctx(ctx).Str("order_id", orderID).Int("body_bytes", len(body)).
		Str("queue_url", mb.queueURL).Msg("sqs: SendMessage starting")

	bodyStr := string(body)
	_, err := mb.client.SendMessage(ctx, &sqs.SendMessageInput{
		MessageBody: aws.String(bodyStr),
		QueueUrl:    aws.String(mb.queueURL),
		MessageAttributes: messageAttributes(mb.tenantID, mb.locationID),
	})
	if err != nil {
		mb.logger.Error().Err(err).Ctx(ctx).Str("order_id", orderID).Msg("sqs: SendMessage failed")
		return err
	}
	mb.logger.Debug().Ctx(ctx).Str("order_id", orderID).Msg("sqs: SendMessage acked")
	return nil
}

// messageAttributes builds the SQS attributes: source=online always, plus the
// optional scope pair (each only when set — SQS rejects empty string values).
func messageAttributes(tenantID, locationID string) map[string]sqstypes.MessageAttributeValue {
	str := func(v string) sqstypes.MessageAttributeValue {
		return sqstypes.MessageAttributeValue{DataType: aws.String("String"), StringValue: aws.String(v)}
	}
	attrs := map[string]sqstypes.MessageAttributeValue{"source": str("online")}
	if tenantID != "" {
		attrs["tenant_id"] = str(tenantID)
	}
	if locationID != "" {
		attrs["location_id"] = str(locationID)
	}
	return attrs
}
