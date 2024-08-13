package pubsub

import (
	"context"

	"cloud.google.com/go/pubsub"
	"github.com/pkg/errors"
	"google.golang.org/api/option"

	"github.com/bitrise-io/bitrise-webhooks/service/hook/common"
)

// Client ...
type Client struct {
	pubsubClient  *pubsub.Client
	pubsubTopicID string
}

// NewClient ...
func NewClient(projectID, serviceAccountJSON, pubsubTopicID string) (*Client, error) {
	client, err := pubsub.NewClient(context.Background(), projectID, option.WithCredentialsJSON([]byte(serviceAccountJSON)))
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &Client{pubsubClient: client, pubsubTopicID: pubsubTopicID}, nil
}

// PublishMetrics ...
func (c *Client) PublishMetrics(ctx context.Context, metrics common.Metrics) (err error) {
	if c == nil {
		return nil
	}
	if metrics == nil {
		return nil
	}

	b, err := metrics.Serialise()
	if err != nil {
		return errors.WithStack(err)
	}

	msg := pubsub.Message{Data: b}

	_ = c.pubsubClient.Topic(c.pubsubTopicID).Publish(ctx, &msg)

	return nil
}
