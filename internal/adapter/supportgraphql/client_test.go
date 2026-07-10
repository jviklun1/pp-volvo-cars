package supportgraphql

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	exiterrors "github.com/jviklun1/pp-volvo-cars/internal/errors"
)

func TestClientRetriesRateLimit(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		if attempts.Add(1) == 1 {
			writer.Header().Set("Retry-After", "0")
			writer.WriteHeader(http.StatusTooManyRequests)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"data":{"markets":[]}}`))
	}))
	defer server.Close()

	client := newClient(server.URL, server.Client(), 0, 3)
	var data marketsData
	if err := client.query(context.Background(), marketsQuery, nil, &data); err != nil {
		t.Fatal(err)
	}
	if attempts.Load() != 2 {
		t.Fatalf("attempts = %d, want 2", attempts.Load())
	}
}

func TestClientClassifiesExhaustedRateLimit(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		writer.Header().Set("Retry-After", "0")
		writer.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := newClient(server.URL, server.Client(), 0, 3)
	err := client.query(context.Background(), marketsQuery, nil, &marketsData{})
	if !errors.Is(err, exiterrors.ErrRateLimited) {
		t.Fatalf("error = %v, want wrapped ErrRateLimited", err)
	}
	if got := exiterrors.CodeOf(err); got != exiterrors.CodeRateLimited {
		t.Fatalf("CodeOf(error) = %d, want %d", got, exiterrors.CodeRateLimited)
	}
	if attempts.Load() != 3 {
		t.Fatalf("attempts = %d, want 3", attempts.Load())
	}
}

func TestClientClassifiesAuthenticationFailure(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(status)
			}))
			defer server.Close()

			client := newClient(server.URL, server.Client(), 0, 1)
			err := client.query(context.Background(), marketsQuery, nil, &marketsData{})
			if !errors.Is(err, exiterrors.ErrAuthentication) {
				t.Fatalf("error = %v, want wrapped ErrAuthentication", err)
			}
			if got := exiterrors.CodeOf(err); got != exiterrors.CodeAuthentication {
				t.Fatalf("CodeOf(error) = %d, want %d", got, exiterrors.CodeAuthentication)
			}
		})
	}
}
