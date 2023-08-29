package pubsub

import (
	"context"

	"cloud.google.com/go/pubsub"
	"github.com/bitrise-io/bitrise-webhooks/service/hook/common"
	"github.com/pkg/errors"
	"google.golang.org/api/option"
)

type Client struct {
	c             *pubsub.Client
	pubsubTopicID string
}

// Init ...
func NewClient(projectID, serviceAccountJSON, pubsubTopicID string) (*Client, error) {
	client, err := pubsub.NewClient(context.Background(), projectID, option.WithCredentialsJSON([]byte(serviceAccountJSON)))
	if err != nil {
		return nil, err
	}
	return &Client{c: client, pubsubTopicID: pubsubTopicID}, nil
}

func (c *Client) PublishMetrics(metrics common.MetricsResultModel) (err error) {
	if c == nil {
		return nil
	}

	msg := transformMetricsToMessage(metrics)

	topic := c.c.Topic(c.pubsubTopicID)
	result := topic.Publish(context.Background(), msg)
	serverID, err := result.Get(context.Background())
	if err != nil {
		return errors.Wrap(errors.WithStack(err), "serverID: "+serverID)
	}
	return nil
}

func transformMetricsToMessage(metrics common.MetricsResultModel) *pubsub.Message {
	return nil
}
