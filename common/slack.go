package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// SlackWebhookURL is the URL for the Slack webhook.
// TODO: Replace with the actual Webhook URL or load from configuration/env.
const SlackWebhookURL = "https://hooks.slack.com/services/YOUR/WEBHOOK/URL"

type SlackMessage struct {
	Text string `json:"text"`
}

// SendSlackNotification sends a notification to Slack with the provided details.
func SendSlackNotification(email string, geo string, revenue float64, orders int) error {
	message := fmt.Sprintf("Initial Sync Completed :\nEmail: %s\nGeo: %s\nRevenue: $%.2f\nOrders: %d", email, geo, revenue, orders)

	payload := SlackMessage{
		Text: message,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal slack payload: %w", err)
	}

	resp, err := http.Post(SlackWebhookURL, "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to send request to slack: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack API responded with status: %s", resp.Status)
	}

	return nil
}

// SendSlackMessage posts a plain-text message to the webhook configured in the
// SLACK_WEBHOOK_URL env var. If the var is unset, it is a no-op.
func SendSlackMessage(text string) error {
	webhook := os.Getenv("SLACK_WEBHOOK_URL")
	if webhook == "" {
		return nil
	}

	payloadBytes, err := json.Marshal(SlackMessage{Text: text})
	if err != nil {
		return fmt.Errorf("failed to marshal slack payload: %w", err)
	}

	resp, err := http.Post(webhook, "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to send request to slack: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack API responded with status: %s", resp.Status)
	}
	return nil
}
