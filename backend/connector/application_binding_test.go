package connector

import (
	"reflect"
	"testing"
)

func TestBindingTermsRequireEveryCommaSeparatedTerm(t *testing.T) {
	terms := bindingTerms(" Java, payments.jar , ")
	if !reflect.DeepEqual(terms, []string{"java", "payments.jar"}) {
		t.Fatalf("bindingTerms() = %#v", terms)
	}
	if !containsAll("java -jar payments.jar", terms) {
		t.Fatal("all matching terms should match")
	}
	if containsAll("java -jar other.jar", terms) {
		t.Fatal("missing term should fail the match")
	}
}
