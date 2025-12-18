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

type spanReportExporter struct {
	path           string
	verbose        bool
	reportInterval time.Duration
	logger         *zap.Logger
	statsMap       sync.Map // map[groupingKey]*spanStats
	stopCh         chan struct{}
	lastExportTime time.Time
}

func (e *spanReportExporter) ConsumeTraces(_ context.Context, td ptrace.Traces) error {
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

func (e *spanReportExporter) startReporting() {
	go func() {
		ticker := time.NewTicker(e.reportInterval)
		defer ticker.Stop()
		for {
			now := time.Now()
			var next time.Time
			if e.reportInterval >= time.Hour {
				// 1時間以上の場合は、次の「00分00秒」に同期
				next = now.Truncate(time.Hour).Add(time.Hour)
			} else {
				// 1時間未満（テスト用）の場合は、単純にインターバル分待つ
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
	f, err := os.OpenFile(e.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		e.logger.Error("Failed to open report file", zap.Error(err))
		return
	}
	defer f.Close()

	// レポート上の表記を調整
	// 例: 01月01日 00:00:00 に実行された場合、表示は "12月31日 23:59:59" 側にする
	displayTime := now.Add(-1 * time.Second).Format("2006-01-02 15:04:05")

	e.statsMap.Range(func(keyAny, valAny any) bool {
		k := keyAny.(groupingKey)
		s := valAny.(*spanStats)

		// 現在の値を読み取り
		h := s.hourly.Swap(0) // hourlyは常にリセット
		// 日付が変わっていたら Daily をリセット
		if now.Day() != e.lastExportTime.Day() && !e.lastExportTime.IsZero() {
			s.daily.Store(0)
		}
		// 月が変わっていたら Monthly をリセット
		if now.Month() != e.lastExportTime.Month() && !e.lastExportTime.IsZero() {
			s.monthly.Store(0)
		}

		d := s.daily.Load()
		m := s.monthly.Load()

		line := fmt.Sprintf("[%s] env:%s, service:%s | hourly:%d, daily:%d, monthly:%d\n",
			displayTime, k.env, k.service, h, d, m)
		f.WriteString(line)
		return true
	})

	// エクスポート時間を更新
	e.lastExportTime = time.Now()
}

func (e *spanReportExporter) Start(_ context.Context, _ component.Host) error {
	e.startReporting()
	return nil
}

func (e *spanReportExporter) Shutdown(_ context.Context) error {
	close(e.stopCh)
	e.rotateAndWrite(time.Now())
	e.logger.Info("SHUTDOWN")
	return nil
}
