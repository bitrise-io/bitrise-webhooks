package pubsub

import (
	"context"

	"cloud.google.com/go/pubsub"
	"github.com/bitrise-io/bitrise-webhooks/service/hook/common"
	"github.com/pkg/errors"
	"google.golang.org/api/option"
)

// Client ...
type Client struct {
	c             *pubsub.Client
	pubsubTopicID string
}

// NewClient ...
func NewClient(projectID, serviceAccountJSON, pubsubTopicID string) (*Client, error) {
	client, err := pubsub.NewClient(context.Background(), projectID, option.WithCredentialsJSON([]byte(serviceAccountJSON)))
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &Client{c: client, pubsubTopicID: pubsubTopicID}, nil
}

// PublishMetrics ...
func (c *Client) PublishMetrics(metrics common.Metrics) (err error) {
	if c == nil {
		return nil
	}

	b, err := metrics.Serialise()
	if err != nil {
		return errors.WithStack(err)
	}

	msg := pubsub.Message{Data: b}

	topic := c.c.Topic(c.pubsubTopicID)
	result := topic.Publish(context.Background(), &msg)
	serverID, err := result.Get(context.Background())
	if err != nil {
		return errors.Wrap(errors.WithStack(err), "serverID: "+serverID)
	}
	return nil
}