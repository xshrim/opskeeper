package connector

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type httpExecutor struct {
	client           *http.Client
	baseURL          string
	secret           map[string]string
	maxResponseBytes int64
}

func newHTTPExecutor(target Target, client *http.Client, maxResponseBytes int64) (*httpExecutor, error) {
	baseURL := configString(target.Resource.Config, "url", "endpoint")
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil {
		return nil, connectorError(CategoryConfiguration, "parse connector URL", false, errors.New("url must be an http or https URL without embedded credentials"))
	}
	if client == nil {
		client = &http.Client{CheckRedirect: sameHostRedirect}
	}
	return &httpExecutor{
		client: client, baseURL: strings.TrimRight(parsed.String(), "/"),
		secret: secretFields(target.Secret), maxResponseBytes: maxResponseBytes,
	}, nil
}

func (e *httpExecutor) get(ctx context.Context, operation, path string, values url.Values) (json.RawMessage, error) {
	endpoint := e.baseURL + "/" + strings.TrimLeft(path, "/")
	if len(values) > 0 {
		endpoint += "?" + values.Encode()
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, connectorError(CategoryConfiguration, operation, false, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "opskeeper-connector/1")
	e.authorize(request)
	response, err := e.client.Do(request)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, connectorError(CategoryTimeout, operation, true, context.DeadlineExceeded)
		}
		return nil, connectorError(CategoryUpstream, operation, true, err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return nil, statusError(operation, response.StatusCode)
	}
	body, err := readLimited(response.Body, e.maxResponseBytes)
	if err != nil {
		return nil, connectorError(CategoryResponseTooLarge, operation, false, err)
	}
	return json.RawMessage(bytes.Clone(body)), nil
}

func (e *httpExecutor) authorize(request *http.Request) {
	if token := strings.TrimSpace(e.secret["token"]); token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	} else {
		username := e.secret["username"]
		password := e.secret["password"]
		if username != "" || password != "" {
			request.SetBasicAuth(username, password)
		}
	}
	if tenant := strings.TrimSpace(e.secret["tenant_id"]); tenant != "" {
		request.Header.Set("X-Scope-OrgID", tenant)
	}
}

func readLimited(reader io.Reader, maximum int64) ([]byte, error) {
	if maximum <= 0 {
		return nil, ErrResponseTooLarge
	}
	body, err := io.ReadAll(io.LimitReader(reader, maximum+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maximum {
		return nil, ErrResponseTooLarge
	}
	return body, nil
}

func secretFields(secret []byte) map[string]string {
	fields := make(map[string]string)
	if len(secret) == 0 {
		return fields
	}
	if json.Unmarshal(secret, &fields) == nil {
		return fields
	}
	fields["token"] = strings.TrimSpace(string(secret))
	return fields
}

func configString(config map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := config[key].(string); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func sameHostRedirect(request *http.Request, via []*http.Request) error {
	if len(via) >= 5 {
		return errors.New("too many redirects")
	}
	if len(via) > 0 && !strings.EqualFold(request.URL.Host, via[0].URL.Host) {
		return errors.New("cross-host redirect is not allowed")
	}
	return nil
}

func validateEnvelope(operation string, body json.RawMessage) (string, int, error) {
	var envelope struct {
		Status    string `json:"status"`
		ErrorType string `json:"errorType"`
		Error     string `json:"error"`
		Data      struct {
			ResultType string            `json:"resultType"`
			Result     []json.RawMessage `json:"result"`
			Alerts     []json.RawMessage `json:"alerts"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return "", 0, connectorError(CategoryUpstream, operation, false, fmt.Errorf("decode JSON response: %w", err))
	}
	if envelope.Status != "success" {
		return "", 0, connectorError(CategoryUpstream, operation, false, fmt.Errorf("upstream query failed with type %q", envelope.ErrorType))
	}
	count := len(envelope.Data.Result)
	if len(envelope.Data.Alerts) > 0 {
		count = len(envelope.Data.Alerts)
	}
	return envelope.Data.ResultType, count, nil
}
