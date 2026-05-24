package tui

import (
	"testing"

	"github.com/jingle2008/toolkit/internal/columns"
	"github.com/jingle2008/toolkit/pkg/models"
)

func TestTuiRowsFlat_BaseModel(t *testing.T) {
	t.Parallel()
	m := []models.BaseModel{{Name: "cohere.command", InternalName: "command-r"}}
	rows := tuiRowsFlat(columns.BaseModelColumns, m, "", false)
	if len(rows) != 1 {
		t.Fatalf("rows: got %d, want 1", len(rows))
	}
	if rows[0][0] != "cohere.command" {
		t.Errorf("first cell: got %q, want cohere.command", rows[0][0])
	}
}

func TestTuiRowsGrouped_GpuNode(t *testing.T) {
	t.Parallel()
	m := map[string][]models.GpuNode{
		"pool-A": {{Name: "node-1", InstanceType: "BM.GPU4.8", Allocatable: 8, Allocated: 1, IsReady: true, Age: "1d"}},
	}
	rows := tuiRowsGrouped(columns.GpuNodeColumns, m, 0, nil, "", false)
	if len(rows) != 1 {
		t.Fatalf("rows: got %d, want 1", len(rows))
	}
	if rows[0][0] != "node-1" || rows[0][1] != "pool-A" {
		t.Errorf("name/pool: got %v", rows[0])
	}
}
