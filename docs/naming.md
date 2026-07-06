# 名前: Wataridori(渡り鳥)

## 由来

渡り鳥は「季節(環境)を越えて、確実に目的地へたどり着く」存在。
検証済みのイメージが dev → staging → prod と環境を渡っていく、という
このツールのコア概念(digest ベースの昇格)にきれいに乗る。

> Wataridori (渡り鳥, "migratory bird") — a GitOps CD tool for Cloud Run.
> Your images migrate safely from dev to prod.

## 確保状況(2026-07-06 調査)

| 項目 | 状況 |
|---|---|
| 競合 OSS | 実質なし。misoca/wataridori(esa.io 移行ツール、8 stars)のみで CD/インフラ領域の被りゼロ |
| GitHub ユーザー名 `wataridori` | 取得済み(個人アカウント)。org 化する場合は `wataridori-dev` / `wataridori-cd` 等を使う |
| `wataridori.dev` | 未登録(RDAP 404) |
| `wataridori.io` | 未登録(RDAP 404) |

## 注意点

英語話者には 5 音節とやや長く、スペルミスもされやすい。
CLI には短縮エイリアス(`wtd` など)を用意し、README で alias を推奨する
(`kubectl` → `k` と同じ文化)。

## 検討した他候補

- **Tasuki(襷)** — 駅伝で走者間を渡す襷。意味は最有力だったが Wataridori を採用
- **Ekiden** — mirego/ekiden(GitHub Actions ランナー)等と被り気味
- **Baton / Stride / Relay 系** — 既存の有名 OSS と衝突するため見送り
