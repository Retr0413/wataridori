# Wataridori

Google Cloud Run に特化した GitOps CD ツールの OSS プロジェクト。
Artifact Registry のイメージを digest ベースで dev → prod へ「昇格」させることが核。

## 現在のフェーズ

設計フェーズ完了、Phase 1(CLI MVP)の実装準備中。実装コードはまだない。

## 必読ドキュメント

- [docs/requirements.md](docs/requirements.md) — 機能要件。**MVP スコープと Non-goals はここが正**。スコープ外の機能を勝手に足さない
- [docs/architecture.md](docs/architecture.md) — アーキテクチャ・技術スタック・リポジトリ構成
- [docs/roadmap.md](docs/roadmap.md) — Phase 1〜3 のロードマップ
- [docs/naming.md](docs/naming.md) — 名前の由来・確保状況

## 重要な設計判断(変更する場合は docs を先に更新)

- 昇格は**タグではなく digest** で行う(bit 単位の同一性保証)
- 真実の源泉は Git(あるべき状態)と Cloud Run API(実際の状態)。DB(SQLite)は履歴・承認記録のみ
- サーバ・コントローラ・CLI は**単一 Go バイナリ**。フロント(React)は embed する
- API は Connect RPC。proto 定義(/proto)が API の単一ソース
- CI(イメージビルド)と Cloud Run 以外のランタイムは**スコープ外**

## 言語・スタック

- バックエンド / CLI: Go(cobra, connectrpc, cloud.google.com/go/run/apiv2, go-containerregistry, go-git)
- フロントエンド: TypeScript + React + Vite + TanStack Query
- ライセンス: Apache-2.0
