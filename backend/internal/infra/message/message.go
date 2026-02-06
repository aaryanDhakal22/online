package message

import (
	"context"

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

func NewMessageBroker(queueName string) *MessageBroker {
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
	return &MessageBroker{
		sqsClient: sqsClient,
		sqsURL:    *out.QueueUrl,
	}
}

func (mb *MessageBroker) Publish(orderID string, payload string) error {
	_, err := mb.sqsClient.SendMessage(context.TODO(), &sqs.SendMessageInput{
		MessageBody:            payload,
		QueueUrl:               &mb.sqsURL,
		MessageGroupId:         &orderID,
		MessageDeduplicationId: &orderID,
	})
	if err != nil {
		return err
	}
	return nil
}
