package sandbox

import "testing"

func TestJobUsesRestrictedSecurityDefaults(t *testing.T) {
	job, err := BuildJob(JobSpec{Name: "x", Namespace: "n", Image: "image", Command: []string{"run"}})
	if err != nil {
		t.Fatal(err)
	}
	spec := job["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)
	if spec["automountServiceAccountToken"] != false {
		t.Fatal("service account token mounted")
	}
	container := spec["containers"].([]any)[0].(map[string]any)
	security := container["securityContext"].(map[string]any)
	if security["privileged"] != false || security["readOnlyRootFilesystem"] != true {
		t.Fatal("sandbox is not restricted")
	}
}
