# Wataridori (渡り鳥)

> **Wataridori (渡り鳥, "migratory bird") — a GitOps CD tool built for Cloud Run.**
> Your images migrate safely from dev to prod.

Wataridori は Google Cloud Run に特化した継続的デリバリー(CD)ツールです。
Artifact Registry のイメージを **digest ベース**で dev → prod へ「昇格」させることを核に、
デプロイの可視化・ロールバック・履歴管理を提供します。

渡り鳥が季節(環境)を越えて確実に目的地へたどり着くように、
検証済みのイメージだけが dev から prod へ渡っていきます。

## なぜ Wataridori か

- **Argo CD は重すぎる** — Kubernetes 前提の汎用ツールで、Cloud Run だけのために運用するにはコンポーネントが多い
- **`gcloud run deploy` を CI に書くのは脆すぎる** — 履歴なし、ロールバック手作業、「dev で動いていたものと同じ」保証なし
- **Wataridori はその間を埋める** — 単一バイナリで動き、Cloud Run のネイティブ機能(リビジョン、トラフィック分割)を最大限活かす

既存の類似ツールとの違い:
[cloud-run-release-manager](https://github.com/GoogleCloudPlatform/cloud-run-release-manager) は
カナリア特化でアーカイブ済み(GitOps・昇格の概念がない)。PipeCD は Cloud Run に対応するが
K8s 中心の汎用アーキテクチャで、Control Plane + Piped の運用が必要。Wataridori は
「digest 昇格 + GitOps」だけに絞った単一バイナリで、この隙間を埋める。

## コアコンセプト

1. **Git が信頼の源泉(GitOps)** — 各環境のあるべき状態は Git 上のマニフェストで宣言する
2. **昇格 = digest の書き写し** — タグではなくイメージ digest で昇格し、「dev で動いていたものと bit 単位で同じもの」を保証する
3. **環境ごとの更新ポリシー** — dev はブランチ自動追従、prod は手動昇格、のように環境単位で選べる
4. **単一バイナリ** — サーバ・コントローラ・CLI を1つの Go バイナリに同居。Cloud Run 自身にデプロイして動かせる

## インストール

[Releases](https://github.com/Retr0413/wataridori/releases) からバイナリを取得するか:

```sh
go install github.com/Retr0413/wataridori/cmd/wataridori@latest
```

名前が長いので短縮エイリアスを推奨:

```sh
alias wtd=wataridori
```

## 3 分 Quickstart

前提: GCP プロジェクト(dev / prod)、Artifact Registry のビルド済みイメージ、
`gcloud auth application-default login` 済み。

**1. マニフェストリポジトリを作る** — [examples/simple](examples/simple/) をコピーして
Git リポジトリにし、プロジェクト ID・リージョン・イメージを自分のものに書き換える。
digest は `crane digest IMAGE:TAG` で取得(タグ参照はエラーになる):

```
your-manifests/
├── wataridori.yaml            # 環境定義(dev = auto, prod = manual)
└── environments/
    ├── dev/hello.yaml         # image: ...@sha256:...(digest 必須)
    └── prod/hello.yaml
```

**2. dev へデプロイ**

```sh
wtd apply --env dev
```

**3. prod へ昇格** — dev の digest が prod のマニフェストへ書き写され、commit が作られる。
環境ごとに AR が分かれていればイメージコピーも自動で行われる:

```sh
wtd promote --to prod   # プラン表示 → y/N 確認 → commit
git push
wtd apply --env prod    # 昇格 = commit、デプロイ = apply(GitOps の分離)
```

**4. 状態確認・ロールバック・履歴**

```sh
wtd status              # Git(あるべき)× Cloud Run(実際)の突き合わせ
wtd rollback --env prod # 前の Ready リビジョンへトラフィック 100% を戻す
wtd history --env prod  # いつ・誰が・何を deploy/promote/rollback したか
```

## ドキュメント

| ドキュメント | 内容 |
|---|---|
| [docs/requirements.md](docs/requirements.md) | 機能要件(MVP / v1.0 / 将来 / 対象外) |
| [docs/architecture.md](docs/architecture.md) | アーキテクチャと技術スタック |
| [docs/roadmap.md](docs/roadmap.md) | 開発ロードマップ(Phase 1〜3) |
| [docs/spec/phase1-cli.md](docs/spec/phase1-cli.md) | Phase 1(CLI)の詳細仕様 |
| [docs/naming.md](docs/naming.md) | 名前の由来と確保状況 |

## 技術スタック(概要)

- **バックエンド / CLI**: Go(Cloud Run Admin API v2, go-containerregistry, go-git, Connect RPC, cobra)
- **フロントエンド**: TypeScript + React + Vite(Go バイナリに embed)
- **ライセンス**: Apache-2.0

## ステータス

🚧 Phase 1(CLI MVP)実装中。`apply` / `promote` / `rollback` / `status` / `history` の
5 コマンドを実装済み。実環境での受け入れテスト(spec §4)が完了し次第 v0.1.0 をリリース予定。
Phase 2(コントローラ + Web UI)以降は [docs/roadmap.md](docs/roadmap.md) を参照。
