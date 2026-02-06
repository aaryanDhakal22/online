package message

import (
	"context"

	"quicc/online/internal/domain/order"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type MessageBroker struct {
	sqsURL    string
	sqsClient *sqs.Client
	logger    *zerolog.Logger
}

func NewMessageBroker(queueName string, logger *zerolog.Logger) *MessageBroker {
	log.Debug().Msgf("Creating new message broker for queue %s", queueName)
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load SDK config")
		panic(err)
	}
	log.Debug().Msgf("Loaded SDK config: %+v", cfg)
	sqsClient := sqs.NewFromConfig(cfg)
	out, err := sqsClient.GetQueueUrl(context.TODO(), &sqs.GetQueueUrlInput{
		QueueName: &queueName,
	})
	log.Debug().Msgf("Got queue URL: %+v", out)
	if err != nil {
		panic(err)
	}
	log.Info().Msgf("Created new message broker for queue %s", queueName)

	mbLogger := logger.With().Str("service", "message").Logger()
	return &MessageBroker{
		sqsClient: sqsClient,
		sqsURL:    *out.QueueUrl,
		logger:    &mbLogger,
	}
}

func (mb *MessageBroker) Publish(orderID string, order order.Order) error {
	mb.logger.Debug().Msgf("Flattening order %s", orderID)
	orderString, err := order.Flatten()
	if err != nil {
		mb.logger.Error().Err(err).Msgf("Error flattening order %s", orderID)
		return err
	}
	mb.logger.Debug().Msgf("Flattened order %s", orderID)
	mb.logger.Info().Msgf("Publishing order %s", orderID)
	_, err = mb.sqsClient.SendMessage(context.TODO(), &sqs.SendMessageInput{
		MessageBody:            &orderString,
		QueueUrl:               &mb.sqsURL,
		MessageGroupId:         &orderID,
		MessageDeduplicationId: &orderID,
	})
	if err != nil {
		mb.logger.Error().Err(err).Msgf("Error publishing order %s", orderID)
		return err
	}
	return nil
}
