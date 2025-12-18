# Span Report Collector

[日本語](README-ja.md)

`span-report-collector` is a custom OpenTelemetry Collector that aggregates the number of received spans by `service.name` and `deployment.environment.name`, exporting hourly, daily, and monthly statistics to a file.

## Key Features

* **Attribute-based Aggregation:** Counts spans based on the combination of Service Name and Environment (e.g., dev/prod).
* **Calendar Synchronization:** Automatically exports statistics at the top of every hour (00:00:00), which is configurable.
* **Cumulative Counters:** Tracks hourly counts along with daily and monthly running totals.
* **Debug Mode:** Enables immediate logging upon span receipt by setting `verbose: true`.

## Build Instructions

This project is built using the [OpenTelemetry Collector Builder (OCB)](https://github.com/open-telemetry/opentelemetry-collector/tree/main/cmd/builder).

### 1. Prerequisites

Ensure you have Go 1.24+ and OCB installed.

```bash
go install go.opentelemetry.io/collector/cmd/builder@latest

```

### 2. Generate Binary

Run the following command in the directory containing `builder-config.yaml`:

```bash
builder --config builder-config.yaml

```

Once completed, the binary will be generated at `./dist/span-report-collector`.

## Configuration

Create a `config.yaml` to define the `reportexporter` custom exporter.

```yaml
receivers:
  otlp:
    protocols:
      grpc:
      http:

exporters:
  reportexporter:
    path: "./span_report.txt"      # File path for the report
    report_interval: "1h"          # Export interval (e.g., 1h, 1m, 10s)
    verbose: true                  # Set to true for per-receipt logging

service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [reportexporter]

```

## Usage

```bash
./dist/span-report-collector --config config.yaml

```

## Report Format

Data is appended to `span_report.txt` in the following format:

```text
[2025-12-18 08:59:59] env:prod, service:order-api | hourly:1500, daily:34200, monthly:120500
[2025-12-18 08:59:59] env:dev, service:auth-svc | hourly:120, daily:800, monthly:5200

```

### Counters and Reset Logic

Each value is aggregated and reset according to the following rules:

* **hourly**: The number of spans received since the last report (typically the last hour). It is **reset to 0 after every report**.
* **daily**: The cumulative span count since 00:00:00 of the current day. It is **reset to 0 at the start of a new day** (during the 00:00:00 report).
* **monthly**: The cumulative span count since 00:00:00 on the 1st of the current month. It is **reset to 0 at the start of a new month** (during the 00:00:00 report on the 1st).

> **Note:** Please be aware that daily and monthly cumulative values are stored in memory and will be **reset to 0 if the collector is restarted**.

## License

This project is licensed under the Apache 2.0 License.

Copyright 2025 Kenshi Muto
