# examples

Wataridori のマニフェストリポジトリのサンプル。quickstart と受け入れテストに使う。

| ディレクトリ | 構成 |
|---|---|
| [simple/](simple/) | dev / prod が同じ Artifact Registry を共有する最小構成 |
| [split-registry/](split-registry/) | 環境ごとに AR が分かれ、昇格時にイメージコピーが走る構成 |

## 使い方

1. どちらかのディレクトリを自分の Git リポジトリにコピーする
2. `wataridori.yaml` の `gcp.project` / `gcp.region` と、各サービスマニフェストの
   `image` / `serviceAccount` を自分の環境に書き換える
3. `image` の digest は [crane](https://github.com/google/go-containerregistry/tree/main/cmd/crane) で取得する:

   ```sh
   crane digest asia-northeast1-docker.pkg.dev/YOUR_PROJECT/images/hello:latest
   ```

4. デプロイと昇格:

   ```sh
   wataridori apply --env dev     # dev へデプロイ
   wataridori promote --to prod   # digest を prod マニフェストへ昇格(commit が作られる)
   wataridori apply --env prod    # prod へデプロイ
   wataridori status              # Git と Cloud Run の突き合わせ
   ```

## 環境ごとに Cloud Run service 名が違う場合

`my-api-dev` / `my-api-prod` のように service 名へ環境を埋め込んでいる構成
(1 プロジェクトに dev と prod を並べると起きやすい)では、`name` と
`cloudRunName` を書き分ける。

```yaml
# environments/dev/my-api.yaml
name: my-api             # 環境をまたいだ同一性。promote はこの名前で対応付ける
cloudRunName: my-api-dev # 実際の Cloud Run service 名
image: ...
```

`name` を揃えないと `promote` は
`service "my-api-prod" exists in "prod" but not in "dev"` で失敗する。

## Secret Manager 参照の env

`apply` は service を全置換するため、実際に動いている env はすべてマニフェストに
書き切る必要がある。Secret Manager 参照は `secret` で書く。

```yaml
env:
  - name: LOG_LEVEL
    value: debug
  - name: JWT_SECRET
    secret: my-api-jwt-dev   # Secret Manager の secret 名
    version: latest          # 省略可
```

書き漏らした env は apply で Cloud Run から消える。既存の service を取り込むときは
`gcloud run services describe <name> --format=json` で現状の env を確認してから書く。
