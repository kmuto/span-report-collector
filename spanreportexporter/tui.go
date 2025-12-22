package spanreportexporter

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

// 1秒ごとに画面を更新するためのメッセージ
type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Every(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type model struct {
	exporter  *spanReportExporter
	table     table.Model
	startTime time.Time
}

func NewTUIModel(e *spanReportExporter) model {
	return model{
		exporter:  e,
		startTime: time.Now(),
	}
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
	var b strings.Builder

	// 1. ヘッダー情報の表示
	uptime := time.Since(m.startTime).Round(time.Second)
	b.WriteString(" [Span Report Monitor]\n")
	b.WriteString(fmt.Sprintf(" Current Time: %s | Uptime: %s\n",
		time.Now().Format("15:04:05"), uptime))
	b.WriteString(" Legend: (T:Total / H:HTTP / S:SQL)\n\n")

	header := fmt.Sprintf("%-12s %-7s %-18s %-18s %-18s\n", "SERVICE", "ENV", "HOURLY(T/H/S)", "DAILY(T/H/S)", "MONTHLY(T/H/S)")
	b.WriteString(header)
	b.WriteString(strings.Repeat("-", len(header)) + "\n")

	// データの描画
	rows := m.generateRows()
	for _, row := range rows {
		// row は []string {service, env, hourly, daily, monthly}
		line := fmt.Sprintf("%-12s %-7s %-18s %-18s %-18s\n",
			truncate(row[0], 12),
			truncate(row[1], 7),
			row[2], row[3], row[4])
		b.WriteString(line)
	}

	b.WriteString("\nPress 'q' to quit")
	return b.String()
}

// 幅を超えた場合に切り詰める補助関数
func truncate(s string, w int) string {
	if len(s) > w {
		return s[:w-1] + "…"
	}
	return s
}
