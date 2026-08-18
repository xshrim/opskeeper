package httpapi

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"
)

func TestRequestLoggerSkipsHealthChecksByDefault(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, nil))
	handler := requestLogger(logger, "/opskeeper", true)(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/opskeeper/health/ready", nil))
	if output.Len() != 0 {
		t.Fatalf("health check log = %q, want none", output.String())
	}

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/opskeeper/api/v1/teams", nil))
	if !strings.Contains(output.String(), "http request") {
		t.Fatalf("application request log = %q, want access log", output.String())
	}
}

func TestRequestLoggerCanLogHealthChecks(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, nil))
	handler := requestLogger(logger, "/", false)(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/health/live", nil))
	if !strings.Contains(output.String(), "http request") {
		t.Fatalf("health check log = %q, want access log", output.String())
	}
}

func TestTrustedProxyClientIPIgnoresUntrustedPeerHeaders(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "192.0.2.20:1234"
	request.Header.Set("X-Forwarded-For", "198.51.100.10")

	got := serveClientIP(request, []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")})
	if got != "192.0.2.20" {
		t.Fatalf("client IP = %q, want direct peer", got)
	}
}

func TestSecurityHeaders(t *testing.T) {
	handler := securityHeaders(true)(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "https://ops.example.com/", nil))
	for _, name := range []string{"Content-Security-Policy", "Permissions-Policy", "Referrer-Policy", "X-Content-Type-Options", "X-Frame-Options", "Strict-Transport-Security"} {
		if response.Header().Get(name) == "" {
			t.Fatalf("security header %s is empty", name)
		}
	}
}

func TestCORSPolicyAllowsConfiguredOriginAndRejectsUnknown(t *testing.T) {
	handler := corsPolicy([]string{"https://console.example.com"})(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))
	allowed := httptest.NewRequest(http.MethodOptions, "https://api.example.com/", nil)
	allowed.Host = "api.example.com"
	allowed.Header.Set("Origin", "https://console.example.com")
	allowed.Header.Set("Access-Control-Request-Method", http.MethodPost)
	allowedResponse := httptest.NewRecorder()
	handler.ServeHTTP(allowedResponse, allowed)
	if allowedResponse.Code != http.StatusNoContent || allowedResponse.Header().Get("Access-Control-Allow-Origin") != "https://console.example.com" {
		t.Fatalf("allowed CORS response = %d %#v", allowedResponse.Code, allowedResponse.Header())
	}

	denied := httptest.NewRequest(http.MethodPost, "https://api.example.com/", nil)
	denied.Host = "api.example.com"
	denied.Header.Set("Origin", "https://attacker.example.com")
	deniedResponse := httptest.NewRecorder()
	handler.ServeHTTP(deniedResponse, denied)
	if deniedResponse.Code != http.StatusForbidden {
		t.Fatalf("denied CORS response = %d, want 403", deniedResponse.Code)
	}
}

func TestCSRFProtectionRejectsCrossSiteWrite(t *testing.T) {
	handler := csrfProtection(nil)(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodPost, "https://ops.example.com/api", nil)
	request.Header.Set("Sec-Fetch-Site", "cross-site")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("cross-site response = %d, want 403", response.Code)
	}
}

func TestRequestBodyLimit(t *testing.T) {
	handler := requestBodyLimit(4)(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, err := io.ReadAll(request.Body)
		if err != nil {
			writeError(writer, request, http.StatusRequestEntityTooLarge, "request_too_large", "Request body is too large")
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/", strings.NewReader("12345")))
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized response = %d, want 413", response.Code)
	}
}

func TestClientRateLimiterRejectsBurst(t *testing.T) {
	limiter := newClientRateLimiter(1)
	handler := limiter.middleware(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	for index, want := range []int{http.StatusNoContent, http.StatusTooManyRequests} {
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.RemoteAddr = "192.0.2.10:1234"
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != want {
			t.Fatalf("request %d status = %d, want %d", index, response.Code, want)
		}
	}
}

func BenchmarkSecurityMiddleware(b *testing.B) {
	handler := securityHeaders(true)(corsPolicy(nil)(csrfProtection(nil)(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))))
	b.ReportAllocs()
	for b.Loop() {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "https://ops.example.com/", nil))
	}
}

func TestTrustedProxyClientIPUsesForwardedChain(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "10.0.0.3:1234"
	request.Header.Set("X-Forwarded-For", "198.51.100.10, 10.0.0.2")

	got := serveClientIP(request, []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")})
	if got != "198.51.100.10" {
		t.Fatalf("client IP = %q, want first untrusted address", got)
	}
}

func TestTrustedProxyClientIPUsesRealIPFallback(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "10.0.0.3:1234"
	request.Header.Set("X-Real-IP", "2001:db8::10")

	got := serveClientIP(request, []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")})
	if got != "2001:db8::10" {
		t.Fatalf("client IP = %q, want X-Real-IP address", got)
	}
}

func TestTrustedProxyClientIPRejectsMalformedForwardedChain(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "10.0.0.3:1234"
	request.Header.Set("X-Forwarded-For", "198.51.100.10, invalid")

	got := serveClientIP(request, []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")})
	if got != "10.0.0.3" {
		t.Fatalf("client IP = %q, want trusted peer after invalid chain", got)
	}
}

func serveClientIP(request *http.Request, trustedProxies []netip.Prefix) string {
	var clientIP string
	handler := trustedProxyClientIP(trustedProxies)(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		clientIP = requestClientIP(request)
	}))
	handler.ServeHTTP(httptest.NewRecorder(), request)
	return clientIP
}

func TestStatusRecorderKeepsFirstStatus(t *testing.T) {
	response := httptest.NewRecorder()
	recorder := &statusRecorder{ResponseWriter: response, status: http.StatusOK}

	recorder.WriteHeader(http.StatusCreated)
	recorder.WriteHeader(http.StatusInternalServerError)

	if recorder.status != http.StatusCreated {
		t.Fatalf("statusRecorder status = %d, want %d", recorder.status, http.StatusCreated)
	}
	if response.Code != http.StatusCreated {
		t.Fatalf("response status = %d, want %d", response.Code, http.StatusCreated)
	}
}

func TestStatusRecorderSupportsResponseController(t *testing.T) {
	response := &controllerResponseWriter{header: make(http.Header)}
	recorder := &statusRecorder{ResponseWriter: response, status: http.StatusOK}
	controller := http.NewResponseController(recorder)
	deadline := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)

	if err := controller.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}
	if err := controller.SetWriteDeadline(deadline); err != nil {
		t.Fatalf("SetWriteDeadline() error = %v", err)
	}
	if !response.flushed {
		t.Fatal("Flush() did not reach the underlying response writer")
	}
	if !response.writeDeadline.Equal(deadline) {
		t.Fatalf("write deadline = %v, want %v", response.writeDeadline, deadline)
	}
}

type controllerResponseWriter struct {
	header        http.Header
	flushed       bool
	writeDeadline time.Time
}

func (w *controllerResponseWriter) Header() http.Header {
	return w.header
}

func (w *controllerResponseWriter) Write(content []byte) (int, error) {
	return len(content), nil
}

func (w *controllerResponseWriter) WriteHeader(int) {}

func (w *controllerResponseWriter) Flush() {
	w.flushed = true
}

func (w *controllerResponseWriter) SetWriteDeadline(deadline time.Time) error {
	w.writeDeadline = deadline
	return nil
}
