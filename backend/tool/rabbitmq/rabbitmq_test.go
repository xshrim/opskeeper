package rabbitmq

import "testing"

func TestCatalogAndEndpointValidation(t *testing.T) {
	if len(ListTools()) != 9 {
		t.Fatalf("tool count=%d", len(ListTools()))
	}
	if _, e := endpoint(ConnectionInput{}); e == nil {
		t.Fatal("missing URL must fail")
	}
	if _, e := endpoint(ConnectionInput{URL: "amqp://rabbit"}); e == nil {
		t.Fatal("non HTTP URL must fail")
	}
	got, e := endpoint(ConnectionInput{URL: "http://rabbit:15672"})
	if e != nil || got != "http://rabbit:15672/api" {
		t.Fatalf("endpoint=%q err=%v", got, e)
	}
}
