# ロードマップ

MVP を絞ることが最重要。Phase 1 を早く出して README と quickstart を磨く方が、
機能を積むより OSS としては先。

## Phase 1 — CLI だけの MVP(目標: 2〜3 週間)

UI なし。これだけで既存の手動昇格プログラムを置き換えられる状態にする。
詳細仕様は [spec/phase1-cli.md](spec/phase1-cli.md)、実装タスクは
[GitHub Milestone "Phase 1 — CLI MVP"](https://github.com/Retr0413/wataridori/milestone/1) を正とする。

- [ ] マニフェスト YAML スキーマの設計(環境定義・更新ポリシー・イメージ digest)
- [ ] `wataridori apply` — マニフェストどおりに Cloud Run へデプロイ
- [ ] `wataridori promote --from dev --to prod` — digest を書き写して commit(必要なら AR 間イメージコピー)
- [ ] `wataridori rollback --env prod` — 前リビジョンへトラフィックを戻す
- [ ] `wataridori status` — 環境 × イメージの現在状態一覧
- [ ] `wataridori history` — デプロイ履歴(SQLite)
- [ ] goreleaser + GitHub Actions(lint / test / release)
- [ ] README(アーキ図 + 3 分 quickstart)

## Phase 2 — コントローラ + Web UI

- [ ] Reconcile loop(Git poll / webhook → Cloud Run へ反映)
- [ ] ブランチ自動追従(dev 環境の auto ポリシー)
- [ ] Connect RPC API の整備(CLI も同 API 経由に移行)
- [ ] Web UI: 環境ダッシュボード・昇格ボタン・履歴表示
- [ ] 承認ゲート(prod 昇格に approve 必須)
- [ ] Slack / webhook 通知
- [ ] コンテナ状態詳細(リビジョンステータス・トラフィック配分・Console へのディープリンク)

## Phase 3 — プログレッシブデリバリー(本当の差別化)

- [ ] トラフィック分割によるカナリアリリース(10% → 50% → 100%)
- [ ] リビジョンタグ付きプレビュー URL(トラフィック 0% での事前確認)
- [ ] Cloud Monitoring 連動の自動ロールバック

## リリース前チェックリスト(初日〜早期にやること)

- [x] 名前決定: **Wataridori**
- [ ] GitHub リポジトリ作成(`github.com/<you>/wataridori`)
- [ ] ドメイン確保の検討(`wataridori.dev` / `wataridori.io` — 2026-07-06 時点で両方未登録)
- [ ] LICENSE(Apache-2.0)・CONTRIBUTING.md・Code of Conduct
