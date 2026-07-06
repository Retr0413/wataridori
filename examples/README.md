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
