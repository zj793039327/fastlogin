package tui

import tea "github.com/charmbracelet/bubbletea"

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "q":
			if m.searching {
				m.search += "q"
				m.cursor = 0
			} else {
				return m, tea.Quit
			}

		case "esc":
			m.searching = false
			m.search = ""
			m.cursor = 0

		case "/":
			m.searching = true
			m.search = ""
			m.cursor = 0

		case "enter":
			if e := m.selectedEntry(); e != nil {
				m.selected = e
				return m, tea.Quit
			}

		case "up", "k":
			if m.searching {
				if msg.String() == "k" {
					m.search += "k"
					m.cursor = 0
				}
				break
			}
			m.moveCursor(-1)

		case "down", "j":
			if m.searching {
				if msg.String() == "j" {
					m.search += "j"
					m.cursor = 0
				}
				break
			}
			m.moveCursor(1)

		case "right", "left", "tab", "h", "l":
			if m.searching {
				if ch := msg.String(); ch == "h" || ch == "l" {
					m.search += ch
					m.cursor = 0
				}
				break
			}
			if r := m.currentRow(); r != nil && r.Kind == RowGroup {
				m.expanded[r.GroupIdx] = !m.expanded[r.GroupIdx]
			}

		case "backspace":
			if m.searching && len(m.search) > 0 {
				m.search = m.search[:len(m.search)-1]
				m.cursor = 0
			}

		default:
			if m.searching {
				ch := msg.String()
				if len(ch) == 1 && ch >= " " {
					m.search += ch
					m.cursor = 0
				}
			}
		}
	}
	return m, nil
}
