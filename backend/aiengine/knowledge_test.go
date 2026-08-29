package aiengine

import "testing"

func TestKnowledgeQueryValidation(t *testing.T) {
	if err := (KnowledgeQuery{ScopeID: "scope-1", Query: "latency", TopK: 5}).Validate(); err != nil {
		t.Fatal(err)
	}
	for _, query := range []KnowledgeQuery{{Query: "latency"}, {ScopeID: "scope-1"}, {ScopeID: "scope-1", Query: "latency", TopK: 101}} {
		if err := query.Validate(); err == nil {
			t.Fatalf("invalid query accepted: %#v", query)
		}
	}
}

func TestRetrievalResultNormalize(t *testing.T) {
	result := (RetrievalResult{}).Normalize()
	if result.RetrievedAt.IsZero() || result.Chunks == nil || result.Citations == nil {
		t.Fatalf("result was not normalized: %#v", result)
	}
}

func TestSearchDocumentsReturnsTraceableMatchingChunks(t *testing.T) {
	result, err := SearchDocuments(KnowledgeQuery{ScopeID: "scope-1", KnowledgeBaseID: "kb-1", Query: "postgres latency", TopK: 1}, KnowledgeBase{Documents: []KnowledgeDocument{
		{ID: "doc-1", Title: "Database", Content: "Postgres latency budget", SourceURI: "runbook://db"},
		{ID: "doc-2", Title: "Redis", Content: "Redis memory"},
	}})
	if err != nil || len(result.Chunks) != 1 || len(result.Citations) != 1 || result.Chunks[0].ID != "doc-1" || !result.Chunks[0].Untrusted {
		t.Fatalf("search result = %#v, err=%v", result, err)
	}
}
