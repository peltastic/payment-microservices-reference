package kafka

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/peltastic/payment-microservices-reference/payment/internal/config"
	"github.com/peltastic/payment-microservices-reference/payment/internal/logger"
	"github.com/segmentio/kafka-go"
)

type Event struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Version   string                 `json:"version"`
	Timestamp time.Time              `json:"timestamp"`
	Source    string                 `json:"source"`
	Data      map[string]interface{} `json:"data"`
}

type Producer struct {
	writer             *kafka.Writer
	topic              string
	eventSigningSecret string
}

func NewProducer(cfg config.KafkaConfig) *Producer {
	log := slog.Default().With("component", "kafka_producer")
	log.Info("kafka producer initialized", "brokers", cfg.Brokers, "topic", cfg.PaymentsTopic)

	return &Producer{
		writer:             newWriter(cfg.Brokers, cfg.PaymentsTopic),
		topic:              cfg.PaymentsTopic,
		eventSigningSecret: cfg.EventSigningSecret,
	}
}

func newWriter(brokers []string, topic string) *kafka.Writer {
	log := slog.Default().With("component", "kafka_producer")
	return &kafka.Writer{
		Addr:                   kafka.TCP(brokers...),
		Topic:                  topic,
		Balancer:               &kafka.Hash{},
		RequiredAcks:           kafka.RequireAll,
		Async:                  false,
		AllowAutoTopicCreation: true,
		ErrorLogger: kafka.LoggerFunc(func(msg string, args ...interface{}) {
			log.Error("kafka writer error", "message", fmt.Sprintf(msg, args...))
		}),
	}
}

func (p *Producer) Publish(ctx context.Context, eventType string, data map[string]interface{}) error {
	log := logger.FromContext(ctx).With("component", "kafka_producer")
	log.Info("kafka publish started",
		"event_type", eventType,
		"topic", p.topic,
		"payment_id", data["payment_id"],
		"merchant_id", data["merchant_id"],
		"amount", data["amount"],
		"currency", data["currency"],
	)

	event := Event{
		ID:        ulid.Make().String(),
		Type:      eventType,
		Version:   "1.0",
		Timestamp: time.Now().UTC(),
		Source:    "payment-service",
		Data:      data,
	}

	payload, err := json.Marshal(event)
	if err != nil {
		log.Error("failed to marshal kafka event",
			"event_id", event.ID,
			"event_type", eventType,
			"error", err,
		)
		return fmt.Errorf("failed to marshal event: %w", err)
	}
	if p.eventSigningSecret == "" {
		return fmt.Errorf("event signing secret is required")
	}

	merchantID, _ := data["merchant_id"].(string)

	if err := p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(merchantID),
		Value: payload,
		Headers: []kafka.Header{
			{Key: "x-event-signature", Value: []byte(signEventPayload(payload, p.eventSigningSecret))},
		},
	}); err != nil {
		log.Error("kafka publish failed",
			"event_id", event.ID,
			"event_type", eventType,
			"topic", p.topic,
			"merchant_id", merchantID,
			"error", err,
		)
		return err
	}

	log.Info("kafka publish completed",
		"event_id", event.ID,
		"event_type", eventType,
		"topic", p.topic,
		"merchant_id", merchantID,
	)
	return nil
}

func signEventPayload(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func (p *Producer) Topic() string {
	return p.topic
}

func (p *Producer) Close() {
	log := slog.Default().With("component", "kafka_producer")
	if err := p.writer.Close(); err != nil {
		log.Error("failed to close kafka writer", "topic", p.topic, "error", err)
		return
	}
	log.Info("kafka writer closed", "topic", p.topic)
}
