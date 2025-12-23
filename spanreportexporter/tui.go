package spanreportexporter

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

// Message to refresh the screen every second
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

func humanize(v int64) string {
	if v < 10000 {
		return strconv.FormatInt(v, 10)
	}

	suffixes := []string{"k", "M", "G", "T", "P", "E"}
	f := float64(v)
	unit := 1000.0

	for _, suffix := range suffixes {
		f /= unit
		if f < unit {
			// Up to one decimal place. e.g., 1.0k, 999.9k
			return fmt.Sprintf("%.1f%s", f, suffix)
		}
	}
	return fmt.Sprintf("%.1fE", f)
}

func (m model) View() string {
	var b strings.Builder

	// Header information
	uptime := time.Since(m.startTime).Round(time.Second)
	b.WriteString(fmt.Sprintf(" [Span Report Monitor]  Time: %s | Uptime: %s\n",
		time.Now().Format("15:04:05"), uptime))
	b.WriteString(" Legend: T=Total, H=HTTP, S=SQL\n\n")

	// Header row (with clear separators)
	// Total width is about 85 characters, fitting within a typical terminal width of 80-100 characters.
	header := fmt.Sprintf("%-12s %-7s | %-17s | %-17s | %-18s\n",
		"SERVICE", "ENV", "  HOURLY (T/H/S)", "  DAILY (T/H/S)", "  MONTHLY (T/H/S)")
	b.WriteString(header)
	b.WriteString(strings.Repeat("-", 12) + "-" + strings.Repeat("-", 8) + "+" +
		strings.Repeat("-", 19) + "+" + strings.Repeat("-", 19) + "+" +
		strings.Repeat("-", 20) + "\n")

	// Render data
	entries := m.exporter.getSortedEntries() // Sorted entries
	for _, e := range entries {
		s := e.stats

		// Function to format a group of three numbers for one period
		fmtGroup := func(t, h, s uint64) string {
			return fmt.Sprintf("%5s %5s %5s", humanize(int64(t)), humanize(int64(h)), humanize(int64(s)))
		}

		line := fmt.Sprintf("%-12s %-7s | %s | %s | %s\n",
			truncate(e.key.service, 12),
			truncate(e.key.env, 7),
			fmtGroup(s.hourly.Load(), s.httpHourly.Load(), s.sqlHourly.Load()),
			fmtGroup(s.daily.Load(), s.httpDaily.Load(), s.sqlDaily.Load()),
			fmtGroup(s.monthly.Load(), s.httpMonthly.Load(), s.sqlMonthly.Load()),
		)
		b.WriteString(line)
	}

	b.WriteString("\n (Press 'q' or 'Ctrl+C' to exit)")
	return b.String()
}

func (m model) View_old() string {
	var b strings.Builder

	// 1. Display header information
	uptime := time.Since(m.startTime).Round(time.Second)
	b.WriteString("[Span Report Monitor]\n")
	b.WriteString(fmt.Sprintf("Current Time: %s | Uptime: %s\n",
		time.Now().Format("15:04:05"), uptime))
	b.WriteString("Legend: (T:Total / H:HTTP / S:SQL)\n\n")

	header := fmt.Sprintf("%-12s %-7s %-18s %-18s %-18s\n", "SERVICE", "ENV", "HOURLY(T/H/S)", "DAILY(T/H/S)", "MONTHLY(T/H/S)")
	b.WriteString(header)
	b.WriteString(strings.Repeat("-", len(header)) + "\n")

	// Render data
	rows := m.generateRows()
	for _, row := range rows {
		// row is []string {service, env, hourly, daily, monthly}
		line := fmt.Sprintf("%-12s %-7s %-18s %-18s %-18s\n",
			truncate(row[0], 12),
			truncate(row[1], 7),
			row[2], row[3], row[4])
		b.WriteString(line)
	}

	b.WriteString("\nPress 'q' to quit")
	return b.String()
}

// Helper function to truncate a string if it exceeds the given width
func truncate(s string, w int) string {
	if len(s) > w {
		return s[:w-1] + "…"
	}
	return s
}
