package spanreportexporter

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type groupingKey struct {
	service string
	env     string
}

type spanStats struct {
	hourly      atomic.Uint64
	daily       atomic.Uint64
	monthly     atomic.Uint64
	httpHourly  atomic.Uint64
	sqlHourly   atomic.Uint64
	httpDaily   atomic.Uint64
	sqlDaily    atomic.Uint64
	httpMonthly atomic.Uint64
	sqlMonthly  atomic.Uint64
}

type spanReportExporter struct {
	path           string
	verbose        bool
	reportInterval time.Duration
	logger         *zap.Logger
	statsMap       sync.Map // map[groupingKey]*spanStats
	stopCh         chan struct{}
	lastExportTime time.Time
	tui            bool
}

func (e *spanReportExporter) ConsumeTraces(_ context.Context, td ptrace.Traces) error {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		attrs := rs.Resource().Attributes()

		// Extract attributes
		sName := "unknown"
		if s, ok := attrs.Get("service.name"); ok {
			sName = s.AsString()
		}
		eName := "unknown"
		if e, ok := attrs.Get("deployment.environment.name"); ok {
			eName = e.AsString()
		}
		key := groupingKey{
			service: sName,
			env:     eName,
		}
		if key.service == "" {
			key.service = "unknown"
		}
		if key.env == "" {
			key.env = "unknown"
		}

		// Retrieve or initialize the statistics object
		val, _ := e.statsMap.LoadOrStore(key, &spanStats{})
		stats := val.(*spanStats)

		ilss := rs.ScopeSpans()
		for j := 0; j < ilss.Len(); j++ {
			spans := ilss.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)

				// Basic total count
				stats.hourly.Add(1)
				stats.daily.Add(1)
				stats.monthly.Add(1)

				// Categorize by Kind and Attributes
				attrs := span.Attributes()

				// Check for HTTP Request (SERVER kind + http.route attribute)
				if span.Kind() == ptrace.SpanKindServer {
					_, hasHttpTarget := attrs.Get("http.target")
					_, hasHttpRoute := attrs.Get("http.route")
					if hasHttpTarget || hasHttpRoute {
						stats.httpHourly.Add(1)
						stats.httpDaily.Add(1)
						stats.httpMonthly.Add(1)
					}
				}

				// Check for SQL Query (db.statement attribute)
				_, hasDbStatement := attrs.Get("db.statement")
				_, hasDbQueryText := attrs.Get("db.query.text")
				if hasDbStatement || hasDbQueryText {
					stats.sqlHourly.Add(1)
					stats.sqlDaily.Add(1)
					stats.sqlMonthly.Add(1)
				}
			}
		}

		count := uint64(td.SpanCount())
		if e.verbose {
			e.logger.Info("Processed spans",
				zap.String("service", key.service),
				zap.String("environment", key.env),
				zap.Uint64("span_count", count),
			)
		}
	}
	return nil
}

func (e *spanReportExporter) startReporting() {
	go func() {
		ticker := time.NewTicker(e.reportInterval)
		defer ticker.Stop()
		for {
			now := time.Now()
			var next time.Time
			if e.reportInterval >= time.Hour {
				// For intervals of 1 hour or more, synchronize with the next "00:00" mark
				next = now.Truncate(time.Hour).Add(time.Hour)
			} else {
				// For intervals less than 1 hour (e.g., for testing), simply wait for the duration
				next = now.Add(e.reportInterval)
			}

			timer := time.NewTimer(time.Until(next))
			select {
			case <-timer.C:
				e.rotateAndWrite(time.Now())
			case <-e.stopCh:
				timer.Stop()
				return
			}
		}
	}()
}

func (e *spanReportExporter) rotateAndWrite(now time.Time) {
	// 1. Calculate and update stats (Logic part)
	lines := e.generateReportLines(now)
	if len(lines) == 0 {
		return
	}

	// 2. File I/O (Side effect part)
	f, err := os.OpenFile(e.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		e.logger.Error("Failed to open report file", zap.Error(err))
		return
	}
	defer f.Close()

	for _, line := range lines {
		f.WriteString(line)
	}

	// Update the last export time
	e.lastExportTime = now
}

// generateReportLines updates internal counters and returns formatted strings for the report.
// This method is now easy to test without creating files.
func (e *spanReportExporter) generateReportLines(now time.Time) []string {
	var lines []string
	displayTime := now.Add(-1 * time.Second).Format("2006-01-02 15:04:05")

	// Pre-calculate boundary flags to avoid checking them inside the loop
	isNewDay := !e.lastExportTime.IsZero() && now.Day() != e.lastExportTime.Day()
	isNewMonth := !e.lastExportTime.IsZero() && now.Month() != e.lastExportTime.Month()

	e.statsMap.Range(func(keyAny, valAny any) bool {
		k := keyAny.(groupingKey)
		s := valAny.(*spanStats)

		// Reset Hourly counters (always)
		h := s.hourly.Swap(0)
		hHTTP := s.httpHourly.Swap(0)
		hSQL := s.sqlHourly.Swap(0)

		// Conditional reset for Daily/Monthly
		if isNewDay {
			s.daily.Store(0)
			s.httpDaily.Store(0)
			s.sqlDaily.Store(0)
		}
		if isNewMonth {
			s.monthly.Store(0)
			s.httpMonthly.Store(0)
			s.sqlMonthly.Store(0)
		}

		// Load current values
		d, m := s.daily.Load(), s.monthly.Load()
		dHTTP, mHTTP := s.httpDaily.Load(), s.httpMonthly.Load()
		dSQL, mSQL := s.sqlDaily.Load(), s.sqlMonthly.Load()

		line := fmt.Sprintf("[%s] env:%s, service:%s | "+
			"Hourly(Total:%d, HTTP:%d, SQL:%d) | "+
			"Daily(Total:%d, HTTP:%d, SQL:%d) | "+
			"Monthly(Total:%d, HTTP:%d, SQL:%d)\n",
			displayTime, k.env, k.service,
			h, hHTTP, hSQL,
			d, dHTTP, dSQL,
			m, mHTTP, mSQL,
		)
		lines = append(lines, line)
		return true
	})

	return lines
}

func (e *spanReportExporter) Start(_ context.Context, _ component.Host) error {
	if e.tui {
		go func() {
			p := tea.NewProgram(NewTUIModel(e), tea.WithAltScreen())
			if _, err := p.Run(); err != nil {
				e.logger.Error("Failed to start TUI: %v", zap.Error(err))
			}
		}()
	}
	e.startReporting()
	return nil
}

func (e *spanReportExporter) Shutdown(_ context.Context) error {
	close(e.stopCh)
	e.rotateAndWrite(time.Now())
	e.logger.Info("SHUTDOWN")
	return nil
}
