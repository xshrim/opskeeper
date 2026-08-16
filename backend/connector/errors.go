package connector

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
)

type Category string

const (
	CategoryConfiguration    Category = "configuration"
	CategoryAuthentication   Category = "authentication"
	CategoryTimeout          Category = "timeout"
	CategoryRateLimited      Category = "rate_limited"
	CategoryResponseTooLarge Category = "response_too_large"
	CategoryUpstream         Category = "upstream"
	CategoryUnsupported      Category = "unsupported"
	CategoryInternal         Category = "internal"
)

var (
	ErrNotFound         = errors.New("connection check not found")
	ErrInvalid          = errors.New("invalid connector request")
	ErrUnsupported      = errors.New("connector is not supported")
	ErrRateLimited      = errors.New("connector concurrency limit reached")
	ErrResponseTooLarge = errors.New("connector response exceeds size limit")
)

type Error struct {
	Category  Category
	Operation string
	Temporary bool
	Err       error
}

func (e *Error) Error() string {
	if e.Operation == "" {
		return e.Err.Error()
	}
	return e.Operation + ": " + e.Err.Error()
}

func (e *Error) Unwrap() error { return e.Err }

func connectorError(category Category, operation string, temporary bool, err error) error {
	if err == nil {
		err = errors.New(string(category))
	}
	return &Error{Category: category, Operation: operation, Temporary: temporary, Err: err}
}

func classify(err error) (Category, bool) {
	if err == nil {
		return "", false
	}
	var connectorErr *Error
	if errors.As(err, &connectorErr) {
		return connectorErr.Category, connectorErr.Temporary
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return CategoryTimeout, true
	}
	if errors.Is(err, ErrRateLimited) {
		return CategoryRateLimited, true
	}
	if errors.Is(err, ErrResponseTooLarge) {
		return CategoryResponseTooLarge, false
	}
	if errors.Is(err, ErrUnsupported) {
		return CategoryUnsupported, false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return CategoryUpstream, netErr.Timeout() || netErr.Temporary()
	}
	return CategoryInternal, false
}

func statusError(operation string, status int) error {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return connectorError(CategoryAuthentication, operation, false, fmt.Errorf("upstream returned HTTP %d", status))
	case http.StatusTooManyRequests:
		return connectorError(CategoryRateLimited, operation, true, fmt.Errorf("upstream returned HTTP %d", status))
	default:
		return connectorError(CategoryUpstream, operation, status >= 500, fmt.Errorf("upstream returned HTTP %d", status))
	}
}

func publicMessage(err error) string {
	category, _ := classify(err)
	switch category {
	case CategoryConfiguration:
		return "连接配置无效"
	case CategoryAuthentication:
		return "上游拒绝认证或授权"
	case CategoryTimeout:
		return "连接上游超时"
	case CategoryRateLimited:
		return "连接请求受到限流"
	case CategoryResponseTooLarge:
		return "上游响应超过允许大小"
	case CategoryUnsupported:
		return "该资源类型尚无可用 Connector"
	case CategoryUpstream:
		return "上游服务不可用或返回错误"
	default:
		return "连接测试失败"
	}
}

func invalid(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalid, strings.TrimSpace(message))
}
