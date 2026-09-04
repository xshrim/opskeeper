package diagnosis

import "testing"

func TestValidateCausalDraftKeepsOnlySessionEvidence(t *testing.T) {
	evidence := []Evidence{{ID: "evidence-1", SessionID: "session-1"}}
	draft := causalChainDraft{
		Summary: "连接池限制导致请求等待并最终超时。",
		Nodes: []CausalNode{
			{ID: "n1", Kind: "cause", Statement: "连接池配置过小", Status: "likely", Confidence: 0.7, EvidenceIDs: []string{"evidence-1", "fabricated"}},
			{ID: "n2", Kind: "effect", Statement: "请求超时", Status: "unverified", EvidenceIDs: []string{"fabricated"}},
		},
		Links: []CausalLink{{From: "n1", To: "n2", Relation: "causes", Status: "likely", Confidence: 0.7, EvidenceIDs: []string{"evidence-1", "fabricated"}}},
	}
	chain, ok := validateCausalDraft("session-1", "run-1", draft, evidence)
	if !ok {
		t.Fatal("validateCausalDraft() rejected a valid evidence-backed chain")
	}
	if got := chain.Nodes[0].EvidenceIDs; len(got) != 1 || got[0] != "evidence-1" {
		t.Fatalf("node evidence = %#v, want only real evidence", got)
	}
	if got := chain.Links[0].EvidenceIDs; len(got) != 1 || got[0] != "evidence-1" {
		t.Fatalf("link evidence = %#v, want only real evidence", got)
	}
}

func TestValidateCausalDraftRejectsUnsupportedClaimWithoutEvidence(t *testing.T) {
	_, ok := validateCausalDraft("session-1", "run-1", causalChainDraft{
		Summary: "未核验结论",
		Nodes:   []CausalNode{{ID: "n1", Kind: "cause", Statement: "没有来源却声称确认", Status: "confirmed"}},
	}, nil)
	if ok {
		t.Fatal("validateCausalDraft() accepted a confirmed node without Evidence")
	}
}
