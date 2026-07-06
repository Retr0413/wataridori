---
name: project-overview
description: Wataridori プロジェクトの全容(コンセプト・機能要件・アーキテクチャ・ロードマップ)を説明する。プロジェクトの概要、スコープ、設計判断、次にやることを聞かれたとき、または新しい機能の追加がスコープ内か判断が必要なときに使う。
---

# Wataridori プロジェクト概要

## 一言でいうと

**Cloud Run 特化の GitOps CD ツール。** Artifact Registry のイメージを digest ベースで
dev → prod へ「昇格」させる。Argo CD ほど重くなく、`gcloud run deploy` を CI に書くより堅牢、
という隙間を狙う OSS(Apache-2.0)。

## コアコンセプト(これに反する提案はしない)

1. **GitOps**: 各環境のあるべき状態は Git 上のマニフェスト。昇格 = digest を書き写す commit/PR
2. **digest ベース**: タグではなく digest で昇格し「dev で動いていたものと同じ」を保証
3. **環境ごとの更新ポリシー**: dev = ブランチ自動追従、prod = 手動昇格、を環境単位で選択
4. **単一バイナリ**: サーバ・コントローラ・CLI が 1 バイナリ。Cloud Run 自身にデプロイ可能

## スコープ判断の基準

質問「この機能はやるべきか?」への答え方:

- **MVP**: 更新ポリシー / digest 昇格 / ロールバック / 状態一覧 / 履歴 / CLI
- **v1.0**: Web UI / コンテナ状態詳細 / 承認ゲート / 通知
- **将来**: カナリア(トラフィック分割)/ 自動ロールバック
- **やらない**: CI(ビルド)/ Cloud Run 以外 / メトリクス・ログ表示(Console リンクで代替)

詳細は docs/requirements.md を参照。迷ったら「MVP を小さく保つ」側に倒す。

## 技術スタック

Go(cobra, Connect RPC, Cloud Run Admin API v2, go-containerregistry, go-git, SQLite)+
TypeScript(React, Vite, TanStack Query, Go バイナリに embed)。
proto 定義(/proto)が API の単一ソースで、Go サーバと TS クライアントを両方生成する。

## 現在地と次の一手

設計フェーズ完了。次は Phase 1(CLI MVP):
マニフェスト YAML スキーマの設計 → `apply` / `promote` / `rollback` / `status` / `history` の実装。
詳細は docs/roadmap.md。

## CLI の想定 UX

```
wataridori promote --from dev --to prod
wataridori rollback --env prod
wataridori status
wataridori history --env prod
```

短縮エイリアス `wtd` を README で推奨する。
