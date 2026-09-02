package server

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"opskeeper/backend/mcpserver/docker/client"
)

func TestToolsUseDockerEngineReadOnlyEndpoints(t *testing.T) {
	var logsQuery map[string]string
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/_ping", "/v1.51/_ping":
			writer.Header().Set("API-Version", "1.51")
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write([]byte("OK"))
		case "/v1.51/info":
			writeJSON(writer, map[string]any{"ID": "daemon-id", "Containers": 2, "Images": 3, "ServerVersion": "test"})
		case "/v1.51/images/json":
			writeJSON(writer, []map[string]any{{"Id": "sha256:image", "RepoTags": []string{"example:test"}}})
		case "/v1.51/containers/json":
			writeJSON(writer, []map[string]any{{"Id": "container-id", "Image": "example:test", "State": "running"}})
		case "/v1.51/containers/container-id/logs":
			logsQuery = map[string]string{"tail": request.URL.Query().Get("tail"), "since": request.URL.Query().Get("since"), "until": request.URL.Query().Get("until")}
			payload := []byte("INFO hello from docker\nERROR failed request\ninfo healthy\n")
			frame := make([]byte, 8+len(payload))
			frame[0] = 1
			binary.BigEndian.PutUint32(frame[4:8], uint32(len(payload)))
			copy(frame[8:], payload)
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write(frame)
		case "/v1.51/containers/container-id/json":
			writeJSON(writer, map[string]any{"Id": "container-id", "Config": map[string]any{"Env": []string{"PASSWORD=hidden", "SAFE=value"}}})
		case "/v1.51/containers/container-id/stats":
			writeJSON(writer, map[string]any{"id": "container-id", "name": "/demo", "memory_stats": map[string]any{"usage": 12}})
		default:
			http.NotFound(writer, request)
		}
	}))
	server.Listener = listener
	server.Start()
	defer server.Close()

	input := client.ConnectionInput{DockerHost: server.URL}
	if _, output, err := dockerInfo(context.Background(), nil, DockerInfoInput{ConnectionInput: input}); err != nil || output.Info.ID == "" {
		t.Fatalf("docker info: output=%v err=%v", output, err)
	}
	if _, output, err := listImages(context.Background(), nil, ListImagesInput{ConnectionInput: input}); err != nil || len(output.Images) != 1 {
		t.Fatalf("docker images: output=%v err=%v", output, err)
	}
	if _, output, err := listContainers(context.Background(), nil, ListContainersInput{ConnectionInput: input}); err != nil || len(output.Containers) != 1 {
		t.Fatalf("docker containers: output=%v err=%v", output, err)
	}
	if _, output, err := containerLogs(context.Background(), nil, ContainerLogsInput{ConnectionInput: input, ContainerName: "container-id"}); err != nil || output.Logs != "INFO hello from docker\nERROR failed request\ninfo healthy\n" {
		t.Fatalf("docker logs: output=%+v err=%v", output, err)
	}
	if logsQuery["tail"] != "1000" || logsQuery["since"] != "" || logsQuery["until"] != "" {
		t.Fatalf("unexpected default log query: %#v", logsQuery)
	}
	if _, output, err := containerLogs(context.Background(), nil, ContainerLogsInput{ConnectionInput: input, ContainerName: "container-id", Since: "2h", Until: "1h", Tail: "20", Keyword: "ERROR"}); err != nil || output.Logs != "INFO hello from docker\nERROR failed request\ninfo healthy\n" {
		t.Fatalf("filtered docker logs: output=%+v err=%v", output, err)
	}
	if logsQuery["tail"] != "20" || logsQuery["since"] == "" || logsQuery["until"] == "" {
		t.Fatalf("unexpected bounded log query: %#v", logsQuery)
	}
	if _, output, err := containerInspect(context.Background(), nil, ContainerInspectInput{ConnectionInput: input, ContainerName: "container-id"}); err != nil || output.ID != "container-id" || output.Config == nil || len(output.Config.EnvNames) != 2 || output.Config.EnvNames[0] != "PASSWORD" {
		t.Fatalf("docker inspect: output=%v err=%v", output, err)
	}
	if _, output, err := containerStats(context.Background(), nil, ContainerStatsInput{ConnectionInput: input, ContainerName: "container-id"}); err != nil || output.ID != "container-id" {
		t.Fatalf("docker stats: output=%+v err=%v", output, err)
	}
}

func TestToFiltersParsesKeyValueString(t *testing.T) {
	args, err := toFilters(" label:com.example.env=prod, reference:nginx:latest ")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := args.Get("label"), []string{"com.example.env=prod"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("label filter = %#v, want %#v", got, want)
	}
	if got, want := args.Get("reference"), []string{"nginx:latest"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("reference filter = %#v, want %#v", got, want)
	}
}

func TestToFiltersAcceptsEmptyString(t *testing.T) {
	args, err := toFilters("")
	if err != nil {
		t.Fatal(err)
	}
	if got := args.Get("label"); len(got) != 0 {
		t.Fatalf("empty filters produced values: %#v", got)
	}
}

func TestToFiltersRejectsMalformedItems(t *testing.T) {
	for _, raw := range []string{"label", ":value", "key:", "key:value,,other:value"} {
		t.Run(raw, func(t *testing.T) {
			if _, err := toFilters(raw); err == nil {
				t.Fatalf("toFilters(%q) unexpectedly succeeded", raw)
			}
		})
	}
}

func TestResolveContainerIdentifier(t *testing.T) {
	if got, err := resolveContainerIdentifier(" id ", "name"); err != nil || got != "id" {
		t.Fatalf("ID should take precedence: got %q, err=%v", got, err)
	}
	if got, err := resolveContainerIdentifier("", " name "); err != nil || got != "name" {
		t.Fatalf("name fallback failed: got %q, err=%v", got, err)
	}
	if _, err := resolveContainerIdentifier(" ", " "); err == nil {
		t.Fatal("expected missing identifier error")
	}
}

func TestFilterLogLines(t *testing.T) {
	logs := "INFO ready\nerror: failed\nwarning pending\n"
	if got := filterLogLines(logs, "ERROR"); got != logs {
		t.Fatalf("case-insensitive keyword filter = %q", got)
	}
	if got := filterLogLines(logs, " "); got != logs {
		t.Fatalf("empty keyword changed logs: %q", got)
	}
	if got := filterLogLines("one\ntwo\nthree\nfour\nfive\nsix\nseven\n", "four"); got != "one\ntwo\nthree\nfour\nfive\nsix\nseven\n" {
		t.Fatalf("keyword context filter did not include surrounding lines: %q", got)
	}
	if got := filterLogLines("0\n1\n2\nMATCH A\n4\n5\n6\n7\nMATCH B\n9\n10\n11\n", "match"); got != "0\n1\n2\nMATCH A\n4\n5\n6\n7\nMATCH B\n9\n10\n11\n" {
		t.Fatalf("overlapping keyword contexts were not merged: %q", got)
	}
}

func TestLimitLogLinesKeepsMostRecentLines(t *testing.T) {
	if got := limitLogLines("1\n2\n3\n4\n5\n", 3); got != "3\n4\n5\n" {
		t.Fatalf("limited logs = %q, want most recent lines", got)
	}
}

func TestKeywordFilteringHappensBeforeFinalLineLimit(t *testing.T) {
	logs := strings.Repeat("MATCH\n", 205)
	got := limitLogLines(filterLogLines(logs, "match"), maxOutputLogLines)
	if lines := strings.Count(got, "\n"); lines != maxOutputLogLines {
		t.Fatalf("final filtered log lines = %d, want %d", lines, maxOutputLogLines)
	}
	if !strings.HasPrefix(got, "MATCH\n") || got != logs[len(logs)-len(got):] {
		t.Fatal("final line limit did not retain the most recent filtered lines")
	}
}

func writeJSON(writer http.ResponseWriter, value any) {
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(value)
}
