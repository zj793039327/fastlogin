package tui

import (
	"testing"

	"fastlogin/internal/config"
)

func sampleConfig() *config.Config {
	return &config.Config{
		Groups: []config.Group{
			{Name: "prod", Entries: []config.Entry{
				{Name: "web", User: "root", Host: "10.0.0.1", Port: 22, Type: "ssh", Tags: []string{"web"}},
				{Name: "db", User: "admin", Host: "10.0.0.2", Port: 22, Type: "ssh"},
			}},
			{Name: "", Entries: []config.Entry{
				{Name: "loose", User: "u", Host: "1.2.3.4", Port: 22, Type: "ssh"},
			}},
		},
	}
}

func TestRowsExpanded(t *testing.T) {
	m := New(sampleConfig())
	rows := m.rows()
	// prod-header, web, db, loose = 4
	if len(rows) != 4 {
		t.Fatalf("rows = %d, %+v", len(rows), rows)
	}
	if rows[0].Kind != RowGroup || rows[0].GroupIdx != 0 {
		t.Errorf("rows[0] = %+v", rows[0])
	}
	if rows[3].EntryIdx != 0 || rows[3].GroupIdx != 1 {
		t.Errorf("loose row = %+v", rows[3])
	}
}

func TestRowsCollapsedHidesEntries(t *testing.T) {
	m := New(sampleConfig())
	m.expanded[0] = false
	rows := m.rows()
	// prod-header only, loose-header-less + loose entry = 2
	if len(rows) != 2 {
		t.Fatalf("rows = %d, %+v", len(rows), rows)
	}
	if rows[0].Kind != RowGroup {
		t.Errorf("rows[0] should be group header")
	}
}

func TestRowsSearchFilter(t *testing.T) {
	m := New(sampleConfig())
	m.searching = true
	m.search = "web"
	rows := m.rows()
	// prod-header + web (db hidden, empty-name group hidden because no match)
	if len(rows) != 2 {
		t.Fatalf("rows = %d, %+v", len(rows), rows)
	}
	if rows[1].EntryIdx != 0 {
		t.Errorf("searched entry row = %+v", rows[1])
	}
}

func TestSearchHidesEmptyNamedGroups(t *testing.T) {
	m := New(sampleConfig())
	m.searching = true
	m.search = "nomatch"
	rows := m.rows()
	if len(rows) != 0 {
		t.Fatalf("expected no rows, got %+v", rows)
	}
}

func TestMoveCursorWraps(t *testing.T) {
	m := New(sampleConfig())
	m.moveCursor(1)
	if m.cursor != 1 {
		t.Errorf("cursor = %d", m.cursor)
	}
	m.cursor = 3
	m.moveCursor(1)
	if m.cursor != 0 {
		t.Errorf("wrap to top failed, cursor = %d", m.cursor)
	}
	m.moveCursor(-1)
	if m.cursor != 3 {
		t.Errorf("wrap to bottom failed, cursor = %d", m.cursor)
	}
}

func TestSelectedEntry(t *testing.T) {
	m := New(sampleConfig())
	m.cursor = 1 // web
	e := m.selectedEntry()
	if e == nil || e.Name != "web" {
		t.Errorf("selected = %+v", e)
	}
	m.cursor = 0 // group header
	if m.selectedEntry() != nil {
		t.Error("group header should not select an entry")
	}
}
