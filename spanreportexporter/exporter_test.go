package spanreportexporter

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestConsumeTraces_Counting(t *testing.T) {
	// 1. テスト用の一時ファイル作成
	tmpFile, err := os.CreateTemp("", "span_report_test.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	// 2. Exporterのインスタンス化
	exp := &spanReportExporter{
		path:    tmpFile.Name(),
		verbose: true,
		logger:  componenttest.NewNopTelemetrySettings().Logger,
		stopCh:  make(chan struct{}),
	}

	// 3. テストデータの作成
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "test-service")
	rs.Resource().Attributes().PutStr("deployment.environment.name", "dev")

	scope := rs.ScopeSpans().AppendEmpty()
	scope.Spans().AppendEmpty() // 1つめのスパン
	scope.Spans().AppendEmpty() // 2つめのスパン

	// 4. 実行
	ctx := context.Background()
	err = exp.ConsumeTraces(ctx, td)
	assert.NoError(t, err)

	// 5. 検証: statsMapに正しく値が入っているか
	key := groupingKey{service: "test-service", env: "dev"}
	val, ok := exp.statsMap.Load(key)
	assert.True(t, ok, "statsMap should have the key")

	stats := val.(*spanStats)
	assert.Equal(t, uint64(2), stats.hourly.Load())
	assert.Equal(t, uint64(2), stats.daily.Load())
	assert.Equal(t, uint64(2), stats.monthly.Load())
}

func TestRotateAndWrite_ResetLogic(t *testing.T) {
	// 1. セットアップ
	tmpFile, err := os.CreateTemp("", "span_reset_test.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	exp := &spanReportExporter{
		path:   tmpFile.Name(),
		logger: componenttest.NewNopTelemetrySettings().Logger,
	}

	key := groupingKey{service: "svc", env: "env"}
	stats := &spanStats{}
	stats.hourly.Store(10)
	stats.daily.Store(100)
	stats.monthly.Store(1000)
	exp.statsMap.Store(key, stats)

	// 2. 「月が変わった」状態をシミュレート
	lastExport := time.Date(2025, 12, 31, 23, 0, 0, 0, time.UTC)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	exp.lastExportTime = lastExport

	// 3. 実行
	exp.rotateAndWrite(now)

	// 4. 検証: Hourly, Daily, Monthlyすべてがリセットされているはず
	assert.Equal(t, uint64(0), stats.hourly.Load())
	assert.Equal(t, uint64(0), stats.daily.Load())
	assert.Equal(t, uint64(0), stats.monthly.Load())
}
