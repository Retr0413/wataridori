# アーキテクチャ

## 全体像

```
┌─ Web UI (TypeScript / React) ──┐
│  環境一覧・昇格ボタン・承認・   │
│  デプロイ履歴・トラフィック操作  │
└──────────┬──────────────────────┘
           │ Connect RPC (gRPC 互換, ブラウザ直結可)
┌──────────▼──────────────────────┐
│  API Server + Controller (Go)   │  ← 単一バイナリに同居
│  - Git リポジトリの poll/webhook │
│  - Reconcile loop               │
│  - 昇格 = Git への commit/PR     │
└───┬──────────────┬──────────────┘
    │ Cloud Run    │ Artifact Registry
    │ Admin API v2 │ (digest 解決・コピー)
```

## 設計原則

### 単一バイナリ

Argo CD が重い理由の一つはコンポーネント分割。Wataridori はサーバ・コントローラ・CLI を
1つの Go バイナリに同居させ、**Cloud Run に自分自身をデプロイして動く CD ツール**にする。
導入障壁を劇的に下げることが OSS としての採用に直結する。

### 真実の所在

- **あるべき状態**: Git 上のマニフェスト(GitOps)
- **実際の状態**: Cloud Run Admin API から取得
- **DB は補助のみ**: 履歴・承認記録用の SQLite(将来的に Firestore オプション)。状態の真実は DB に置かない

### 認証

- GCP 側: Workload Identity / Application Default Credentials(ADC)
- UI 側: IAP または OIDC

## 技術スタック

| 層 | 選定 | 理由 |
|---|---|---|
| Cloud Run 操作 | `cloud.google.com/go/run/apiv2` | 公式クライアント。Knative 形式 YAML より proto ベースの v2 API が扱いやすい |
| イメージ操作 | [`go-containerregistry`](https://github.com/google/go-containerregistry) | digest 解決・レジストリ間コピー(crane のライブラリ) |
| Git 操作 | `go-git` + GitHub/GitLab API | 昇格 PR の作成に必要 |
| API | Connect RPC (`connectrpc.com/connect`) | proto 定義から Go サーバと TypeScript クライアントを両方生成。型共有問題が消える |
| CLI | `cobra` | UI と同じ API を叩く CLI を提供 |
| フロント | React + Vite + TanStack Query | Connect の生成クライアントと相性が良い。成果物は Go バイナリに `embed` |
| ストア | SQLite | 履歴・承認記録用。導入の軽さを優先 |

## リポジトリ構成(モノレポ)

```
/cmd/wataridori/      # main() のみ
/internal/
  cli/                # cobra コマンド層(フラグ処理・表示のみ。ロジックを持たない)
  core/               # ユースケース層(apply / promote / rollback / status の手順)
  manifest/           # マニフェスト YAML の型・loader・validator
  controller/         # reconcile loop(Phase 2)
  cloudrun/           # Admin API ラッパ
  registry/           # digest 解決・イメージコピー
  gitops/             # Git 監視・昇格 PR 作成
  store/              # SQLite
/proto/               # Connect RPC 定義(API の単一ソース、Phase 2〜)
/web/                 # TypeScript フロント(Phase 2〜)
/docs/                # 設計ドキュメント・quickstart
/examples/            # サンプルのマニフェストリポジトリ
```

依存方向は一方通行: `cli → core → 下位パッケージ`(Phase 2 で `server → core` が並ぶ)。
core の Request/Result 型は構造化データとして定義し、表示は cli 層・シリアライズは
API 層の責務とする。これにより CLI と Web UI が同一のユースケース実装を共有する。

## Cloud Run 特化で活きる設計ポイント

1. **リビジョンベースのロールバック** — Cloud Run はリビジョンを保持しているため、K8s と違い「前リビジョンへトラフィック 100% を戻す」が API 一発
2. **トラフィック分割** — `traffic` フィールドの段階更新でカナリア / Blue-Green をネイティブ実装できる(将来対応)
3. **リビジョンタグ付き URL** — 昇格前に prod でタグ URL(トラフィック 0%)を発行して動作確認(将来対応)

## OSS としての実務

- ライセンス: **Apache-2.0**(Argo / PipeCD と同じ。企業採用されやすい)
- 配布: `goreleaser` でバイナリ + コンテナイメージ
- CI: GitHub Actions(lint / test / release)
- ドキュメント: アーキ図 + 3 分 quickstart を README に。設計思想は docs/ に残す
- 競合調査: アーカイブ済みの [cloud-run-release-manager](https://github.com/GoogleCloudPlatform/cloud-run-release-manager) や PipeCD の Cloud Run 対応を調査し、「なぜ既存では駄目か」を README で明文化する
