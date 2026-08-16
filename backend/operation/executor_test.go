package operation

import "testing"

func TestBuildSandboxJobDoesNotAcceptArbitraryCommands(t *testing.T) {
	request := Request{OperationName: RestartWorkload, Status: Executing}
	job, err := BuildSandboxJob(request, "01234567-89ab-cdef", "opskeeper", "opskeeper/runner")
	if err != nil {
		t.Fatal(err)
	}
	container := job["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["containers"].([]any)[0].(map[string]any)
	command := container["command"].([]string)
	if len(command) != 3 || command[0] != "/opskeeper-operation-runner" {
		t.Fatalf("command = %#v", command)
	}
}

func TestBuildSandboxJobRejectsUnsupportedOperation(t *testing.T) {
	_, err := BuildSandboxJob(Request{OperationName: "shell.exec", Status: Executing}, "01234567-89ab-cdef", "opskeeper", "image")
	if err == nil {
		t.Fatal("arbitrary operation was accepted")
	}
}
