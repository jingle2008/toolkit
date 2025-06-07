package rows

import (
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
)

func TestGetEnvironments(t *testing.T) {
	envs := []models.Environment{
		{Type: "k8s", Realm: "public", Region: "us-west"},
		{Type: "k8s", Realm: "private", Region: "us-east"},
		{Type: "baremetal", Realm: "public", Region: "eu-central"},
	}

	// No filter: all environments returned
	rows := Environments(envs, "")
	if len(rows) != 3 {
		t.Errorf("expected 3 rows, got %d", len(rows))
	}

	// Filter by type
	rows = Environments(envs, "baremetal")
	if len(rows) != 1 || rows[0].GetName() != "baremetal-UNKNOWN" {
		t.Errorf("expected 1 row for baremetal, got %v", rows)
	}

	// Filter by region
	rows = Environments(envs, "west")
	if len(rows) != 1 || rows[0].GetName() != "k8s-UNKNOWN" || rows[0].Region != "us-west" {
		t.Errorf("expected 1 row for us-west, got %v", rows)
	}

	// Filter by realm
	rows = Environments(envs, "private")
	if len(rows) != 1 || rows[0].Realm != "private" {
		t.Errorf("expected 1 row for private realm, got %v", rows)
	}

	// Filter with no match
	rows = Environments(envs, "doesnotexist")
	if len(rows) != 0 {
		t.Errorf("expected 0 rows for unmatched filter, got %v", rows)
	}
}
