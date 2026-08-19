package httpapi

import (
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"path"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"golang.org/x/time/rate"
	"opskeeper/backend/observability"
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

func requestLogger(logger *slog.Logger, basePath string, ignoreHealthLogs bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			started := time.Now()
			recorder := &statusRecorder{ResponseWriter: writer, status: http.StatusOK}
			next.ServeHTTP(recorder, request)
			if ignoreHealthLogs && isHealthCheckRequest(request, basePath) {
				return
			}
			logger.Info("http request", "kind", "http-request",
				"reqid", middleware.GetReqID(request.Context()),
				"method", request.Method,
				"path", request.URL.Path,
				"clientip", requestClientIP(request),
				"status", recorder.status,
				"duration", time.Since(started).Round(time.Millisecond),
			)
		})
	}
}

func isHealthCheckRequest(request *http.Request, basePath string) bool {
	if request.Method != http.MethodGet {
		return false
	}
	return request.URL.Path == path.Join(basePath, "health/live") || request.URL.Path == path.Join(basePath, "health/ready")
}

func recoverer(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					observability.RecordError(request.Context(), "http", "panic")
					logger.Error("http panic", "kind", "error",
						"reqid", middleware.GetReqID(request.Context()),
						"error_type", "panic",
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

func securityHeaders(production bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			header := writer.Header()
			header.Set("Content-Security-Policy", "default-src 'self'; base-uri 'self'; frame-ancestors 'none'; form-action 'self'; object-src 'none'")
			header.Set("Permissions-Policy", "camera=(), geolocation=(), microphone=()")
			header.Set("Referrer-Policy", "no-referrer")
			header.Set("X-Content-Type-Options", "nosniff")
			header.Set("X-Frame-Options", "DENY")
			if production {
				header.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}
			next.ServeHTTP(writer, request)
		})
	}
}

func requestBodyLimit(maxBytes int64) func(http.Handler) http.Handler {
	if maxBytes <= 0 {
		maxBytes = 2 << 20
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.Body != nil {
				request.Body = http.MaxBytesReader(writer, request.Body, maxBytes)
			}
			next.ServeHTTP(writer, request)
		})
	}
}

func corsPolicy(allowed []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			origin := strings.ToLower(strings.TrimSpace(request.Header.Get("Origin")))
			if origin == "" {
				next.ServeHTTP(writer, request)
				return
			}
			if !sameRequestOrigin(request, origin) && !containsOrigin(allowed, origin) {
				writeError(writer, request, http.StatusForbidden, "origin_forbidden", "Request origin is not allowed")
				return
			}
			header := writer.Header()
			header.Set("Access-Control-Allow-Origin", origin)
			header.Set("Access-Control-Allow-Credentials", "true")
			header.Add("Vary", "Origin")
			if request.Method == http.MethodOptions && request.Header.Get("Access-Control-Request-Method") != "" {
				header.Set("Access-Control-Allow-Methods", "GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS")
				header.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")
				header.Set("Access-Control-Max-Age", "600")
				writer.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(writer, request)
		})
	}
}

func csrfProtection(allowed []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.Method == http.MethodGet || request.Method == http.MethodHead || request.Method == http.MethodOptions {
				next.ServeHTTP(writer, request)
				return
			}
			origin := strings.ToLower(strings.TrimSpace(request.Header.Get("Origin")))
			fetchSite := strings.ToLower(strings.TrimSpace(request.Header.Get("Sec-Fetch-Site")))
			if (origin != "" && !sameRequestOrigin(request, origin) && !containsOrigin(allowed, origin)) || fetchSite == "cross-site" {
				writeError(writer, request, http.StatusForbidden, "csrf_forbidden", "Cross-site state-changing request is not allowed")
				return
			}
			next.ServeHTTP(writer, request)
		})
	}
}

func sameRequestOrigin(request *http.Request, origin string) bool {
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}
	return strings.EqualFold(parsed.Host, request.Host)
}

func containsOrigin(allowed []string, origin string) bool {
	for _, candidate := range allowed {
		if strings.EqualFold(candidate, origin) {
			return true
		}
	}
	return false
}

type clientRateLimiter struct {
	mutex   sync.Mutex
	clients map[string]clientLimit
	limit   rate.Limit
	burst   int
	now     func() time.Time
}

type clientLimit struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func newClientRateLimiter(perMinute int) *clientRateLimiter {
	if perMinute <= 0 {
		perMinute = 600
	}
	return &clientRateLimiter{clients: make(map[string]clientLimit), limit: rate.Limit(float64(perMinute) / 60), burst: max(1, perMinute/10), now: time.Now}
}

func (l *clientRateLimiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		now := l.now()
		key := requestClientIP(request)
		l.mutex.Lock()
		entry, exists := l.clients[key]
		if !exists {
			entry.limiter = rate.NewLimiter(l.limit, l.burst)
		}
		entry.lastSeen = now
		allowed := entry.limiter.AllowN(now, 1)
		l.clients[key] = entry
		if len(l.clients) > 1000 {
			for client, candidate := range l.clients {
				if now.Sub(candidate.lastSeen) > 5*time.Minute {
					delete(l.clients, client)
				}
			}
		}
		l.mutex.Unlock()
		if !allowed {
			writer.Header().Set("Retry-After", "1")
			writeError(writer, request, http.StatusTooManyRequests, "rate_limited", "Request rate limit exceeded")
			return
		}
		next.ServeHTTP(writer, request)
	})
}
