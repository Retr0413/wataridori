# proto

Connect RPC の proto 定義(API の単一ソース)。message は
[internal/core](../internal/core) の Request/Result 型に 1:1 対応する。

- `wataridori/v1/wataridori.proto` — `DeploymentService`
  (Status / Apply / Promote / Rollback / History)

## コード生成

生成物は [`gen/`](../gen) にコミットしている(下流パッケージが codegen なしで
ビルドできるように)。proto を編集したら再生成すること:

```sh
make tools   # 初回のみ: protoc-gen-go / protoc-gen-connect-go を導入
make gen     # buf lint + buf generate → gen/ を更新
```

CI は `make gen-check` で「コミット済みの生成物が proto と一致しているか」を検証する。

設定: [`buf.yaml`](../buf.yaml)(lint / breaking)、
[`buf.gen.yaml`](../buf.gen.yaml)(生成プラグイン)。
