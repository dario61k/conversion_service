package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/dario61k/conversion-service/internal/config"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Client struct {
	conn           *amqp.Connection
	publishChannel *amqp.Channel
	cfg            config.Config
	publishMu      sync.Mutex
}

func NewClient(cfg config.Config) (*Client, error) {
	conn, err := amqp.Dial(cfg.AMQPURL)
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}

	if err := declareTopology(ch, cfg); err != nil {
		return nil, err
	}

	return &Client{conn: conn, publishChannel: ch, cfg: cfg}, nil
}

func (c *Client) Close() error {
	if c.publishChannel != nil {
		_ = c.publishChannel.Close()
	}
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

type Consumer struct {
	channel    *amqp.Channel
	deliveries <-chan amqp.Delivery
}

func (c *Consumer) Deliveries() <-chan amqp.Delivery {
	return c.deliveries
}

func (c *Consumer) Close() error {
	if c.channel != nil {
		return c.channel.Close()
	}
	return nil
}

func (c *Client) NewConsumer() (*Consumer, error) {
	ch, err := c.conn.Channel()
	if err != nil {
		return nil, err
	}

	if err := declareTopology(ch, c.cfg); err != nil {
		_ = ch.Close()
		return nil, err
	}

	if err := ch.Qos(c.cfg.AMQPPrefetch, 0, false); err != nil {
		_ = ch.Close()
		return nil, err
	}

	deliveries, err := ch.Consume(
		c.cfg.AMQPQueueBuild,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		_ = ch.Close()
		return nil, err
	}

	return &Consumer{channel: ch, deliveries: deliveries}, nil
}

func (c *Client) PublishBuildRequest(ctx context.Context, msg VideoBuildRequested) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	c.publishMu.Lock()
	defer c.publishMu.Unlock()
	return c.publishChannel.PublishWithContext(ctx, c.cfg.AMQPExchange, c.cfg.AMQPQueueBuild, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now(),
		Body:         payload,
	})
}

func (c *Client) PublishRetry(ctx context.Context, msg VideoBuildRequested) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	c.publishMu.Lock()
	defer c.publishMu.Unlock()
	return c.publishChannel.PublishWithContext(ctx, c.cfg.AMQPExchange, c.cfg.AMQPQueueRetry, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now(),
		Body:         payload,
	})
}

func (c *Client) PublishDLQ(ctx context.Context, msg VideoBuildRequested, reason string) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	c.publishMu.Lock()
	defer c.publishMu.Unlock()
	return c.publishChannel.PublishWithContext(ctx, c.cfg.AMQPExchange, c.cfg.AMQPQueueDLQ, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now(),
		Body:         payload,
		Headers:      amqp.Table{"reason": reason},
	})
}

func (c *Client) ParseMessage(delivery amqp.Delivery) (VideoBuildRequested, error) {
	var msg VideoBuildRequested
	if err := json.Unmarshal(delivery.Body, &msg); err != nil {
		return VideoBuildRequested{}, fmt.Errorf("invalid payload: %w", err)
	}
	return msg, nil
}

func declareTopology(ch *amqp.Channel, cfg config.Config) error {
	if err := ch.ExchangeDeclare(
		cfg.AMQPExchange,
		"direct",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return err
	}

	argsRetry := amqp.Table{
		"x-dead-letter-exchange":    cfg.AMQPExchange,
		"x-dead-letter-routing-key": cfg.AMQPQueueBuild,
		"x-message-ttl":             int32(cfg.AMQPRetryTTLMS),
	}

	if _, err := ch.QueueDeclare(cfg.AMQPQueueBuild, true, false, false, false, nil); err != nil {
		return err
	}
	if _, err := ch.QueueDeclare(cfg.AMQPQueueRetry, true, false, false, false, argsRetry); err != nil {
		return err
	}
	if _, err := ch.QueueDeclare(cfg.AMQPQueueDLQ, true, false, false, false, nil); err != nil {
		return err
	}

	if err := ch.QueueBind(cfg.AMQPQueueBuild, cfg.AMQPQueueBuild, cfg.AMQPExchange, false, nil); err != nil {
		return err
	}
	if err := ch.QueueBind(cfg.AMQPQueueRetry, cfg.AMQPQueueRetry, cfg.AMQPExchange, false, nil); err != nil {
		return err
	}
	if err := ch.QueueBind(cfg.AMQPQueueDLQ, cfg.AMQPQueueDLQ, cfg.AMQPExchange, false, nil); err != nil {
		return err
	}

	return nil
}
