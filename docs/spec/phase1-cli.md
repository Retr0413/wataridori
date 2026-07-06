# Phase 1 詳細仕様 — CLI MVP

[requirements.md](../requirements.md) の MVP 必須機能を、実装可能なレベルまで詳細化する。
スコープは [roadmap.md](../roadmap.md) の Phase 1 と一致する。**ここに書いていないものは Phase 1 ではやらない。**

## 前提と全体像

- Phase 1 はサーバ/コントローラなしの **CLI 単体**。ユーザーはマニフェストリポジトリの
  working copy 内で `wataridori` を実行する
- あるべき状態 = リポジトリ内のマニフェスト、実際の状態 = Cloud Run Admin API(v2)からの取得値
- 認証は GCP の Application Default Credentials(ADC)のみ。独自の認証機構は持たない
- Phase 1 での「昇格の記録」の一次ソースは **Git の commit そのもの**。SQLite はローカルの
  操作履歴(いつ・誰が・何を実行したか)の補助記録

```
マニフェストリポジトリ(ユーザー管理)
├── wataridori.yaml            # 環境定義(リポジトリルート)
└── environments/
    ├── dev/
    │   └── my-app.yaml        # サービスマニフェスト(1 ファイル = 1 サービス)
    └── prod/
        └── my-app.yaml
```

## 1. マニフェスト YAML スキーマ

### 1.1 `wataridori.yaml`(環境定義)

```yaml
version: 1
environments:
  dev:
    policy: auto                # auto | manual
    branch: develop             # policy: auto のとき必須(Phase 1 では検証のみ、追従は Phase 2)
    gcp:
      project: my-app-dev
      region: asia-northeast1
    services: environments/dev  # サービスマニフェストを置くディレクトリ(リポジトリルート相対)
  prod:
    policy: manual
    promoteFrom: dev            # 昇格元環境。--from 省略時のデフォルト
    gcp:
      project: my-app-prod
      region: asia-northeast1
    services: environments/prod
    imageCopy:                  # 環境ごとに Artifact Registry が分かれている場合のみ指定
      to: asia-northeast1-docker.pkg.dev/my-app-prod/images
```

バリデーションルール:

- `version` は `1` のみ受理
- `policy: auto` の環境には `branch` 必須
- `policy: manual` の環境には `promoteFrom` を推奨(なければ `promote` 時に `--from` 必須)
- 全環境が `auto` の構成は warning(昇格の概念が消えるため)

### 1.2 サービスマニフェスト(環境ディレクトリ内、1 ファイル = 1 サービス)

Knative YAML ではなく、Cloud Run Admin API v2 の `Service` proto に素直にマップする独自の最小スキーマ。

```yaml
name: my-app
image: asia-northeast1-docker.pkg.dev/my-app-dev/images/my-app@sha256:abc123...
env:
  - name: LOG_LEVEL
    value: debug
resources:
  cpu: "1"
  memory: 512Mi
scaling:
  min: 0
  max: 10
serviceAccount: my-app@my-app-dev.iam.gserviceaccount.com
concurrency: 80          # 省略可
port: 8080               # 省略可(デフォルト 8080)
```

バリデーションルール:

- `image` は **digest 参照(`@sha256:`)のみ受理**。タグ参照はエラー
  (初回セットアップ用に `wataridori resolve` 的な補助は Phase 1 ではやらない。
  digest の初期値はユーザーが `crane digest` 等で書く。README に手順を書く)
- Phase 1 で対応するフィールドは上記のみ。VPC / volumes / secrets 等の追加は Phase 2 以降の Issue で判断

### 1.3 昇格のセマンティクス(最重要の設計判断)

- 昇格で書き写すのは **image の digest 部分だけ**。イメージパス(レジストリ/リポジトリ名)は
  昇格先マニフェストのものを維持する(`imageCopy` 構成では環境ごとにパスが異なるため)
- `env` / `resources` / `scaling` 等の設定値は**書き写さない**。これらは環境固有の設定であり、
  変更したければ各環境のマニフェストを直接編集して `apply` する

## 2. コマンド仕様

共通フラグ: `--repo <path>`(マニフェストリポジトリのルート。デフォルト: カレントから上方探索で
`wataridori.yaml` を見つける)、`--json`(機械可読出力)

終了コード: `0` 成功 / `1` エラー / `2` ドリフト検知(`status --check` のみ)

### 2.1 `wataridori apply --env <env> [--service <name>] [--dry-run]`

マニフェストどおりに Cloud Run へデプロイする。

1. マニフェストを読み込み・検証する
2. 対象サービスごとに Cloud Run の現状を取得し、差分があれば `UpdateService`(なければ `CreateService`)
3. 新リビジョンが Ready になるまで待機(タイムアウト: `--timeout`、デフォルト 5 分)
4. 結果(サービス名・リビジョン名・digest)を出力し、SQLite に履歴を記録する

- `--dry-run`: 差分(現状 → あるべき状態)の表示のみ。API の書き込みは行わない
- 失敗時: Cloud Run 側のエラー(Ready にならない理由)をそのまま表示する。自動ロールバックはしない(将来対応)

### 2.2 `wataridori promote --to <env> [--from <env>] [--service <name>] [--yes]`

昇格元マニフェストの digest を昇格先マニフェストへ書き写し、commit を作る。

1. `--from` 省略時は昇格先の `promoteFrom` を使う
2. 昇格先が `policy: auto` の場合はエラー(自動追従環境への手動昇格は矛盾)
3. 対象サービスごとに from/to の digest を比較し、差分を表示して確認プロンプト(`--yes` でスキップ)
4. `imageCopy` が設定されていれば、go-containerregistry で **digest 指定のイメージコピー**を
   先に実行する(コピー後の digest が一致することを検証)
5. 昇格先マニフェストの digest を書き換え、規約化されたメッセージで **git commit を作成する**
   (例: `promote(prod): my-app to sha256:abc123 (from dev)`)
6. **push / PR 作成はしない**。ユーザーが自分のフローで push する(CI 連携・PR 化は Phase 2)
7. commit ID と共に SQLite に履歴を記録する

- working tree が dirty な場合(対象ファイル以外に変更がある場合)はエラーにして安全側に倒す
- 差分がない(既に同じ digest)場合はその旨を表示して正常終了
- **注意: promote はマニフェストを書き換えるだけで、デプロイはしない。** 反映は commit 後の
  `apply --env prod`(または Phase 2 のコントローラ)が行う。この分離が GitOps の核

### 2.3 `wataridori rollback --env <env> [--service <name>] [--yes]`

Cloud Run のリビジョン保持を使い、前リビジョンへトラフィックを 100% 戻す。

1. 対象サービスのリビジョン一覧を取得し、現在 100% を受けているリビジョンの直前の Ready な
   リビジョンを特定する(`--revision <name>` で明示指定も可)
2. 対象リビジョンと digest を表示して確認プロンプト
3. `traffic` を対象リビジョン 100% に更新する
4. SQLite に履歴を記録する

- **rollback は Cloud Run 上の操作のみで、マニフェストは書き換えない。** 結果として
  マニフェストと実態がずれる(ドリフト)。コマンド完了時に
  「マニフェストは古い digest のままです。恒久化するにはマニフェストを更新してください」と警告を出す。
  `status` はこのドリフトを表示する
- 戻れる Ready リビジョンがなければエラー

### 2.4 `wataridori status [--env <env>] [--check]`

環境 × サービスの「あるべき状態(Git)」と「実際の状態(Cloud Run)」を突き合わせて一覧する。

```
ENV   SERVICE  DESIRED (manifest)  ACTUAL (Cloud Run)  REVISION        STATUS
dev   my-app   sha256:abc123…      sha256:abc123…      my-app-00042    ✓ in sync
prod  my-app   sha256:def456…      sha256:9999ff…      my-app-00017    ✗ drift
```

- `--check`: ドリフトがあれば終了コード `2`(CI での検証用)
- Cloud Run 側にサービスが存在しない場合は `not deployed` と表示する

### 2.5 `wataridori history [--env <env>] [--limit N]`

SQLite に記録した操作履歴を新しい順に表示する。

```
TIME              ACTOR           ACTION    ENV   SERVICE  DIGEST          DETAIL
2026-07-07 10:12  arima@…         promote   prod  my-app   sha256:abc123…  from dev, commit 1a2b3c
2026-07-07 09:58  arima@…         apply     dev   my-app   sha256:abc123…  revision my-app-00042
```

## 3. 履歴ストア(SQLite)

- 置き場所: `~/.local/share/wataridori/history.db`(`--db` / `WATARIDORI_DB` で変更可)
- Phase 1 ではローカル記録。チーム共有の監査ログは Phase 2 のサーバで実現する(Git の commit
  履歴が promote の一次記録なので、Phase 1 でもチームで最低限の追跡は可能)

```sql
CREATE TABLE history (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  ts         TEXT NOT NULL,               -- RFC3339
  actor      TEXT NOT NULL,               -- ADC の principal(取得できなければ OS ユーザー)
  action     TEXT NOT NULL,               -- apply | promote | rollback
  env        TEXT NOT NULL,
  service    TEXT NOT NULL,
  digest     TEXT NOT NULL,
  detail     TEXT                          -- JSON(commit ID, from env, revision 名など)
);
```

## 4. 受け入れ基準(Phase 1 完了の定義)

examples/ のサンプルリポジトリと実 GCP プロジェクト 2 つ(dev / prod)を使い、以下が通ること:

1. `apply --env dev` で新規サービスが作成され、Ready になる
2. dev のマニフェストの digest を更新 → `apply --env dev` で新リビジョンがデプロイされる
3. `promote --to prod` で prod マニフェストに digest が書き写され、commit が作られる
   (AR 分離構成ではイメージコピーも行われる)
4. `apply --env prod` で prod に同一 digest がデプロイされる
5. `rollback --env prod` で前リビジョンにトラフィックが戻り、`status` がドリフトを表示する
6. `history` に上記の全操作が記録されている
7. README の quickstart どおりに新規ユーザーが 10 分以内に 1〜4 を再現できる
