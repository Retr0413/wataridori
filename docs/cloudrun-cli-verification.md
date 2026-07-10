# Cloud Run CLI Verification

最終確認日: 2026-07-10 JST

この文書は、Wataridori の CLI が Cloud Run API / Artifact Registry を使って
dev から prod へ digest ベースで昇格できるかを確認した記録である。

## 対象環境

| 項目 | 値 |
|---|---|
| GCP project | `helical-parity-501908-q1` |
| Service name | `wataridori-hello` |
| dev Cloud Run | `asia-northeast1` |
| prod Cloud Run | `asia-northeast2` |
| dev image repository | `asia-northeast1-docker.pkg.dev/helical-parity-501908-q1/wataridori-dev-images` |
| prod image repository | `asia-northeast1-docker.pkg.dev/helical-parity-501908-q1/wataridori-prod-images` |
| manifest repo used for verification | `infra/test-gcp/generated` |

dev / prod は同じ Cloud Run service name を使うため、1 project 内で重複を避ける目的で
region を分けている。Wataridori の `promote` は service name で昇格元と昇格先を対応付ける。

## 検証結果サマリ

2026-07-09 の検証では、以下の一連のフローが成功した。

1. `wataridori status` で dev / prod がそれぞれ Cloud Run の実 revision と同期していることを確認
2. `wataridori promote --to prod` で dev の image digest を prod manifest へ反映
3. prod 用 Artifact Registry repository へ同じ digest の image をコピー
4. `wataridori apply --env prod` で prod Cloud Run を dev と同じ digest へ更新
5. prod endpoint が `version: dev-v2` を返すことを確認
6. `wataridori rollback --env prod` で prod を前 revision に戻す
7. prod endpoint が `version: prod-v1` を返すことを確認
8. `wataridori history --env prod` に `promote` / `apply` / `rollback` が記録されることを確認

この時点では、Wataridori の CLI は Cloud Run API を使った dev -> prod 昇格、
prod への適用、rollback の最小フローを実行できていた。

## 2026-07-10 時点の再確認結果

2026-07-10 に同じ環境を再確認したところ、GCP project の Billing が無効になっていた。

```json
{
  "billingEnabled": false,
  "projectId": "helical-parity-501908-q1"
}
```

そのため、Cloud Run service / revision の read は可能だが、Artifact Registry の image 確認、
Cloud Run の更新、Cloud Run instance の起動は GCP 側で拒否された。

### 成功した確認

`status` は Cloud Run API から現在 revision と image digest を取得できた。

```text
ENV   SERVICE           DESIRED             ACTUAL              REVISION                    STATUS
dev   wataridori-hello  hello@201ec8f48f37  hello@201ec8f48f37  wataridori-hello-00001-brh  in sync
prod  wataridori-hello  hello@f5f6775f59e7  hello@f5f6775f59e7  wataridori-hello-00001-9m2  in sync
```

Cloud Run revision も Ready として取得できた。

```text
prod current revision: wataridori-hello-00001-9m2
prod promoted revision: wataridori-hello-00002-5bw
```

### Billing 無効により失敗した確認

HTTP endpoint は Cloud Run instance 起動時に 503 を返した。

```text
Error: Server Error
The service you requested is not available yet.
```

Cloud Logging では以下の理由が記録されていた。

```text
The request failed because billing is disabled for this project.
```

`promote` は prod 側 Artifact Registry の digest 確認時に失敗した。

```text
DENIED: This API method requires billing to be enabled.
```

`apply --env prod` は Cloud Run service 更新時に失敗した。

```text
rpc error: code = PermissionDenied desc = This API method requires billing to be enabled.
reason = BILLING_DISABLED
service = run.googleapis.com
```

## 実行コマンド

通常の full-path 検証は以下で行う。

```bash
go run ./cmd/wataridori status \
  --repo infra/test-gcp/generated \
  --db /tmp/wataridori-gcp-cli-test.db

go run ./cmd/wataridori promote \
  --repo infra/test-gcp/generated \
  --to prod \
  --yes \
  --db /tmp/wataridori-gcp-cli-test.db

go run ./cmd/wataridori apply \
  --repo infra/test-gcp/generated \
  --env prod \
  --db /tmp/wataridori-gcp-cli-test.db

go run ./cmd/wataridori rollback \
  --repo infra/test-gcp/generated \
  --env prod \
  --yes \
  --db /tmp/wataridori-gcp-cli-test.db

go run ./cmd/wataridori history \
  --repo infra/test-gcp/generated \
  --env prod \
  --db /tmp/wataridori-gcp-cli-test.db
```

HTTP で runtime まで確認する場合は以下を使う。

```bash
curl -sS https://wataridori-hello-z6qqrxowsq-an.a.run.app/
curl -sS https://wataridori-hello-z6qqrxowsq-dt.a.run.app/
```

## 判断

- Wataridori CLI の `status` は現時点でも Cloud Run API に対して動作している。
- 2026-07-09 の検証では、`promote` / `apply` / `rollback` は実 Cloud Run 環境で成功している。
- 2026-07-10 時点では GCP project の Billing が無効なため、同じ full-path 検証は再実行できない。
- Billing を再度有効化すれば、上記の full-path コマンドで dev -> prod 昇格と rollback を再確認する。

