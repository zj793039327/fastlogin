package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"fastlogin/internal/config"
)

// RowKind distinguishes group-header rows from entry rows in the flattened list.
type RowKind int

const (
	RowGroup RowKind = iota
	RowEntry
)

type row struct {
	Kind     RowKind
	GroupIdx int
	EntryIdx int
}

// Model is the bubbletea model. It is a value type: Update returns a copy.
type Model struct {
	cfg       *config.Config
	cursor    int
	expanded  map[int]bool
	searching bool
	search    string
	width     int
	height    int
	selected  *config.Entry
	err       string
}

// New builds a Model with all groups expanded by default.
func New(cfg *config.Config) Model {
	expanded := make(map[int]bool, len(cfg.Groups))
	for i := range cfg.Groups {
		expanded[i] = true
	}
	return Model{cfg: cfg, expanded: expanded}
}

func (m Model) Init() tea.Cmd { return nil }

// Selected returns the entry chosen via Enter, or nil.
func (m Model) Selected() *config.Entry { return m.selected }

// rows flattens the visible structure honoring fold + search state.
func (m Model) rows() []row {
	var out []row
	q := strings.ToLower(m.search)
	searching := m.searching && m.search != ""
	for gi, g := range m.cfg.Groups {
		var idxs []int
		for ei, e := range g.Entries {
			if searching && !matches(e, q) {
				continue
			}
			idxs = append(idxs, ei)
		}
		// Named group: emit header; hide entirely if a search yields no hits.
		if g.Name != "" {
			if searching && len(idxs) == 0 {
				continue
			}
			out = append(out, row{Kind: RowGroup, GroupIdx: gi})
			if !m.expanded[gi] {
				continue
			}
		}
		for _, ei := range idxs {
			out = append(out, row{Kind: RowEntry, GroupIdx: gi, EntryIdx: ei})
		}
	}
	return out
}

func matches(e config.Entry, q string) bool {
	if strings.Contains(strings.ToLower(e.Name), q) {
		return true
	}
	if strings.Contains(strings.ToLower(e.Host), q) {
		return true
	}
	if strings.Contains(strings.ToLower(e.User), q) {
		return true
	}
	for _, t := range e.Tags {
		if strings.Contains(strings.ToLower(t), q) {
			return true
		}
	}
	return false
}

func (m Model) currentRow() *row {
	rows := m.rows()
	if m.cursor < 0 || m.cursor >= len(rows) {
		return nil
	}
	r := rows[m.cursor]
	return &r
}

func (m Model) selectedEntry() *config.Entry {
	r := m.currentRow()
	if r == nil || r.Kind != RowEntry {
		return nil
	}
	e := m.cfg.Groups[r.GroupIdx].Entries[r.EntryIdx]
	return &e
}

// moveCursor advances by delta, wrapping around the visible rows.
func (m *Model) moveCursor(delta int) {
	rows := m.rows()
	n := len(rows)
	if n == 0 {
		m.cursor = 0
		return
	}
	m.cursor = ((m.cursor+delta)%n + n) % n
}
