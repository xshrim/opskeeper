package aiengine

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// KnowledgeQuery is the authorization-scoped input to a knowledge provider.
// Providers must apply ScopeID and ResourceIDs before returning any chunk.
type KnowledgeQuery struct {
	ScopeID         string
	Query           string
	KnowledgeBaseID string
	ResourceIDs     []string
	TopK            int
}

type KnowledgeChunk struct {
	ID              string         `json:"id"`
	KnowledgeBaseID string         `json:"knowledge_base_id"`
	DocumentID      string         `json:"document_id"`
	Title           string         `json:"title,omitempty"`
	Content         string         `json:"content"`
	SourceURI       string         `json:"source_uri,omitempty"`
	Score           float64        `json:"score,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
	Untrusted       bool           `json:"untrusted"`
}

type KnowledgeCitation struct {
	ChunkID   string  `json:"chunk_id"`
	Title     string  `json:"title,omitempty"`
	SourceURI string  `json:"source_uri,omitempty"`
	Start     int     `json:"start,omitempty"`
	End       int     `json:"end,omitempty"`
	Score     float64 `json:"score,omitempty"`
}

type RetrievalResult struct {
	Chunks      []KnowledgeChunk    `json:"chunks"`
	Citations   []KnowledgeCitation `json:"citations"`
	RetrievedAt time.Time           `json:"retrieved_at"`
}

type KnowledgeDocument struct {
	ID        string         `json:"id"`
	Title     string         `json:"title,omitempty"`
	Content   string         `json:"content"`
	SourceURI string         `json:"source_uri,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type KnowledgeBase struct {
	Documents []KnowledgeDocument `json:"documents"`
}

// SearchDocuments provides a deterministic baseline retriever for the
// resource-backed KnowledgeBase format. A future vector retriever can satisfy
// KnowledgeRetriever without changing the query/result contract.
func SearchDocuments(query KnowledgeQuery, base KnowledgeBase) (RetrievalResult, error) {
	if err := query.Validate(); err != nil {
		return RetrievalResult{}, err
	}
	terms := strings.Fields(strings.ToLower(query.Query))
	result := RetrievalResult{}
	for _, document := range base.Documents {
		text := strings.ToLower(document.Title + "\n" + document.Content)
		matched := true
		for _, term := range terms {
			if !strings.Contains(text, term) {
				matched = false
				break
			}
		}
		if !matched {
			continue
		}
		result.Chunks = append(result.Chunks, KnowledgeChunk{ID: document.ID, KnowledgeBaseID: query.KnowledgeBaseID, DocumentID: document.ID, Title: document.Title, Content: document.Content, SourceURI: document.SourceURI, Score: 1, Metadata: document.Metadata, Untrusted: true})
		result.Citations = append(result.Citations, KnowledgeCitation{ChunkID: document.ID, Title: document.Title, SourceURI: document.SourceURI, Score: 1})
		if query.TopK > 0 && len(result.Chunks) >= query.TopK {
			break
		}
	}
	return result.Normalize(), nil
}

// KnowledgeRetriever is intentionally narrow: retrieval is a tool operation,
// so implementations remain behind the same authorization and audit gateway.
type KnowledgeRetriever interface {
	Retrieve(ctx context.Context, query KnowledgeQuery) (RetrievalResult, error)
}

// KnowledgeRetrieverFunc makes it possible for HTTP and worker layers to
// adapt their resource stores without coupling the AIEngine package to them.
type KnowledgeRetrieverFunc func(context.Context, KnowledgeQuery) (RetrievalResult, error)

func (f KnowledgeRetrieverFunc) Retrieve(ctx context.Context, query KnowledgeQuery) (RetrievalResult, error) {
	if f == nil {
		return RetrievalResult{}, fmt.Errorf("knowledge retriever is unavailable")
	}
	return f(ctx, query)
}

func (q KnowledgeQuery) Validate() error {
	if strings.TrimSpace(q.ScopeID) == "" {
		return fmt.Errorf("knowledge query scope_id is required")
	}
	if strings.TrimSpace(q.Query) == "" {
		return fmt.Errorf("knowledge query query is required")
	}
	if q.TopK < 0 || q.TopK > 100 {
		return fmt.Errorf("knowledge query top_k must be between 0 and 100")
	}
	return nil
}

func (r RetrievalResult) Normalize() RetrievalResult {
	if r.RetrievedAt.IsZero() {
		r.RetrievedAt = time.Now().UTC()
	}
	if r.Chunks == nil {
		r.Chunks = []KnowledgeChunk{}
	}
	if r.Citations == nil {
		r.Citations = []KnowledgeCitation{}
	}
	return r
}
