package httpapi

import (
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"runtime/debug"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func trustedProxyClientIP(trustedProxies []netip.Prefix) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			peer, ok := parseRemoteAddress(request.RemoteAddr)
			if ok && addressInPrefixes(peer, trustedProxies) {
				if client, ok := forwardedClientAddress(request.Header, trustedProxies); ok {
					request.RemoteAddr = client.String()
				}
			}
			next.ServeHTTP(writer, request)
		})
	}
}

func forwardedClientAddress(header http.Header, trustedProxies []netip.Prefix) (netip.Addr, bool) {
	if values := header.Values("X-Forwarded-For"); len(values) != 0 {
		addresses := make([]netip.Addr, 0)
		for _, value := range values {
			for _, rawAddress := range strings.Split(value, ",") {
				address, err := netip.ParseAddr(strings.TrimSpace(rawAddress))
				if err != nil || address.Zone() != "" {
					return netip.Addr{}, false
				}
				addresses = append(addresses, address.Unmap())
			}
		}
		if len(addresses) == 0 {
			return netip.Addr{}, false
		}
		for index := len(addresses) - 1; index >= 0; index-- {
			if !addressInPrefixes(addresses[index], trustedProxies) {
				return addresses[index], true
			}
		}
		return addresses[0], true
	}

	if value := strings.TrimSpace(header.Get("X-Real-IP")); value != "" {
		address, err := netip.ParseAddr(value)
		if err == nil && address.Zone() == "" {
			return address.Unmap(), true
		}
	}
	return netip.Addr{}, false
}

func parseRemoteAddress(value string) (netip.Addr, bool) {
	if addressPort, err := netip.ParseAddrPort(value); err == nil {
		return addressPort.Addr().Unmap(), true
	}
	host, _, err := net.SplitHostPort(value)
	if err == nil {
		value = host
	}
	address, err := netip.ParseAddr(strings.Trim(value, "[]"))
	if err != nil || address.Zone() != "" {
		return netip.Addr{}, false
	}
	return address.Unmap(), true
}

func addressInPrefixes(address netip.Addr, prefixes []netip.Prefix) bool {
	for _, prefix := range prefixes {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}

func requestClientIP(request *http.Request) string {
	if address, ok := parseRemoteAddress(request.RemoteAddr); ok {
		return address.String()
	}
	return request.RemoteAddr
}

func (r *statusRecorder) WriteHeader(status int) {
	if r.wroteHeader {
		return
	}
	r.wroteHeader = true
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(content []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	return r.ResponseWriter.Write(content)
}

func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			started := time.Now()
			recorder := &statusRecorder{ResponseWriter: writer, status: http.StatusOK}
			next.ServeHTTP(recorder, request)
			logger.Info("http request",
				"request_id", middleware.GetReqID(request.Context()),
				"method", request.Method,
				"path", request.URL.Path,
				"client_ip", requestClientIP(request),
				"status", recorder.status,
				"duration_ms", time.Since(started).Milliseconds(),
			)
		})
	}
}

func recoverer(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.Error("http panic",
						"request_id", middleware.GetReqID(request.Context()),
						"panic", recovered,
						"stack", string(debug.Stack()),
					)
					writeError(writer, request, http.StatusInternalServerError, "internal_error", "Internal server error")
				}
			}()
			next.ServeHTTP(writer, request)
		})
	}
}
