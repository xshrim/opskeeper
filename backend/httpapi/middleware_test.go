package httpapi

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"
)

func TestTrustedProxyClientIPIgnoresUntrustedPeerHeaders(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "192.0.2.20:1234"
	request.Header.Set("X-Forwarded-For", "198.51.100.10")

	got := serveClientIP(request, []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")})
	if got != "192.0.2.20" {
		t.Fatalf("client IP = %q, want direct peer", got)
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
