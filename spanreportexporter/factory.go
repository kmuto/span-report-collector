package spanreportexporter

import (
	"context"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
)

const typeStr = "reportexporter"

func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		typeStr,
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, component.StabilityLevelAlpha),
	)
}

type Config struct {
	FilePath string `mapstructure:"path"`
}

func createDefaultConfig() component.Config {
	return &Config{FilePath: "./span_report.txt"}
}

func createTracesExporter(_ context.Context, set exporter.CreateSettings, cfg component.Config) (exporter.Traces, error) {
	c := cfg.(*Config)
	exp := &reportExporter{
		path:   c.FilePath,
		logger: set.Logger,
		stopCh: make(chan struct{}),
	}
	return exporter.NewTracesExporter(context.TODO(), set, exp.ConsumeTraces, 
		exporter.WithStart(exp.Start), 
		exporter.WithShutdown(exp.Shutdown))
}
