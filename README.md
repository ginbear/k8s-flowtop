# k8s-flowtop

Kubernetes 上で流れる非同期処理（Workflows, Jobs, Events, Pipelines）をいい感じに見る TUI

![k8s-flowtop](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)

## Features

- **Job / CronJob** の監視
- **Argo Workflows** (Workflow, CronWorkflow) の監視
- **Argo Events** (Sensor, EventSource) の監視
- **タブ別カラム表示**
  - All: シンプルな概要（KIND, NAMESPACE, NAME, STATUS, SA, DURATION, MESSAGE）
  - Jobs/Workflows: スケジュール重視（cron フィールド, TZ, LAST, NEXT）
  - Events: イベント情報重視（EVENT_SOURCE, EVENT_NAME, TRIGGER）
- **ツリー表示**: 親子関係を可視化（CronWorkflow → Workflow, CronJob → Job）
- **DAG 進捗表示**: Workflow の詳細画面で DAG ノードの進捗を表示
- **タイムゾーン対応**: リソースの timezone 設定を考慮した次回実行時間の計算
- ソート切替（ステータス順 / 次回実行順）
- JST/UTC 切替
- 5秒ごとの自動更新

## Installation

```bash
# From source
go install github.com/ginbear/k8s-flowtop/cmd/flowtop@latest

# Or clone and build
git clone https://github.com/ginbear/k8s-flowtop.git
cd k8s-flowtop
make install
```

## Usage

```bash
# Watch all namespaces
flowtop

# Watch specific namespace
flowtop -n my-namespace

# Show version
flowtop -v
```

## Keybindings

| Key | Action |
|-----|--------|
| `↑/k` | Move up |
| `↓/j` | Move down |
| `Tab` | Next view |
| `1-4` | Switch view (All/Jobs/Workflows/Events) |
| `Enter` | Show details |
| `s` | Sort by next run / status |
| `J` | Toggle JST/UTC |
| `r` | Refresh |
| `?` | Toggle help |
| `q` | Quit |

## Requirements

- Kubernetes cluster with `~/.kube/config` configured
- (Optional) Argo Workflows installed for Workflow resources
- (Optional) Argo Events installed for Sensor/EventSource resources

## Development

```bash
# Build
make build

# Run
make run

# Test
make test

# Lint (requires golangci-lint)
make lint
```

## License

MIT
