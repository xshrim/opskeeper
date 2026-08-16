package connector

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"opskeeper/backend/resource"
)

func TestHTTPExecutorAuthenticationAndTenantHeaders(t *testing.T) {
	tests := []struct {
		name       string
		secret     string
		wantAuth   string
		wantTenant string
		configure  func(map[string]any)
	}{
		{name: "bearer token", secret: `{"token":"secret-token"}`, wantAuth: "Bearer secret-token"},
		{name: "basic auth", secret: `{"username":"operator","password":"secret"}`, wantAuth: "Basic b3BlcmF0b3I6c2VjcmV0"},
		{name: "Loki tenant", secret: `{"token":"secret-token"}`, wantAuth: "Bearer secret-token", wantTenant: "tenant-a", configure: func(config map[string]any) { config["tenant_id"] = "tenant-a" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				if got := request.Header.Get("Authorization"); got != test.wantAuth {
					t.Errorf("Authorization = %q, want %q", got, test.wantAuth)
				}
				if got := request.Header.Get("X-Scope-OrgID"); got != test.wantTenant {
					t.Errorf("X-Scope-OrgID = %q, want %q", got, test.wantTenant)
				}
				return response(http.StatusOK, `{"status":"success","data":{"result":[]}}`), nil
			})}

			config := map[string]any{"url": "https://example.test"}
			if test.configure != nil {
				test.configure(config)
			}
			target := Target{Resource: resource.Resource{Config: config}, Secret: []byte(test.secret)}
			executor, err := newHTTPExecutor(target, client, 1024)
			if err != nil {
				t.Fatalf("newHTTPExecutor() error = %v", err)
			}
			if tenant, ok := config["tenant_id"].(string); ok {
				executor.secret["tenant_id"] = tenant
			}
			if _, err := executor.get(context.Background(), "test request", "/ready", nil); err != nil {
				t.Fatalf("get() error = %v", err)
			}
		})
	}
}

func TestHTTPExecutorClassifiesStatusAndResponseFailures(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		body      string
		maximum   int64
		want      Category
		temporary bool
	}{
		{name: "unauthorized", status: http.StatusUnauthorized, body: `{"secret":"do-not-return"}`, maximum: 1024, want: CategoryAuthentication},
		{name: "rate limited", status: http.StatusTooManyRequests, body: `{}`, maximum: 1024, want: CategoryRateLimited, temporary: true},
		{name: "upstream failure", status: http.StatusBadGateway, body: `{}`, maximum: 1024, want: CategoryUpstream, temporary: true},
		{name: "response too large", status: http.StatusOK, body: strings.Repeat("x", 17), maximum: 16, want: CategoryResponseTooLarge},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return response(test.status, test.body), nil
			})}
			executor, err := newHTTPExecutor(Target{Resource: resource.Resource{Config: map[string]any{"url": "https://example.test"}}}, client, test.maximum)
			if err != nil {
				t.Fatalf("newHTTPExecutor() error = %v", err)
			}
			_, err = executor.get(context.Background(), "test request", "/", nil)
			category, temporary := classify(err)
			if category != test.want || temporary != test.temporary {
				t.Fatalf("classify(%v) = %q, %v; want %q, %v", err, category, temporary, test.want, test.temporary)
			}
			if strings.Contains(publicMessage(err), "do-not-return") {
				t.Fatalf("publicMessage() exposed upstream response: %q", publicMessage(err))
			}
		})
	}
}

func TestHTTPExecutorHonorsContextTimeout(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		<-request.Context().Done()
		return nil, request.Context().Err()
	})}
	executor, err := newHTTPExecutor(Target{Resource: resource.Resource{Config: map[string]any{"url": "https://example.test"}}}, client, 1024)
	if err != nil {
		t.Fatalf("newHTTPExecutor() error = %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	_, err = executor.get(ctx, "test timeout", "/", nil)
	if category, _ := classify(err); category != CategoryTimeout || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("get() error = %v, category = %q", err, category)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func response(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestValidateEnvelopeRejectsMalformedJSONAndFailedStatus(t *testing.T) {
	for _, body := range []string{`not-json`, `{"status":"error","errorType":"bad_data","error":"private detail"}`} {
		if _, _, err := validateEnvelope("query", []byte(body)); err == nil {
			t.Fatalf("validateEnvelope(%q) error = nil", body)
		}
	}
}
