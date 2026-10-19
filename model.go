package main

import (
	"fmt"
	"strconv"

	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type procsReloadedMsg struct{}

type model struct {
	table      table.Model
	err        error
	confirming bool
	targetPID  int
	targetName string
	search     textinput.Model
	searching  bool
	rows       []table.Row
}

func initModel() model {
	procs := listProcs()

	columns := []table.Column{
		{Title: "PID", Width: 8},
		{Title: "NAME", Width: 20},
		{Title: "ADDR", Width: 24},
	}

	rows := procsToRows(procs)

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithHeight(15),
		table.WithFocused(true),
	)
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("27")). // blue
		Bold(false)
	t.SetStyles(s)

	search := textinput.New()

	search.Placeholder = "Search port / name / PID"
	search.Prompt = " / "
	search.SetVirtualCursor(true)
	search.CharLimit = 64

	return model{table: t, err: nil, search: search, rows: rows}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case procsReloadedMsg:
		procs := listProcs()
		rows := procsToRows(procs)
		m.rows = rows
		m.table.SetRows(filterRows(rows, m.search.Value()))
		m.table.SetCursor(0)

		return m, nil
	case tea.WindowSizeMsg:
		h := msg.Height - 10
		if h < 1 {
			h = 1
		}
		m.table.SetWidth(msg.Width)
		m.table.SetHeight(h)

	case tea.KeyPressMsg:
		if m.confirming {
			switch msg.String() {
			case "y", "enter":
				m.confirming = false
				m.table.Focus()
				return m, killProc(m.targetPID)
			case "up":
				m.confirming = false
				m.table.MoveUp(1)
				if m.searching == false {
					m.searching = true
					m.table.Blur()
					return m, m.search.Focus()
				}
				return m, nil
			case "down":
				m.confirming = false
				m.table.MoveDown(1)
				if m.searching == false {
					m.searching = true
					m.table.Blur()
					return m, m.search.Focus()
				}
				return m, nil
			default:
				m.confirming = false
				m.searching = false
				return m, nil
			}
		}

		if m.searching {
			switch msg.String() {
			case "esc":
				m.searching = false
				m.search.Reset()
				m.search.Blur()
				m.table.SetRows(m.rows)
				m.table.SetCursor(0)
				m.table.Focus()

				return m, nil
			case "enter":
				row := m.table.SelectedRow()
				if row == nil {
					return m, nil
				}

				pid, err := strconv.Atoi(row[0])
				if err != nil {
					return m, nil
				}

				m.confirming = true
				m.targetPID = pid
				m.targetName = row[1]

				return m, nil
			case "up":
				m.table.MoveUp(1)
				return m, nil
			case "down":
				m.table.MoveDown(1)
				return m, nil
			}

			var scmd tea.Cmd
			m.search, scmd = m.search.Update(msg)
			m.table.SetRows(filterRows(m.rows, m.search.Value()))
			m.table.SetCursor(0)

			return m, scmd
		}

		switch msg.String() {
		case "enter":
			row := m.table.SelectedRow()
			if row == nil {
				return m, nil
			}

			pid, err := strconv.Atoi(row[0])
			if err != nil {
				return m, nil
			}

			m.confirming = true
			m.targetPID = pid
			m.targetName = row[1]
			return m, nil
		case "/":
			m.searching = true
			m.table.Blur()
			return m, m.search.Focus()
		case "q", "esc":
			return m, tea.Quit
		case "r":
			procs := listProcs()
			rows := procsToRows(procs)
			m.rows = rows
			m.table.SetRows(filterRows(rows, m.search.Value()))

			return m, nil

		}
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m model) View() tea.View {
	if m.err != nil {
		v := tea.NewView("Gorbang Error, press q to quit")

		v.AltScreen = true
		return v
	}
	title := lipgloss.NewStyle().Bold(true).Render("Gorbang - Listening ports")
	body := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		Render(m.table.View())
	footer := ""

	switch {
	case m.confirming:
		footer = fmt.Sprintf("\n Kill %s (PID %d)? [y] Yes", m.targetName, m.targetPID)
	case m.searching:
		footer = fmt.Sprintf("Search: %s\nPress [ESC] to exit search mode", m.search.Value()+" ")
	default:
		footer = "[R] Refresh\n[/] Search process\n[Enter] kill procces\n[ESC, Q] Quit"
	}

	v := tea.NewView(title + "\n\n" + body + "\n\n" + footer)
	v.AltScreen = true

	return v
}
