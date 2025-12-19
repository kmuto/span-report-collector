# Span Report Collector

[English](README.md)

`span-report-collector` は、受信したトレース（Span）の数を、`service.name` および `deployment.environment.name` ごとに集計し、時間・日・月単位の統計をファイルに出力するカスタム OpenTelemetry Collector です。

## 主な機能

* **属性別集計:** サービス名とサービス名前空間（dev/prodなど）の組み合わせごとにカウント。
* **カレンダー同期:** 毎時 0 分 0 秒に統計をレポート出力（設定で変更可能）。
* **累積カウント:** 1時間ごとのリセットに加え、日次・月次の累計を保持。
* **デバッグモード:** `verbose: true` 設定により、受信時の即時ログ出力が可能。

## ビルド方法

このプロジェクトは [OpenTelemetry Collector Builder (OCB)](https://github.com/open-telemetry/opentelemetry-collector/tree/main/cmd/builder) を使用してビルドします。

### 1. 準備

Go 1.24+ と OCB がインストールされていることを確認してください。

```bash
go install go.opentelemetry.io/collector/cmd/builder@latest
```

### 2. バイナリの生成

`builder-config.yaml` があるディレクトリで以下を実行します。

```bash
builder --config builder-config.yaml
```

ビルドが完了すると、`./dist/span-report-collector` にバイナリが生成されます。

## 設定方法

`config.yaml` を作成し、カスタムエクスポーター `spanreportexporter` を定義します。

```yaml
receivers:
  otlp:
    protocols:
      grpc:
      http:

exporters:
  spanreportexporter:
    path: "./span_report.txt"      # レポートの出力先
    report_interval: "1h"          # 出力間隔 (1h, 1m, 10s等)
    verbose: true                  # true にすると受信ごとにログ出力

service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [spanreportexporter]
```

## 実行方法

```bash
./dist/span-report-collector --config config.yaml
```

## レポート形式

`span_report.txt` に以下のような形式で追記されます。

```text
[2025-12-18 08:59:59] env:prod, service:order-api | Hourly(Total:1500, HTTP:1000, SQL:500) | Daily(Total:34200, HTTP:20000, SQL:14200) | Monthly(Total:120500, HTTP:80000, SQL:40500)
[2025-12-18 08:59:59] env:dev, service:auth-svc | Hourly(Total:120, HTTP:0, SQL:0) | Daily(Total:800, HTTP:0, SQL: 0) | Monthly(Total:5200, hTTP:0, SQL:0)
```

### カウンターの定義
- **Total**: すべての受信スパン。
- **HTTP**: `Kind=SERVER` および、`http.route` または `http.target` 属性を持つスパン。
- **SQL**: `db.query.text` または `db.statement` 属性を持つスパン。

### 統計値の性質とリセットタイミング

出力される各数値は、以下のルールに従って集計・リセットされます。

- **hourly**: 前回のレポート出力（通常は1時間前）から現在までのスパン数です。**レポート出力のたびに 0 にリセット**されます。
- **daily**: その日の 00:00:00 からの累積スパン数です。**日付が変わるタイミング（00:00:00 のレポート出力時）に 0 にリセット**されます。
- **monthly**: その月の 1日 00:00:00 からの累積スパン数です。**月が変わるタイミング（毎月1日 00:00:00 のレポート出力時）に 0 にリセット**されます。

> **Note:** コレクターを再起動した場合は、メモリ上の累積値（daily, monthly）は 0 にリセットされますのでご注意ください。

## ライセンス
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
