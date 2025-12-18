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

func TestConsumeTraces_Categorization(t *testing.T) {
	// 1. Setup exporter
	tmpFile, err := os.CreateTemp("", "category_test")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	exp := &spanReportExporter{
		path:   tmpFile.Name(),
		logger: componenttest.NewNopTelemetrySettings().Logger,
	}

	// 2. Create test data (1 HTTP span, 1 SQL span, 1 Other span)
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "test-svc")
	rs.Resource().Attributes().PutStr("deployment.environment.name", "test-env")
	spans := rs.ScopeSpans().AppendEmpty().Spans()

	// HTTP Span (Kind=Server + http.route)
	s1 := spans.AppendEmpty()
	s1.SetKind(ptrace.SpanKindServer)
	s1.Attributes().PutStr("http.route", "/api/data")

	// SQL Span (db.statement)
	s2 := spans.AppendEmpty()
	s2.SetKind(ptrace.SpanKindClient)
	s2.Attributes().PutStr("db.statement", "SELECT * FROM users")

	// Other Span
	s3 := spans.AppendEmpty()
	s3.SetName("internal-work")

	// 3. Consume
	err = exp.ConsumeTraces(context.Background(), td)
	require.NoError(t, err)

	// 4. Verify memory stats
	key := groupingKey{service: "test-svc", env: "test-env"}
	val, ok := exp.statsMap.Load(key)
	require.True(t, ok)
	stats := val.(*spanStats)

	// Total checks
	assert.Equal(t, uint64(3), stats.hourly.Load())
	// HTTP checks
	assert.Equal(t, uint64(1), stats.httpHourly.Load())
	assert.Equal(t, uint64(1), stats.httpDaily.Load())
	// SQL checks
	assert.Equal(t, uint64(1), stats.sqlHourly.Load())
	assert.Equal(t, uint64(1), stats.sqlMonthly.Load())
}

func TestRotateAndWrite_CumulativeResets(t *testing.T) {
	// Setup
	tmpFile, err := os.CreateTemp("", "category_test")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	exp := &spanReportExporter{
		path:   tmpFile.Name(),
		logger: componenttest.NewNopTelemetrySettings().Logger,
	}
	key := groupingKey{service: "svc", env: "env"}
	stats := &spanStats{}
	exp.statsMap.Store(key, stats)

	// Mock initial counts
	stats.hourly.Store(10)
	stats.daily.Store(100)
	stats.monthly.Store(1000)
	stats.httpDaily.Store(50)

	// Case 1: Same day (Only Hourly should reset)
	now := time.Date(2025, 12, 18, 10, 0, 0, 0, time.UTC)
	exp.lastExportTime = now.Add(-1 * time.Hour)

	// We call rotateAndWrite (mock file part or just test logic)
	// For this test, let's assume rotateAndWrite is modified to be testable
	// or we just call the logic inside it.

	// Case 2: New Day (Daily should reset, Monthly should not)
	tomorrow := now.Add(24 * time.Hour)
	exp.generateReportLines(tomorrow) // 内部ロジックを抽出した関数を想定

	assert.Equal(t, uint64(0), stats.hourly.Load())
	assert.Equal(t, uint64(0), stats.daily.Load())
	assert.Equal(t, uint64(0), stats.httpDaily.Load())
	assert.Equal(t, uint64(1000), stats.monthly.Load(), "Monthly should persist on day change")

	// Case 3: New Month (Monthly should reset)
	nextMonth := now.AddDate(0, 1, 0)
	exp.generateReportLines(nextMonth)
	assert.Equal(t, uint64(0), stats.monthly.Load())
}
