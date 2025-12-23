# Span Report Collector ("Span-su wo Kazoeru-kun")

[日本語](README-ja.md)

`span-report-collector` aggregates the number of received Spans per `service.name` and `deployment.environment.name`, and exports hourly, daily, and monthly statistics to a file.

Since it retains full Collector functionality, it can also act as a gateway to forward trace data to external backends such as Jaeger, SigNoz, or Mackerel while simultaneously performing aggregation.

## Key Features

* **Attribute-based Aggregation:** Counts spans based on the combination of service name and environment (e.g., dev/prod).
* **TUI Dashboard:** Displays a dynamically updated statistics screen directly in your terminal.
* **Zero-Config:** Works out of the box without a configuration file. Customizable via environment variables.
* **Workload Classification:** Automatically identifies and separately counts HTTP requests and SQL queries.
* **Calendar Sync:** Exports statistical reports exactly at the top of the hour (customizable).
* **Cumulative Counting:** Maintains daily and monthly totals in addition to hourly counts.
* **Debug Mode:** Supports immediate logging upon span reception by setting `verbose: true`.
* **Multi-platform Support:** Binaries available for Linux, macOS, and Windows.

## Installation

Download the latest binary for your OS and architecture from the [Releases](https://github.com/kmuto/span-report-collector/releases) page.

* **Linux / macOS**: Download and extract the `.tar.gz` file.
* **Windows**: Download and extract the `.zip` file.

## Usage

```sh
./span-report-collector
```

## Report Format

By default, statistics are appended to `span_report.txt` in the following format:

```text
[2025-12-18 08:59:59] service:order-api, env:prod | Hourly(Total:1500, HTTP:1000, SQL:500) | Daily(Total:34200, HTTP:20000, SQL:14200) | Monthly(Total:120500, HTTP:80000, SQL:40500)
[2025-12-18 08:59:59] service:auth-svc, env:dev | Hourly(Total:120, HTTP:0, SQL:0) | Daily(Total:800, HTTP:0, SQL: 0) | Monthly(Total:5200, hTTP:0, SQL:0)
```

### Counter Definitions

* **Total**: All received spans.
* **HTTP**: Spans with `Kind=SERVER` and containing `http.route` or `http.target` attributes.
* **SQL**: Spans containing `db.query.text` or `db.statement` attributes.

### Reset Intervals and Behavior

Statistics are collected and reset according to the following rules:

* **hourly**: Number of spans since the last report (typically 1 hour). **Resets to 0 after each report.**
* **daily**: Cumulative spans since 00:00:00 of the current day. **Resets to 0 at midnight (00:00:00).**
* **monthly**: Cumulative spans since 00:00:00 on the 1st of the month. **Resets to 0 at the start of each month.**

> **Note:** Restarting the collector will reset the in-memory cumulative values (`daily`, `monthly`) to 0.

## Customization

### Environment Variables

You can customize the behavior using the following environment variables:

| Environment Variable | Description | Default Value |
| --- | --- | --- |
| `SPAN_REPORT_TUI` | Enable TUI mode (`true` / `false`) | `false` |
| `SPAN_REPORT_VERBOSE` | Enable verbose logging (Recommended when TUI is disabled) | `false` |
| `SPAN_REPORT_PATH` | File path for the statistical report | `./span_report.txt` |
| `SPAN_REPORT_INTERVAL` | Interval for file output (e.g., `1h`, `30m`) | `1h` |
| `SPAN_REPORT_OTLP_ENDPOINT_GRPC` | Listen address for gRPC receiver | `localhost:4317` |
| `SPAN_REPORT_OTLP_ENDPOINT_HTTP` | Listen address for HTTP receiver | `localhost:4318` |

#### Example: Launch in TUI mode and expose the endpoint

```sh
SPAN_REPORT_TUI=true SPAN_REPORT_OTLP_ENDPOINT_HTTP=0.0.0.0:4318 ./span-report-collector
```

![](./tui.png)

## Using a Custom Configuration File

To use a custom `config.yaml`, use the `--config` flag. In this case, the environment variables mentioned above are ignored, and the settings in the file take precedence.

```sh
./span-report-collector --config my-custom-config.yaml
```

A sample `config.yaml` is provided in the extracted folder.

#### Example: Forwarding to Mackerel while aggregating

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: localhost:4317
      http:
        endpoint: localhost:4318

exporters:
  spanreportexporter:
    path: "./span_report.txt"
    report_interval: "1h"
    tui: false
    verbose: false
  otlphttp/mackerel:
    endpoint: https://otlp-vaxila.mackerelio.com
    compression: gzip
    headers:
      Mackerel-Api-Key: ${env:MACKEREL_APIKEY}

service:
  telemetry:
    logs:
      level: error
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [spanreportexporter, otlphttp/mackerel]
```

## TUI Controls

When TUI mode is enabled, you can use the following keys:

* `q` or `Ctrl+C`: Quit the application.
* The screen displays the following information:
  * **Uptime**: Elapsed time since startup.
  * **Hourly / Daily / Monthly**: Cumulative counts (Total / HTTP / SQL) for each period.

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
