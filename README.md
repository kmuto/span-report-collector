# Span Report Collector

[日本語](README-ja.md)

`span-report-collector` is a custom OpenTelemetry Collector that aggregates the number of received spans by `service.name` and `deployment.environment.name`, exporting hourly, daily, and monthly statistics to a file.

In addition to span aggregation, it acts as a gateway that relays telemetry data to external backends like Jaeger, SigNoz, and Mackerel.

## Key Features

* **Attribute-based Aggregation:** Counts spans based on the combination of Service Name and Environment (e.g., dev/prod).
* **Workload Classification:** Automatically identifies and separately aggregates HTTP requests and SQL queries.
* **Calendar Synchronization:** Automatically exports statistics at the top of every hour (00:00:00), which is configurable.
* **Cumulative Counters:** Tracks hourly counts along with daily and monthly running totals.
* **Debug Mode:** Enables immediate logging upon span receipt by setting `verbose: true`.
* **Multi-platform Support:** Pre-compiled binaries are available for Linux, macOS, and Windows.

## Installation
Download the latest version for your OS and architecture from https://github.com/kmuto/span-report-collector/releases .

- **Linux / macOS**: Download and extract the `.tar.gz` archive.
- **Windows**: Download and extract the `.zip` archive.

## Configuration

A sample `config.yaml` is provided in the extracted folder.

```yaml
receivers:
  otlp:
    protocols:
      grpc:
      http:

exporters:
  spanreportexporter:
    path: "./span_report.txt"      # File path for the report
    report_interval: "1h"          # Export interval (e.g., 1h, 1m, 10s)
    verbose: false                 # Set to true for per-receipt logging

service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [spanreportexporter]
```

### Examples

#### Allowing requests from non-localhost addresses
```
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318
```

#### Forwarding received spans to Mackerel
```yaml
exporters:
  spanreportexporter:
    ...
  otlphttp/mackerel:
    endpoint: https://otlp-vaxila.mackerelio.com
    compression: gzip
    headers:
      Mackerel-Api-Key: ${env:MACKEREL_APIKEY}

service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [spanreportexporter, otlphttp/mackerel]
```

## Usage

```bash
./span-report-collector --config config.yaml
```

## Report Format

Data is appended to `span_report.txt` in the following format:

```text
[2025-12-18 08:59:59] env:prod, service:order-api | Hourly(Total:1500, HTTP:1000, SQL:500) | Daily(Total:34200, HTTP:20000, SQL:14200) | Monthly(Total:120500, HTTP:80000, SQL:40500)
[2025-12-18 08:59:59] env:dev, service:auth-svc | Hourly(Total:120, HTTP:0, SQL:0) | Daily(Total:800, HTTP:0, SQL: 0) | Monthly(Total:5200, hTTP:0, SQL:0)
```

### Counter Definitions
- **Total**: All spans received.
- **HTTP**: Spans with `Kind=SERVER` and `http.route` or `http.target` attribute.
- **SQL**: Spans with `db.query.text` or `db.statement` attribute.

### Counters and Reset Logic

Each value is aggregated and reset according to the following rules:

* **Hourly**: The number of spans received since the last report (typically the last hour). It is **reset to 0 after every report**.
* **Daily**: The cumulative span count since 00:00:00 of the current day. It is **reset to 0 at the start of a new day** (during the 00:00:00 report).
* **Monthly**: The cumulative span count since 00:00:00 on the 1st of the current month. It is **reset to 0 at the start of a new month** (during the 00:00:00 report on the 1st).

> **Note:** Please be aware that daily and monthly cumulative values are stored in memory and will be **reset to 0 if the collector is restarted**.

## License

```
Copyright 2025 Kenshi Muto

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
```
