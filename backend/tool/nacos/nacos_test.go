package nacos

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestToolsAreFixedAndBounded(t *testing.T) {
	if len(ListTools()) != 6 {
		t.Fatalf("tool count = %d", len(ListTools()))
	}
	if InputSchema(nil)["additionalProperties"] != false {
		t.Fatal("schema permits arbitrary fields")
	}
	if page(0) != 1 || page(2000) != 1000 || pageSize(200) != 100 {
		t.Fatal("pagination bounds are wrong")
	}
}

func TestClientUsesReadOnlyNacosAPIs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"count":0,"doms":[]}`)
	}))
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")
	c, err := New(context.Background(), ConnectionInput{Host: strings.Split(host, ":")[0], Port: mustPort(host), ContextPath: ""})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Services(context.Background(), ServicesInput{}); err != nil {
		t.Fatal(err)
	}
}

func mustPort(host string) int {
	var p int
	fmt.Sscanf(host[strings.LastIndex(host, ":")+1:], "%d", &p)
	return p
}
