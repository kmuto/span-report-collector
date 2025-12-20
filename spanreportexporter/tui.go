package spanreportexporter

import (
	"fmt"
	"sort"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// 1秒ごとに画面を更新するためのメッセージ
type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Every(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type model struct {
	exporter *spanReportExporter
	table    table.Model
}

func NewTUIModel(e *spanReportExporter) model {
	columns := []table.Column{
		{Title: "Service", Width: 12},
		{Title: "Env", Width: 7},
		{Title: "Hourly (T/H/S)", Width: 18},
		{Title: "Daily (T/H/S)", Width: 18},
		{Title: "Monthly (T/H/S)", Width: 18},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	// スタイルの設定
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	return model{exporter: e, table: t}
}

func (m model) Init() tea.Cmd {
	return tick()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case tickMsg:
		m.table.SetRows(m.generateRows())
		return m, tick()
	}
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m model) generateRows() []table.Row {
	var rows []table.Row

	type entry struct {
		key   groupingKey
		stats *spanStats
	}

	var entries []entry

	m.exporter.statsMap.Range(func(key, value interface{}) bool {
		entries = append(entries, entry{
			key:   key.(groupingKey),
			stats: value.(*spanStats),
		})
		return true
	})

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].key.service != entries[j].key.service {
			return entries[i].key.service < entries[j].key.service
		}
		return entries[i].key.env < entries[j].key.env
	})

	for _, e := range entries {
		s := e.stats
		rows = append(rows, table.Row{
			e.key.service,
			e.key.env,
			// Hourly: Total / HTTP / SQL
			fmt.Sprintf("%d / %d / %d",
				s.hourly.Load(), s.httpHourly.Load(), s.sqlHourly.Load()),
			// Daily: Total / HTTP / SQL
			fmt.Sprintf("%d / %d / %d",
				s.daily.Load(), s.httpDaily.Load(), s.sqlDaily.Load()),
			// Monthly: Total / HTTP / SQL
			fmt.Sprintf("%d / %d / %d",
				s.monthly.Load(), s.httpMonthly.Load(), s.sqlMonthly.Load()),
		})
	}
	return rows
}

func (m model) View() string {
	baseStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(1, 2)

	return baseStyle.Render(
		fmt.Sprintf("Span Report Monitor | %s\n\n%s\n\nPress 'q' to quit",
			time.Now().Format("15:04:05"),
			m.table.View(),
		),
	)
}
