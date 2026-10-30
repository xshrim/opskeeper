package connector

import "testing"

func TestFinalizeApplicationCandidatesUsesUniqueIntersection(t *testing.T) {
	makeMatch := func(name, resourceID string, count int) *candidateResourceMatch {
		instances := []ApplicationCandidateInstance{}
		if count > 0 {
			instances = append(instances, ApplicationCandidateInstance{
				Name: name, TargetResourceID: resourceID,
				Selector: map[string]any{"container_name": "orders"},
			})
		}
		return &candidateResourceMatch{
			candidate: ApplicationCandidate{
				Identity: "orders", RuntimeKind: "containerized", Name: "orders",
				Instances: instances,
			},
			count: count,
		}
	}

	byIdentity := map[string]map[string]*candidateResourceMatch{
		"orders": {
			"docker-a": makeMatch("Docker", "docker-a", 1),
			"docker-b": makeMatch("Docker", "docker-b", 1),
		},
		"duplicate": {
			"docker-a": makeMatch("Docker", "docker-a", 2),
			"docker-b": makeMatch("Docker", "docker-b", 1),
		},
		"missing": {
			"docker-a": makeMatch("Docker", "docker-a", 1),
		},
	}

	items := finalizeApplicationCandidates("containerized", []string{"docker-a", "docker-b"}, byIdentity)
	if len(items) != 1 || items[0].Identity != "orders" {
		t.Fatalf("candidates = %#v, want only the unique shared orders target", items)
	}
	if items[0].Replicas != 2 || len(items[0].Instances) != 2 {
		t.Fatalf("candidate instances = %#v, want one instance per runtime resource", items[0])
	}
	if items[0].Instances[0].Name != "Docker" || items[0].Instances[1].Name != "Docker-2" {
		t.Fatalf("instance names = %#v, want unique names", items[0].Instances)
	}
}

func TestFinalizeApplicationCandidatesSortsByNameAndIdentity(t *testing.T) {
	makeMatch := func(identity, name string) *candidateResourceMatch {
		return &candidateResourceMatch{
			candidate: ApplicationCandidate{
				Identity: identity, RuntimeKind: "containerized", Name: name,
				Instances: []ApplicationCandidateInstance{{Name: "Docker", TargetResourceID: "docker-a"}},
			},
			count: 1,
		}
	}
	byIdentity := map[string]map[string]*candidateResourceMatch{
		"z": {"docker-a": makeMatch("z", "zeta")},
		"b": {"docker-a": makeMatch("b", "same")},
		"a": {"docker-a": makeMatch("a", "same")},
	}
	items := finalizeApplicationCandidates("containerized", []string{"docker-a"}, byIdentity)
	if len(items) != 3 || items[0].Identity != "a" || items[1].Identity != "b" || items[2].Identity != "z" {
		t.Fatalf("candidate order = %#v", items)
	}
}

func TestUniqueIDsPreservesFirstOccurrence(t *testing.T) {
	got := uniqueIDs([]string{" docker-a ", "", "docker-b", "docker-a"})
	if len(got) != 2 || got[0] != "docker-a" || got[1] != "docker-b" {
		t.Fatalf("uniqueIDs() = %#v", got)
	}
}
