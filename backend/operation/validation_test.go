package operation

import (
	"testing"
	"time"
)

func TestApprovalCannotBeReplayedForChangedParameters(t *testing.T) {
	hash, _ := ParametersHash(map[string]any{"replicas": 2})
	expires := time.Now().Add(time.Minute)
	r := Request{Status: Approved, RequestedBy: "requester", ParametersHash: hash, ExpiresAt: &expires}
	if err := CanExecute(r, Approval{ApproverUserID: "approver", Decision: "approved", ParametersHash: hash}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := CanExecute(r, Approval{ApproverUserID: "approver", Decision: "approved", ParametersHash: "changed"}, time.Now()); err == nil {
		t.Fatal("changed parameters accepted")
	}
	if err := CanExecute(r, Approval{ApproverUserID: "requester", Decision: "approved", ParametersHash: hash}, time.Now()); err == nil {
		t.Fatal("self approval accepted")
	}
}
