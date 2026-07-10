package supportgraphql

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	exiterrors "github.com/jviklun1/pp-volvo-cars/internal/errors"
)

const Endpoint = "https://support-content-service.weu-prod.ecpaz.volvocars.biz/api/graphql"

type Client struct {
	endpoint    string
	httpClient  *http.Client
	minInterval time.Duration
	maxAttempts int

	rateMu      sync.Mutex
	lastRequest time.Time
}

func NewClient() *Client {
	return newClient(Endpoint, &http.Client{Timeout: 30 * time.Second}, 100*time.Millisecond, 3)
}

func newClient(endpoint string, httpClient *http.Client, minInterval time.Duration, maxAttempts int) *Client {
	return &Client{endpoint: endpoint, httpClient: httpClient, minInterval: minInterval, maxAttempts: maxAttempts}
}

type graphqlRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

type graphqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func (client *Client) query(ctx context.Context, query string, variables map[string]any, output any) error {
	payload, err := json.Marshal(graphqlRequest{Query: query, Variables: variables})
	if err != nil {
		return err
	}
	for attempt := 0; attempt < client.maxAttempts; attempt++ {
		if err := client.wait(ctx); err != nil {
			return err
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.endpoint, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Accept", "application/json")
		response, err := client.httpClient.Do(request)
		if err != nil {
			if attempt+1 < client.maxAttempts {
				if err := sleepContext(ctx, retryDelay(attempt, "")); err != nil {
					return err
				}
				continue
			}
			return fmt.Errorf("query Volvo support GraphQL: %w", err)
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 20<<20))
		closeErr := response.Body.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		if response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500 {
			if attempt+1 < client.maxAttempts {
				if err := sleepContext(ctx, retryDelay(attempt, response.Header.Get("Retry-After"))); err != nil {
					return err
				}
				continue
			}
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			if response.StatusCode == http.StatusTooManyRequests {
				return fmt.Errorf("Volvo support rate limited: %w", exiterrors.ErrRateLimited)
			}
			if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
				return fmt.Errorf("Volvo support authentication failed: %w", exiterrors.ErrAuthentication)
			}
			return fmt.Errorf("Volvo support GraphQL returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
		}
		var envelope graphqlResponse
		if err := json.Unmarshal(body, &envelope); err != nil {
			return fmt.Errorf("decode Volvo support GraphQL response: %w", err)
		}
		if len(envelope.Errors) > 0 {
			messages := make([]string, 0, len(envelope.Errors))
			for _, item := range envelope.Errors {
				messages = append(messages, item.Message)
			}
			return errors.New(strings.Join(messages, "; "))
		}
		if output == nil {
			return nil
		}
		if err := json.Unmarshal(envelope.Data, output); err != nil {
			return fmt.Errorf("decode Volvo support GraphQL data: %w", err)
		}
		return nil
	}
	return fmt.Errorf("Volvo support GraphQL retry budget exhausted: %w", exiterrors.ErrRateLimited)
}

func (client *Client) wait(ctx context.Context) error {
	client.rateMu.Lock()
	defer client.rateMu.Unlock()
	wait := time.Until(client.lastRequest.Add(client.minInterval))
	if wait > 0 {
		if err := sleepContext(ctx, wait); err != nil {
			return err
		}
	}
	client.lastRequest = time.Now()
	return nil
}

func retryDelay(attempt int, retryAfter string) time.Duration {
	if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 100 * time.Millisecond
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
