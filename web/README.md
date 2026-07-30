# web

Wataridori の Web UI。`wataridori serve` が embed した成果物を配信し、ブラウザは
Connect RPC で `DeploymentService` を呼ぶ。

## 画面

中心は **Pipeline**(昇格ボード)。service を行、環境を列に置き、列は
`ListEnvironments` が返す昇格順(`promoteFrom` のチェーン順)に並ぶ。digest が
dev → prod へ横に流れるのが見えることが、この画面の目的。

- **Pipeline** — service × 環境。cell クリックで詳細、列の間の矢印が昇格導線
- **Timeline** — Cloud Run の revision から復元したデプロイ履歴
- **Inventory** — Cloud Run 上の service 一覧(manifest 管理外のものを含む)
- **Activity** — Wataridori が記録した apply / promote / rollback の履歴

**Timeline と Activity は情報源が違う**。Activity は SQLite の操作記録なので、
CI や手動でデプロイされた service では空になる。Timeline は `ListRevisions` で
Cloud Run 側を読むため、誰がデプロイしたかに関係なく事実が残る。

Timeline の上部「now serving」は各環境が実際に動かしている revision を昇格順に
並べる。digest だけでは環境間の差は読めないが、経過時間(「28 日前」)が並ぶと
prod がどれだけ dev から遅れているかが一目で分かる。行の `serving` と `in Git`
のマークが別々の revision に付いている状態が、そのまま drift の正体になる。

昇格と rollback は **Plan → 確認 → Execute** の二段階。確認ダイアログは
`PlanPromote` / `PlanRollback` を毎回引き直すので、表示される差分はサーバが実際に
行う操作そのものになる。

## 構成

- `src/gen/`: proto から生成した型と client(**手で編集しない**。`make gen`)
- `src/api/`: Connect transport
- `src/views/`: Pipeline / Timeline / Inventory / Activity
- `src/components/`: drawer・ダイアログなど画面をまたぐ部品
- `src/lib/`: board の行組み立てと表示フォーマット(純粋関数)
- `src/theme.css`: パレット。色の定義はここ 1 箇所
- `e2e/`: Playwright テストと、その相手をする Go の fake backend
- `dist/`: `make web-build` の成果物。**コミットする**(`go build` が node を
  必要としないため)

## E2E テスト

```sh
npx playwright install chromium   # 初回のみ
npm --prefix web run build        # dist を最新に(embed 済みの成果物をテストする)
npm --prefix web run test:e2e
```

`e2e/fakeserver` は `server.UseCases` を実装した Go の偽バックエンドで、
Playwright が自動で起動する。GCP 認証は不要。**本物の Connect handler と proto
シリアライズを通る**ので、proto の契約が壊れればテストが落ちる — 手書き JSON の
モックだと素通りしてしまう部分をここで押さえている。

再現性のために、fake のデータは時刻まで固定し、`playwright.config.ts` で
locale (`en-US`) と timezone (`Asia/Tokyo`) を固定している。

`e2e/screenshots.spec.ts` は `docs/screenshots/*.png` を生成する。UI を変えたら
再実行して差分をコミットする。

## 開発

```sh
npm --prefix web ci
make web-build              # dist/ を更新(web/src を変えたら必ず実行)
```

UI を触りながら開発する場合は、RPC サーバと dev server を並べて動かす:

```sh
go run ./cmd/wataridori serve            # :8080 で RPC
npm --prefix web run dev                 # :5173、RPC は :8080 へ proxy
```

## スタック

React + Vite + TanStack Query + Connect(`@connectrpc/connect-web`)。
protobuf-es v2 は service descriptor も生成するため、`protoc-gen-connect-es` は不要。
