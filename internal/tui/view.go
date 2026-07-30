package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	groupStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("213"))
	cursorStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("fastlogin"))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", max(m.width, 50)))
	b.WriteString("\n")

	rows := m.rows()
	if len(rows) == 0 {
		b.WriteString(dimStyle.Render("  (no matching entries)"))
		b.WriteString("\n")
	} else {
		for i, r := range rows {
			line := m.renderRow(r)
			marker := "  "
			if i == m.cursor && !m.searching {
				line = cursorStyle.Render(line)
				marker = "▶ "
			}
			b.WriteString(marker + line + "\n")
		}
	}

	b.WriteString(strings.Repeat("─", max(m.width, 50)))
	b.WriteString("\n")
	if m.searching {
		b.WriteString("search: " + m.search)
	} else {
		b.WriteString(dimStyle.Render("↑↓/kj navigate · →←/hl fold · ⏎ connect · / search · q quit"))
	}
	if m.err != "" {
		b.WriteString("\n" + errStyle.Render(m.err))
	}
	return b.String()
}

func (m Model) renderRow(r row) string {
	if r.Kind == RowGroup {
		g := m.cfg.Groups[r.GroupIdx]
		marker := "▼"
		if !m.expanded[r.GroupIdx] {
			marker = "▶"
		}
		return groupStyle.Render(fmt.Sprintf("%s %s", marker, g.Name)) +
			dimStyle.Render(fmt.Sprintf("  (%d)", len(g.Entries)))
	}
	g := m.cfg.Groups[r.GroupIdx]
	e := g.Entries[r.EntryIdx]
	indent := "    "
	if g.Name == "" {
		indent = ""
	}
	return fmt.Sprintf("%s%-14s %s@%s [%s]", indent, e.Name, e.User, e.Host, e.Type)
}
