# 今後追加すると良い機能

このドキュメントは、現在の CLI MVP に続いて Wataridori に追加すると良い機能を整理する。
PipeCD の考え方は参考にするが、Wataridori は Cloud Run の digest 昇格に絞った軽量 GitOps CD
であり続ける。したがって、機能追加は「見える」「止められる」「戻せる」「承認できる」を
小さく実現する順で進める。

## 判断基準

- Cloud Run 特化を維持する
- digest ベースの昇格を壊さない
- 単一 Go バイナリで配布できる範囲に収める
- CI のイメージビルドは扱わない
- メトリクスやログの本格 UI は Cloud Console / Cloud Logging へのリンクで代替する
- Phase 1 の CLI MVP を大きくしすぎない

## 推奨順

1. Plan Preview
2. GitHub PR 昇格モード
3. 実 GCP 受け入れテスト用サンプル環境
4. Web UI の最小版
5. Cloud Run Inventory
6. 既存 Cloud Run service の import
7. Managed Image Update
8. 承認ゲート
9. 通知
10. Drift 詳細
11. Cloud Console deep link
12. 段階的 rollout
13. 自動 rollback
14. Event watcher
15. Deployment Chain

## 最優先

### 1. Plan Preview

`promote` / `apply` 前に、何が変わるかを明確に表示する。

例:

- prod の `hello` が `sha256:aaa` から `sha256:bbb` に変わる
- `imageCopy` により dev repository から prod repository へ digest 指定コピーが走る
- Cloud Run service が create / update / no-op のどれになるか
- actual revision と desired manifest が drift しているか

Plan Preview は CLI、Web UI、将来の PR コメントのすべてに効く。最初は CLI の構造化出力を
強化し、後から Connect RPC / UI で同じデータを表示する。

### 2. GitHub PR 昇格モード

現在の `promote` は target manifest を書き換えて commit を作る。OSS としては、prod 昇格を
PR 化できるモードがあると安全性と GitOps らしさが上がる。

想定 UX:

```sh
wataridori promote --to prod --pr
```

この機能では以下を行う。

- 昇格用 branch を作成
- prod manifest の digest を更新
- commit を作成
- GitHub PR を作成
- PR 本文に Plan Preview を記載

注意点:

- push / PR 作成は通常の `promote` では行わない
- GitHub 以外の Git provider は後回しにする
- Phase 1 の基本挙動は「local commit まで」に保つ

### 3. 実 GCP 受け入れテスト用サンプル環境

Wataridori は Cloud Run / Artifact Registry / IAM / ADC に依存するため、ユニットテストだけでは
リリース品質を判断しにくい。実 GCP で以下を検証できる手順とサンプルを整備する。

受け入れテストの流れ:

1. dev / prod の GCP project または environment を用意する
2. Artifact Registry に digest-pinned image を用意する
3. `wataridori apply --env dev` で dev service を作成する
4. dev manifest の digest を更新して再度 `apply` する
5. `wataridori promote --to prod` で prod manifest に digest を昇格する
6. `wataridori apply --env prod` で prod service を更新する
7. `wataridori rollback --env prod` で前 revision に traffic を戻す
8. `wataridori status` で drift を確認する
9. `wataridori history --env prod` で履歴を確認する

この手順は release checklist としても使う。

### 4. Web UI の最小版

最初の Web UI は多機能にしない。Cloud Run 運用に必要な状態確認と安全な操作だけに絞る。

最小画面:

- 環境一覧
- service 一覧
- desired digest / actual digest
- revision / traffic / Ready 状態
- drift 表示
- promote plan 表示
- history 表示

最初は「操作できる UI」より「状態が分かる UI」を優先する。昇格・rollback の実行ボタンは、
Plan Preview と承認ゲートの設計が固まってから追加する。

## v1.0 で欲しい機能

### 5. Cloud Run Inventory

Cloud Run API から project / region 内の service を一覧し、Wataridori 管理対象かどうかを表示する。
詳細な設計方針は [gitops-cloudrun-management.md](gitops-cloudrun-management.md) にまとめる。

表示したい内容:

- manifest に存在する managed service
- Cloud Run には存在するが manifest に存在しない unmanaged service
- desired image / actual image
- serving revision
- traffic
- Ready 状態
- Cloud Console deep link

この機能は read-only なので GitOps を壊さない。Web UI の価値を上げる最初の拡張として扱う。

### 6. 既存 Cloud Run service の import

unmanaged service を Wataridori の manifest に変換し、Git commit / PR として取り込む。

流れ:

1. Cloud Run Inventory から unmanaged service を選ぶ
2. Cloud Run service の設定を Phase 1 manifest schema に変換する
3. 未対応フィールドがあれば warning として表示する
4. 生成 manifest を review する
5. Git commit / PR を作る

既存の Cloud Run 利用者が Wataridori を導入しやすくなるため、OSS として重要度が高い。

### 7. Managed Image Update

管理対象 service に対して、Artifact Registry の digest を選んで manifest を更新する。

注意点:

- 標準導線では Cloud Run を直接更新しない
- manifest 更新を Git commit / PR として残す
- prod では承認ゲートと組み合わせる
- apply / controller が Cloud Run へ反映する

任意 image を直接 Cloud Run に反映する機能は、通常の image update ではなく
`break-glass` の緊急操作として別扱いにする。

### 8. 承認ゲート

prod 昇格前に approve を必須にできる機能。PipeCD の `WAIT_APPROVAL` に近いが、
Wataridori では prod の `promote` / `apply` に絞る。

要件:

- 対象環境ごとに承認必須を設定できる
- 誰が承認したかを履歴に残す
- 承認前に Plan Preview を確認できる
- 承認済みの plan と実行時の plan がずれた場合は再承認を求める

### 9. 通知

Slack / generic webhook へ操作イベントを通知する。

対象イベント:

- promote planned
- promote committed
- apply started / succeeded / failed
- rollback executed
- drift detected
- approval requested / approved / rejected

通知はまず webhook payload を安定させ、その後 Slack 用の見やすい表示を追加する。

### 10. Drift 詳細

現在の `status` は digest 比較が中心である。次の段階では、なぜ drift しているかをより詳しく出す。

表示したい内容:

- desired image / actual serving image
- desired digest / actual digest
- serving revision
- latest ready revision
- traffic allocation
- Ready / failed reason
- manifest に存在するが Cloud Run に存在しない service
- Cloud Run に存在するが manifest に存在しない service

ただし、Cloud Run の全フィールド差分を自前で完全再現する必要はない。MVP では image / revision /
traffic / Ready に絞る。

### 11. Cloud Console deep link

Wataridori はログやメトリクス UI を再発明しない。代わりに Cloud Console / Cloud Logging への
deep link を CLI / Web UI に出す。

リンク候補:

- Cloud Run service
- Cloud Run revisions
- Cloud Logging query
- Cloud Monitoring dashboard

## 将来で良い機能

### 12. 段階的 rollout

Cloud Run の traffic split を使い、prod 反映を段階的に進める。

例:

```yaml
rollout:
  steps:
    - percent: 10
      wait: 10m
    - percent: 50
      wait: 10m
    - percent: 100
```

最初は手動ステップ実行でもよい。自動進行や analysis 連動は後回しにする。

### 13. 自動 rollback

Cloud Monitoring や HTTP smoke test と連動し、異常を検知したら rollback する。

最初に扱う候補:

- HTTP smoke test が失敗した
- error rate が閾値を超えた
- Cloud Run revision が Ready にならない

これは強力だが設計コストが高い。承認・通知・状態可視化・段階的 rollout が揃ってから扱う。

### 14. Event watcher

Artifact Registry push や GitHub Actions 完了などを受けて、dev manifest の digest を自動更新する。

注意点:

- CI のビルドは行わない
- ビルド済み image digest を manifest に反映するだけにする
- prod への反映は引き続き promote / approval によって制御する

### 15. Deployment Chain

`dev -> staging -> prod` のような多段昇格を扱う。

最初は `promoteFrom` を環境ごとに設定できる現在のモデルを活かし、環境グラフを複雑にしすぎない。

例:

```yaml
environments:
  dev:
    policy: auto
  staging:
    policy: manual
    promoteFrom: dev
  prod:
    policy: manual
    promoteFrom: staging
```

## やらないこと

以下は Wataridori の差別化を弱めるため、少なくとも v1.0 までは扱わない。

- Cloud Run 以外の runtime 対応
- Kubernetes / ECS / Lambda / Terraform の provider 化
- CI image build
- Cloud Logging / Cloud Monitoring の本格 UI
- 複雑な multi-tenant Control Plane
- plugin system

## PipeCD から取り込む考え方

PipeCD の機能をそのまま再現するのではなく、以下の考え方を小さく取り込む。

- deployment の plan を事前に見せる
- 本番操作には approval を挟める
- 実環境と Git の drift を検知する
- pipeline / deployment の状態を UI で見せる
- failure 時に戻せる導線を持つ
- notification でチームに状態を伝える

Wataridori の価値は、これらを Cloud Run と digest 昇格に絞って、軽く導入できる形で提供することにある。
