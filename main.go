package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type Proc struct {
	PID  int
	Name string
	Addr string
}
type Column struct {
	Title string
	Width int
}
type Row []string

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

func main() {
	p := tea.NewProgram(initModel())
	if _, err := p.Run(); err != nil {
		panic(err)
	}
}

func listProcs() []Proc {
	output, _ := loadProcs()
	result := string(output)
	lines := strings.Split(result, "\n")

	var procs []Proc

	for _, line := range lines {
		fields := strings.Fields(line)

		if len(fields) == 0 {
			continue
		}

		pid, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		// for i, proc := range fields {
		// 	fmt.Printf("%d -> %s\n", i, proc)
		// }
		//
		proc := Proc{
			PID:  pid,
			Name: string(fields[0]),
			Addr: fields[8],
		}
		procs = append(procs, proc)

	}

	return procs
}

func loadProcs() ([]byte, error) {
	commandLsf := `lsof -nP -iTCP -sTCP:LISTEN | grep -E '127\.0\.0\.1|\[::1\]|\*:'`

	cmd := exec.Command("bash", "-c", commandLsf)

	output, err := cmd.Output()

	if err != nil {
		fmt.Println("Error outputing result", err)
		return nil, err
	}

	return output, nil
}

func initModel() model {
	procs := listProcs()

	columns := []table.Column{
		{Title: "PID", Width: 8},
		{Title: "NAME", Width: 20},
		{Title: "ADDR", Width: 24},
	}

	rows := make([]table.Row, 0, len(procs))
	for _, p := range procs {
		rows = append(rows, table.Row{
			strconv.Itoa(p.PID),
			p.Name,
			p.Addr,
		})
	}

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
		rows := make([]table.Row, 0, len(procs))
		for _, p := range procs {
			rows = append(rows, table.Row{
				strconv.Itoa(p.PID),
				p.Name,
				p.Addr,
			})
		}
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
				return m, killProc(m.targetPID)
			default:
				m.confirming = false
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
				m.searching = false
				m.search = textinput.New()
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
		footer = fmt.Sprintf("\n Kill %s (PID %d)? [y] yes", m.targetName, m.targetPID)
	case m.searching:
		footer = fmt.Sprintf("Search for: /%s", m.search.Value()+" ")
	}

	v := tea.NewView(title + "\n\n" + body + "\n\n" + footer)
	v.AltScreen = true

	return v
}

func log() (*slog.Logger, *os.File, error) {
	file, err := os.OpenFile(
		"logger.log",
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0644,
	)

	if err != nil {
		return nil, nil, err
	}

	logger := slog.New(slog.NewTextHandler(file, nil))

	return logger, file, nil
}

func killProc(pid int) tea.Cmd {
	return func() tea.Msg {
		_ = exec.Command("kill", strconv.Itoa(pid)).Run()
		return procsReloadedMsg{}
	}
}

func filterRows(rows []table.Row, query string) []table.Row {
	q := strings.ToLower(strings.TrimSpace(query))

	if q == "" {
		return rows
	}

	filtered := make([]table.Row, 0, len(rows))

	for _, r := range rows {
		for _, cell := range r {
			value := strings.ToLower(cell)
			if strings.Contains(value, q) {
				filtered = append(filtered, r)
				break
			}
		}
	}

	return filtered
}
