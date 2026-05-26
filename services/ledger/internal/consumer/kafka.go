package consumer

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/peltastic/payment-microservices-reference/ledger/internal/config"
	appLogger "github.com/peltastic/payment-microservices-reference/ledger/internal/logger"
	"github.com/peltastic/payment-microservices-reference/ledger/internal/metrics"
	"github.com/peltastic/payment-microservices-reference/ledger/internal/service"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type PaymentEvent struct {
	ID        string           `json:"id"`
	Type      string           `json:"type"`
	Version   string           `json:"version"`
	Timestamp time.Time        `json:"timestamp"`
	Source    string           `json:"source"`
	Data      PaymentEventData `json:"data"`
}

type PaymentEventData struct {
	PaymentID     string `json:"payment_id"`
	MerchantID    string `json:"merchant_id"`
	Amount        int64  `json:"amount"`
	Currency      string `json:"currency"`
	Status        string `json:"status"`
	CustomerEmail string `json:"customer_email"`
	CustomerName  string `json:"customer_name"`
	BankReference string `json:"bank_reference"`
	FailedReason  string `json:"failed_reason"`
}

type KafkaConsumer struct {
	readers            map[string]*kafka.Reader
	svc                *service.LedgerService
	eventSigningSecret string
}

func NewKafkaConsumer(ledgerService *service.LedgerService, cfg config.KafkaConfig) *KafkaConsumer {
	log := slog.Default().With("component", "kafka_consumer")
	log.Info("kafka consumer initialized", "brokers", cfg.Brokers, "group_id", cfg.GroupID, "topic", cfg.PaymentsTopic)

	readers := map[string]*kafka.Reader{
		cfg.PaymentsTopic: newReader(cfg.Brokers, cfg.GroupID, cfg.PaymentsTopic),
	}
	if cfg.PaymentsTopic != "payment.succeeded" {
		readers["payment.succeeded"] = newReader(cfg.Brokers, cfg.GroupID, "payment.succeeded")
	}

	return &KafkaConsumer{
		svc:                ledgerService,
		readers:            readers,
		eventSigningSecret: cfg.EventSigningSecret,
	}
}

func newReader(brokers []string, groupID, topic string) *kafka.Reader {
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:          topic,
		GroupID:        groupID,
		MinBytes:       1,
		MaxBytes:       10e6, // 10MB
		CommitInterval: 0,    // manual commit only — we commit after successful processing
		StartOffset:    kafka.FirstOffset,
	})
}

func (c *KafkaConsumer) logger(ctx context.Context) *slog.Logger {
	return appLogger.FromContext(ctx).With("component", "kafka_consumer")
}

func (c *KafkaConsumer) Start(ctx context.Context) {
	log := c.logger(ctx)
	log.Info("starting kafka consumers", "topics", topicNames(c.readers))
	for topic, reader := range c.readers {
		go c.consume(ctx, topic, reader)
	}
}

func (c *KafkaConsumer) consume(ctx context.Context, topic string, reader *kafka.Reader) {
	log := c.logger(ctx)
	log.Info("starting consumer", "topic", topic)

	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				log.Info("consumer stopping", "topic", topic, "error", ctx.Err())
				return
			}
			log.Error("failed to fetch message", "topic", topic, "error", err)
			time.Sleep(1 * time.Second)
			continue
		}

		log.Info("received message",
			"topic", topic,
			"offset", msg.Offset,
			"key", string(msg.Key),
			"payload_size", len(msg.Value),
		)
		updateConsumerLag(topic, reader)

		processed, err := c.handleMessage(ctx, topic, msg)
		if err != nil {
			metrics.KafkaConsumeTotal.WithLabelValues(topic, "failed").Inc()
			log.Error("failed to handle message",
				"topic", topic,
				"offset", msg.Offset,
				"error", err,
			)
			continue
		}

		if err := reader.CommitMessages(ctx, msg); err != nil {
			metrics.KafkaConsumeTotal.WithLabelValues(topic, "failed").Inc()
			log.Error("failed to commit offset",
				"topic", topic,
				"offset", msg.Offset,
				"error", err,
			)
			continue
		}
		status := "success"
		if !processed {
			status = "failed"
		}
		metrics.KafkaConsumeTotal.WithLabelValues(topic, status).Inc()
		log.Info("committed message offset",
			"topic", topic,
			"offset", msg.Offset,
		)
	}
}

func (c *KafkaConsumer) handleMessage(ctx context.Context, topic string, msg kafka.Message) (bool, error) {
	ctx = extractTraceContext(ctx, msg.Headers)
	ctx, span := otel.Tracer("ledger-service/kafka").Start(ctx, "consume payment event",
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.destination.name", topic),
			attribute.Int("messaging.kafka.partition", msg.Partition),
			attribute.Int64("messaging.kafka.offset", msg.Offset),
			attribute.Int("messaging.message.body.size", len(msg.Value)),
		),
	)
	defer span.End()

	log := c.logger(ctx).With(
		"topic", topic,
		"partition", msg.Partition,
		"offset", msg.Offset,
	)
	if !c.validEventSignature(msg) {
		log.Warn("payment event rejected due to invalid signature")
		return true, nil
	}

	var event PaymentEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		log.Error("failed to unmarshal payment event",
			"error", err,
			"payload_size", len(msg.Value),
		)
		return false, nil
	}

	span.SetAttributes(
		attribute.String("event.id", event.ID),
		attribute.String("event.type", event.Type),
		attribute.String("event.source", event.Source),
		attribute.String("payment.id", event.Data.PaymentID),
		attribute.String("merchant.id", event.Data.MerchantID),
	)

	switch event.Type {
	case "payment.succeeded":
		if err := c.handlePaymentSucceeded(ctx, event); err != nil {
			if errors.Is(err, service.ErrInvalidPaymentEvent) {
				log.Warn("invalid payment succeeded event skipped",
					"event_id", event.ID,
					"payment_id", event.Data.PaymentID,
					"merchant_id", event.Data.MerchantID,
					"error", err,
				)
				return true, nil
			}

			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return false, err
		}
		return true, nil
	case "payment.failed":
		log.Info("payment failed event ignored by ledger",
			"event_id", event.ID,
			"payment_id", event.Data.PaymentID,
			"merchant_id", event.Data.MerchantID,
			"failed_reason", event.Data.FailedReason,
		)
		return true, nil
	default:
		log.Warn("unknown payment event type, skipping",
			"event_id", event.ID,
			"event_type", event.Type,
			"payment_id", event.Data.PaymentID,
			"merchant_id", event.Data.MerchantID,
		)
		return true, nil
	}
}

func (c *KafkaConsumer) validEventSignature(msg kafka.Message) bool {
	if c.eventSigningSecret == "" {
		return false
	}

	signature := kafkaHeaderValue(msg.Headers, "x-event-signature")
	if signature == "" {
		return false
	}

	mac := hmac.New(sha256.New, []byte(c.eventSigningSecret))
	_, _ = mac.Write(msg.Value)
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(signature), []byte(expected))
}

func kafkaHeaderValue(headers []kafka.Header, key string) string {
	for _, header := range headers {
		if strings.EqualFold(header.Key, key) {
			return string(header.Value)
		}
	}

	return ""
}

func (c *KafkaConsumer) handlePaymentSucceeded(ctx context.Context, event PaymentEvent) error {
	log := c.logger(ctx)
	log.Info("payment succeeded event decoded",
		"event_id", event.ID,
		"event_type", event.Type,
		"version", event.Version,
		"source", event.Source,
		"payment_id", event.Data.PaymentID,
		"merchant_id", event.Data.MerchantID,
		"amount", event.Data.Amount,
		"currency", event.Data.Currency,
	)
	return c.svc.HandlePaymentSucceeded(ctx, service.PaymentEvent{
		ID:        event.ID,
		Type:      event.Type,
		Version:   event.Version,
		Timestamp: event.Timestamp,
		Source:    event.Source,
		Data: service.PaymentEventData{
			PaymentID:     event.Data.PaymentID,
			MerchantID:    event.Data.MerchantID,
			Amount:        event.Data.Amount,
			Currency:      event.Data.Currency,
			Status:        event.Data.Status,
			CustomerEmail: event.Data.CustomerEmail,
			CustomerName:  event.Data.CustomerName,
		},
	})
}

func (c *KafkaConsumer) Close() {
	log := slog.Default().With("component", "kafka_consumer")
	for topic, reader := range c.readers {
		if err := reader.Close(); err != nil {
			log.Error("failed to close kafka reader", "topic", topic, "error", err)
			continue
		}
		log.Info("kafka reader closed", "topic", topic)
	}
}

func extractTraceContext(ctx context.Context, headers []kafka.Header) context.Context {
	if len(headers) == 0 {
		return ctx
	}

	carrier := propagation.MapCarrier{}
	for _, header := range headers {
		if header.Key == "" {
			continue
		}
		carrier.Set(header.Key, string(header.Value))
	}

	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}

func topicNames(readers map[string]*kafka.Reader) []string {
	topics := make([]string, 0, len(readers))
	for topic := range readers {
		topics = append(topics, topic)
	}
	return topics
}

func updateConsumerLag(topic string, reader *kafka.Reader) {
	lag := reader.Stats().Lag
	if lag >= 0 {
		metrics.KafkaConsumerLag.WithLabelValues(topic).Set(float64(lag))
	}
}
