package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/yourname/go-clean-base/config"
	domainModel "github.com/yourname/go-clean-base/internal/domain/model"
	domainSvc "github.com/yourname/go-clean-base/internal/domain/service"
	infraDTO "github.com/yourname/go-clean-base/internal/infrastructure/dto"
)

const defaultHTTPTimeout = 10 * time.Second

type notificationClient struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func NewNotificationClient(cfg *config.Config) domainSvc.INotificationClient {
	return &notificationClient{
		baseURL: cfg.Notification.BaseURL,
		apiKey:  cfg.Notification.APIKey,
		http:    &http.Client{Timeout: defaultHTTPTimeout},
	}
}

func (c *notificationClient) Send(ctx context.Context, n *domainModel.Notification) (string, error) {
	req := &infraDTO.NotificationRequest{To: n.To, Subject: n.Subject, Body: n.Body}
	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal notification: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/send", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("send notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.ErrorContext(ctx, "notification API error", "status", resp.StatusCode)
		return "", fmt.Errorf("notification API returned %d", resp.StatusCode)
	}

	var result infraDTO.NotificationResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	return result.MessageID, nil
}
