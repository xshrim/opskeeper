package oracle

import (
	"strings"
	"testing"
)

func TestCatalogAndInputSchemaAreFixed(t *testing.T) {
	if got := len(ListTools()); got != 7 {
		t.Fatalf("tool count = %d, want 7", got)
	}
	if InputSchema(nil)["additionalProperties"] != false {
		t.Fatal("connection schema must reject undeclared fields")
	}
}

func TestDSNRequiresOneOracleServiceSelector(t *testing.T) {
	base := ConnectionInput{Host: "db.example", Username: "ops", Password: "secret"}
	if _, err := dsn(base); err == nil {
		t.Fatal("missing service selector must fail")
	}
	if _, err := dsn(ConnectionInput{Host: base.Host, Username: base.Username, Password: base.Password, ServiceName: "svc", SID: "sid"}); err == nil {
		t.Fatal("service name and SID must be mutually exclusive")
	}
	got, err := dsn(ConnectionInput{Host: base.Host, Username: base.Username, Password: base.Password, SID: "ORCL", TLS: true})
	if err != nil {
		t.Fatalf("dsn(): %v", err)
	}
	if !strings.Contains(got, "SID=ORCL") || !strings.Contains(got, "AUTH TYPE=TCPS") {
		t.Fatalf("dsn = %q, want SID and TCPS options", got)
	}
}
