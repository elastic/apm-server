package elasticapm_test

import (
	"regexp"
	"testing"

	"github.com/elastic/apm-agent-go"
)

func TestNewUUID(t *testing.T) {
	id, err := elasticapm.NewUUID()
	if err != nil {
		t.Fatalf("NewUUID failed: %v", err)
	}
	matched, err := regexp.MatchString("^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$", id)
	if err != nil {
		t.Fatalf("regexp.MatchString failed: %v", err)
	}
	if !matched {
		t.Fatalf("ID %q does not match UUID regexp from JSON Schema", id)
	}
}
