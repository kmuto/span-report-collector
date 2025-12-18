package spanreportexporter

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const typeStr = "reportexporter"

var componentType = component.MustNewType("reportexporter")

func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		componentType,
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, component.StabilityLevelAlpha),
	)
}

type Config struct {
	FilePath       string `mapstructure:"path"`
	Verbose        bool   `mapstructure:"verbose"`
	ReportInterval string `mapstructure:"report_interval"`
}

func createDefaultConfig() component.Config {
	return &Config{
		FilePath:       "./span_report.txt",
		Verbose:        false,
		ReportInterval: "1h",
	}
}

func createTracesExporter(ctx context.Context, set exporter.Settings, cfg component.Config) (exporter.Traces, error) {
	c := cfg.(*Config)
	interval, _ := time.ParseDuration(c.ReportInterval)
	if interval <= 0 {
		interval = time.Hour
	}
	exp := &reportExporter{
		path:           c.FilePath,
		verbose:        c.Verbose,
		reportInterval: interval,
		logger:         set.Logger,
		stopCh:         make(chan struct{}),
	}
	return exporterhelper.NewTraces(
		ctx,
		set,
		cfg,
		exp.ConsumeTraces,
		exporterhelper.WithStart(exp.Start),
		exporterhelper.WithShutdown(exp.Shutdown),
	)
}
