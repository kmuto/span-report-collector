package spanreportexporter

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type groupingKey struct {
	service string
	env     string
}

type spanStats struct {
	hourly  atomic.Uint64
	daily   atomic.Uint64
	monthly atomic.Uint64
}

type reportExporter struct {
	path     string
	verbose  bool
	logger   *zap.Logger
	statsMap sync.Map // map[groupingKey]*spanStats
	stopCh   chan struct{}
}

func (e *reportExporter) ConsumeTraces(_ context.Context, td ptrace.Traces) error {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		attrs := rs.Resource().Attributes()

		// 属性の抽出
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

		// 統計オブジェクトの取得または生成
		val, _ := e.statsMap.LoadOrStore(key, &spanStats{})
		stats := val.(*spanStats)

		// カウントアップ
		count := uint64(td.SpanCount())
		if e.verbose {
			e.logger.Info("Processed spans",
				zap.String("service", key.service),
				zap.String("environment", key.env),
				zap.Uint64("span_count", count),
			)
		}
		stats.hourly.Add(count)
		stats.daily.Add(count)
		stats.monthly.Add(count)
	}
	return nil
}

func (e *reportExporter) startReporting() {
	go func() {
		for {
			now := time.Now()
			// 次の「0分0秒」までの待ち時間を計算
			nextHour := now.Truncate(time.Hour).Add(time.Hour)
			timer := time.NewTimer(time.Until(nextHour))

			select {
			case <-timer.C:
				e.rotateAndWrite(now.Format("2006-01-02 15:00:00"))
			case <-e.stopCh:
				timer.Stop()
				return
			}
		}
	}()
}

func (e *reportExporter) rotateAndWrite(timestamp string) {
	f, err := os.OpenFile(e.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		e.logger.Error("Failed to open report file", zap.Error(err))
		return
	}
	defer f.Close()

	now := time.Now()

	e.statsMap.Range(func(keyAny, valAny any) bool {
		k := keyAny.(groupingKey)
		s := valAny.(*spanStats)

		// 現在の値を読み取り
		h := s.hourly.Swap(0) // hourlyは常にリセット
		d := s.daily.Load()
		m := s.monthly.Load()

		// 日次・月次のリセット判定
		if now.Hour() == 0 {
			d = s.daily.Swap(0)
		}
		if now.Day() == 1 && now.Hour() == 0 {
			m = s.monthly.Swap(0)
		}

		line := fmt.Sprintf("[%s] env:%s, service:%s | hourly:%d, daily:%d, monthly:%d\n",
			timestamp, k.env, k.service, h, d, m)
		f.WriteString(line)
		return true
	})
}

func (e *reportExporter) Start(_ context.Context, _ component.Host) error {
	e.startReporting()
	return nil
}

func (e *reportExporter) Shutdown(_ context.Context) error {
	close(e.stopCh)
	e.rotateAndWrite("SHUTDOWN")
	return nil
}
