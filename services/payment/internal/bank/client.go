package bank

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/peltastic/payment-microservices-reference/payment/internal/config"
	"github.com/peltastic/payment-microservices-reference/payment/internal/logger"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type AuthorizeResponse struct {
	Success   bool   `json:"success"`
	Reference string `json:"reference"`
	Code      string `json:"code"`
	Message   string `json:"message"`
}

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(cfg config.BankConfig) *Client {
	log := slog.Default().With("component", "bank_client")
	log.Info("bank client initialized", "base_url", cfg.MockURL)
	return &Client{
		baseURL: cfg.MockURL,
		httpClient: &http.Client{
			Timeout:   10 * time.Second,
			Transport: otelhttp.NewTransport(http.DefaultTransport),
		},
	}
}

func (c *Client) AuthorizePayment(ctx context.Context) (*AuthorizeResponse, error) {
	log := logger.FromContext(ctx).With("component", "bank_client")
	targetURL := c.baseURL + "/bank/authorize"
	log.Info("bank authorization request started", "url", targetURL)
	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		targetURL,
		nil,
	)

	if err != nil {
		log.Error("failed to create bank authorization request", "url", targetURL, "error", err)
		return nil, fmt.Errorf("failed to marshal bank request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)

	if err != nil {
		log.Error("bank authorization request failed", "url", targetURL, "error", err)
		return nil, fmt.Errorf("bank unreachable: %w", err)
	}
	defer resp.Body.Close()
	log.Info("bank authorization response received",
		"url", targetURL,
		"status_code", resp.StatusCode,
	)
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		log.Error("bank authorization returned unexpected status",
			"url", targetURL,
			"status_code", resp.StatusCode,
			"response_body", string(body),
		)
		return nil, fmt.Errorf("bank returned unexpected status %d", resp.StatusCode)
	}

	var bankResp AuthorizeResponse
	if err := json.NewDecoder(resp.Body).Decode(&bankResp); err != nil {
		log.Error("failed to decode bank authorization response",
			"url", targetURL,
			"status_code", resp.StatusCode,
			"error", err,
		)
		return nil, fmt.Errorf("failed to decode bank response: %w", err)
	}

	log.Info("bank authorization response decoded",
		"success", bankResp.Success,
		"reference", bankResp.Reference,
		"code", bankResp.Code,
	)
	return &bankResp, nil
}
