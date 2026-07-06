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

## コアコンセプト

1. **Git が信頼の源泉(GitOps)** — 各環境のあるべき状態は Git 上のマニフェストで宣言する
2. **昇格 = digest の書き写し** — タグではなくイメージ digest で昇格し、「dev で動いていたものと bit 単位で同じもの」を保証する
3. **環境ごとの更新ポリシー** — dev はブランチ自動追従、prod は手動昇格、のように環境単位で選べる
4. **単一バイナリ** — サーバ・コントローラ・CLI を1つの Go バイナリに同居。Cloud Run 自身にデプロイして動かせる

## ドキュメント

| ドキュメント | 内容 |
|---|---|
| [docs/requirements.md](docs/requirements.md) | 機能要件(MVP / v1.0 / 将来 / 対象外) |
| [docs/architecture.md](docs/architecture.md) | アーキテクチャと技術スタック |
| [docs/roadmap.md](docs/roadmap.md) | 開発ロードマップ(Phase 1〜3) |
| [docs/naming.md](docs/naming.md) | 名前の由来と確保状況 |

## 技術スタック(概要)

- **バックエンド / CLI**: Go(Cloud Run Admin API v2, go-containerregistry, go-git, Connect RPC, cobra)
- **フロントエンド**: TypeScript + React + Vite(Go バイナリに embed)
- **ライセンス**: Apache-2.0(予定)

## ステータス

🚧 設計フェーズ。要件定義は完了、Phase 1(CLI MVP)の実装準備中。
