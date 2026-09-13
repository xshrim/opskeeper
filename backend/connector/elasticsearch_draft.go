package connector

import (
	"context"
	"fmt"
	"time"

	es "opskeeper/backend/tool/elasticsearch"
)

type ElasticsearchDraftInput struct {
	URL            string `json:"url"`
	Username       string `json:"username,omitempty"`
	Password       string `json:"password,omitempty"`
	TLSInsecure    bool   `json:"tls_insecure,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}
type ElasticsearchDraftCheck struct {
	Status    string `json:"status"`
	Message   string `json:"message"`
	LatencyMS int64  `json:"latency_ms"`
}

func (s *Service) TestElasticsearchDraft(ctx context.Context, in ElasticsearchDraftInput) (ElasticsearchDraftCheck, error) {
	started := time.Now()
	out := ElasticsearchDraftCheck{Status: "failed"}
	c, e := es.New(es.ConnectionInput{URL: in.URL, Username: in.Username, Password: in.Password, TLSInsecure: in.TLSInsecure, TimeoutSeconds: in.TimeoutSeconds})
	if e == nil {
		_, e = c.Health(ctx)
	}
	out.LatencyMS = time.Since(started).Milliseconds()
	if e != nil {
		out.Message = fmt.Sprintf("Elasticsearch 连接测试失败：%s", e)
		return out, nil
	}
	out.Status = "succeeded"
	out.Message = "Elasticsearch 集群连接测试通过"
	return out, nil
}
