package oceanbase

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestModeAndSchema(t *testing.T) {
	if got, err := mode(ConnectionInput{}); err != nil || got != ModeMySQL {
		t.Fatalf("default mode = %q, %v", got, err)
	}
	if _, err := mode(ConnectionInput{Mode: "other"}); err == nil {
		t.Fatal("unsupported mode must fail")
	}
	if got := len(ListTools()); got != 7 {
		t.Fatalf("tool count = %d, want 7", got)
	}
	if InputSchema(nil)["additionalProperties"] != false {
		t.Fatal("schema must reject undeclared fields")
	}
}

func TestProbeMySQLWire(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = conn.Write([]byte{3, 0, 0, 0, 10, '8', 0})
	}()
	wired, reachable, err := probeMySQLWire(context.Background(), listener.Addr().String(), time.Second)
	if err != nil || !reachable || !wired {
		t.Fatalf("probe = wire:%v reachable:%v err:%v, want MySQL Wire", wired, reachable, err)
	}
}
