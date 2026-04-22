package message

import (
	"context"

	"quicc/online/internal/domain/order"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/rs/zerolog"
)

type MessageBroker struct {
	sqsURL         string
	sqsClient      *sqs.Client
	stagingURL     string // empty when staging is disabled
	stagingClient  *sqs.Client
	logger         zerolog.Logger
}

func NewMessageBroker(queueName string, stagingQueueName string, logger zerolog.Logger) *MessageBroker {
	mbLogger := logger.With().Str("service", "message").Logger()
	mbLogger.Debug().Msgf("Creating new message broker for queue %s", queueName)
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load SDK config")
		panic(err)
	}

	stsClient := sts.NewFromConfig(cfg)
	stsOut, err := stsClient.GetCallerIdentity(context.TODO(), &sts.GetCallerIdentityInput{})
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to get caller identity")
		panic(err)
	}
	mbLogger.Debug().Msgf("Got caller UserID: %v", *stsOut.UserId)
	mbLogger.Debug().Msgf("Got caller Account: %v", *stsOut.Account)
	mbLogger.Debug().Msgf("Got caller Arn: %v", *stsOut.Arn)

	sqsClient := sqs.NewFromConfig(cfg)
	out, err := sqsClient.GetQueueUrl(context.TODO(), &sqs.GetQueueUrlInput{
		QueueName: &queueName,
	})
	if err != nil {
		panic(err)
	}
	mbLogger.Debug().Msgf("Got queue URL: %v", *out.QueueUrl)

	mb := &MessageBroker{
		sqsClient: sqsClient,
		sqsURL:    *out.QueueUrl,
		logger:    mbLogger,
	}

	if stagingQueueName != "" {
		stagingOut, err := sqsClient.GetQueueUrl(context.TODO(), &sqs.GetQueueUrlInput{
			QueueName: &stagingQueueName,
		})
		if err != nil {
			mbLogger.Warn().Err(err).Str("staging_queue", stagingQueueName).Msg("Failed to resolve staging queue URL — staging publish disabled")
		} else {
			mb.stagingURL = *stagingOut.QueueUrl
			mb.stagingClient = sqsClient
			mbLogger.Info().Str("staging_url", mb.stagingURL).Msg("Staging queue configured")
		}
	}

	mbLogger.Info().Msgf("Created new message broker for queue %s", queueName)
	return mb
}

func (mb *MessageBroker) Publish(orderID string, order order.Order) error {
	mb.logger.Debug().Msgf("Flattening order %s", orderID)
	orderString, err := order.Flatten()
	if err != nil {
		mb.logger.Error().Err(err).Msgf("Error flattening order %s", orderID)
		return err
	}

	mb.logger.Info().Msgf("Publishing order %s to production queue", orderID)
	_, err = mb.sqsClient.SendMessage(context.TODO(), &sqs.SendMessageInput{
		MessageBody:    &orderString,
		QueueUrl:       &mb.sqsURL,
		MessageGroupId: &orderID,
	})
	if err != nil {
		mb.logger.Error().Err(err).Msgf("Error publishing order %s to production queue", orderID)
		return err
	}

	if mb.stagingURL != "" {
		mb.logger.Info().Msgf("Publishing order %s to staging queue", orderID)
		_, stagingErr := mb.stagingClient.SendMessage(context.TODO(), &sqs.SendMessageInput{
			MessageBody:    &orderString,
			QueueUrl:       &mb.stagingURL,
			MessageGroupId: &orderID,
		})
		if stagingErr != nil {
			mb.logger.Error().Err(stagingErr).Msgf("Error publishing order %s to staging queue — production unaffected", orderID)
		}
	}

	return nil
}
