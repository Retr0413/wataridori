# システムフロー

Wataridori は Cloud Run に特化した GitOps CD ツールであり、中心は
「Git 上のマニフェストに書かれた digest を、Cloud Run の実際の状態へ反映する」ことにある。
このドキュメントでは、OSS 本体がどのように動くかをフローとして整理する。

## フロントエンドの位置付け

フロントエンドは OSS 本体に含める。ただし Phase 1 の必須要件ではなく、Phase 2 以降で
単一 Go バイナリに embed する Web UI として実装する。

理由は以下の通り。

- 昇格・apply・rollback は運用上の重要操作なので、CLI だけではチーム内の可視性が不足しやすい
- 環境ごとの状態、drift、履歴、承認待ちを一覧できる UI は v1.0 の価値に直結する
- 別サービスとして分離すると導入コストが上がるため、Wataridori の「単一バイナリ」という設計価値を損なう
- 先に CLI / core / proto / Connect RPC を固めることで、UI は同じ use case を呼ぶ薄い表示層にできる

したがって、実装方針は「CLI first, API shared, UI embedded」とする。

```mermaid
flowchart LR
    UserCLI["User / CLI"] --> Core["core use cases"]
    UserWeb["User / Web UI"] --> RPC["Connect RPC API"]
    RPC --> Core

    Core --> Git["Git manifests\n(desired state)"]
    Core --> CloudRun["Cloud Run Admin API\n(actual state)"]
    Core --> Registry["Artifact Registry\nimage copy / digest"]
    Core --> Store["SQLite\nhistory / audit records"]

    WebAssets["React + Vite build"] --> Binary["single wataridori binary"]
    RPC --> Binary
    Core --> Binary
```

## 全体モデル

Wataridori が扱う真実は 2 つだけである。

- あるべき状態: Git 上の `wataridori.yaml` と service manifest
- 実際の状態: Cloud Run Admin API から取得する service / revision / traffic

SQLite は履歴・承認記録の補助であり、現在状態の真実にはしない。

```mermaid
flowchart TB
    subgraph Repo["Manifest repository"]
        Config["wataridori.yaml\n- environments\n- policy\n- promoteFrom\n- imageCopy"]
        DevManifest["environments/dev/*.yaml\nimage: repo/app@sha256:dev"]
        ProdManifest["environments/prod/*.yaml\nimage: repo/app@sha256:prod"]
    end

    subgraph Wataridori["wataridori binary"]
        CLI["CLI commands"]
        Server["Connect RPC server"]
        Controller["controller\npolicy:auto reconcile"]
        Core["core use cases"]
    end

    subgraph GCP["Google Cloud"]
        AR["Artifact Registry"]
        CRDev["Cloud Run dev"]
        CRProd["Cloud Run prod"]
    end

    CLI --> Core
    Server --> Core
    Controller --> Core
    Core --> Config
    Core --> DevManifest
    Core --> ProdManifest
    Core --> AR
    Core --> CRDev
    Core --> CRProd
```

## 昇格フロー

昇格はデプロイではない。昇格は、昇格元環境の service manifest にある digest を、
昇格先環境の service manifest に書き写して Git commit を作る操作である。

環境ごとに Artifact Registry が分かれている場合は、書き換え前に digest 指定でイメージをコピーする。
このときコピーされるのはタグではなく digest であり、bit 単位で同一のイメージを保証する。

```mermaid
sequenceDiagram
    autonumber
    actor User
    participant CLI as wataridori promote
    participant Repo as Git manifests
    participant AR as Artifact Registry
    participant Git as Git commit
    participant DB as SQLite history

    User->>CLI: promote --from dev --to prod
    CLI->>Repo: load wataridori.yaml
    CLI->>Repo: load dev/prod service manifests
    CLI->>CLI: validate target policy is manual
    CLI->>CLI: compare dev digest and prod digest

    alt no digest difference
        CLI-->>User: nothing to promote
    else digest differs
        CLI-->>User: show promotion plan
        User->>CLI: confirm

        opt prod imageCopy is configured
            CLI->>AR: copy dev image digest to prod repository
            AR-->>CLI: destination digest verified
        end

        CLI->>Repo: rewrite only prod image digest
        CLI->>Git: create commit
        Git-->>CLI: commit id
        CLI->>DB: record promote history
        CLI-->>User: push commit and apply prod
    end
```

重要な制約:

- `env` / `resources` / `scaling` などの環境固有設定は昇格しない
- 昇格先の image path は維持し、digest だけを更新する
- `promote` は push / PR 作成をしない
- `promote` は Cloud Run にデプロイしない

## Apply フロー

`apply` は Git 上のあるべき状態を Cloud Run に反映する操作である。
service が存在しなければ作成し、存在すれば manifest に合わせて更新する。

```mermaid
sequenceDiagram
    autonumber
    actor User
    participant CLI as wataridori apply
    participant Repo as Git manifests
    participant CR as Cloud Run Admin API
    participant DB as SQLite history

    User->>CLI: apply --env prod
    CLI->>Repo: load environment and service manifests
    CLI->>Repo: validate digest-pinned images
    CLI->>CR: get current Cloud Run service

    alt --dry-run
        CLI-->>User: show create/update/no-op plan
    else service does not exist
        CLI->>CR: CreateService
        CR-->>CLI: wait until ready
        CLI->>DB: record apply history
        CLI-->>User: deployed revision
    else service exists
        CLI->>CR: UpdateService
        CR-->>CLI: wait until ready
        CLI->>DB: record apply history
        CLI-->>User: deployed revision
    end
```

## Status と Drift 検知

`status` は Git の desired digest と Cloud Run の actual serving revision digest を比較する。
Cloud Run service が存在しない場合は `not deployed`、digest が異なる場合は `drift` とする。

```mermaid
flowchart LR
    Start["wataridori status"] --> Load["load manifests"]
    Load --> Desired["desired digest\nfrom Git"]
    Load --> Actual["actual digest\nfrom serving Cloud Run revision"]
    Desired --> Compare{"digest matches?"}
    Actual --> Compare
    Compare -->|yes| InSync["in sync"]
    Compare -->|no| Drift["drift"]
    Actual -->|service missing| Missing["not deployed"]
    Drift --> Check{"--check?"}
    Missing --> Check
    Check -->|yes| Exit2["exit code 2"]
    Check -->|no| Exit0["exit code 0"]
    InSync --> Exit0
```

## Rollback フロー

`rollback` は Git manifest を書き換えない。Cloud Run の revision 履歴を使って、
現在 traffic を受けている revision より古い Ready revision に traffic を 100% 戻す。

そのため rollback 後は、Git の desired digest と Cloud Run の actual digest がずれる可能性がある。
このずれは `status` で drift として表示される。

```mermaid
sequenceDiagram
    autonumber
    actor User
    participant CLI as wataridori rollback
    participant CR as Cloud Run Admin API
    participant DB as SQLite history

    User->>CLI: rollback --env prod
    CLI->>CR: list revisions newest first
    CLI->>CLI: find current serving revision
    CLI->>CLI: find previous Ready revision
    CLI-->>User: show rollback plan
    User->>CLI: confirm
    CLI->>CR: set target revision traffic to 100%
    CR-->>CLI: traffic updated
    CLI->>DB: record rollback history
    CLI-->>User: warn manifest may now drift
```

## Phase 2 Controller フロー

Phase 2 では `policy:auto` の環境を controller が reconcile する。
これは Git の変更を検知して `apply` 相当の処理を走らせるものであり、CI のイメージビルドは扱わない。

```mermaid
flowchart TB
    Trigger["timer / webhook trigger"] --> Refresh["refresh manifest repository"]
    Refresh --> Load["load wataridori.yaml"]
    Load --> Filter["select policy:auto environments"]
    Filter --> Apply["apply each auto environment"]
    Apply --> CR["Cloud Run"]
    Apply --> DB["history"]

    Refresh -->|failed| Skip["skip cycle and keep running"]
    Apply -->|one env failed| Continue["log error and continue next env"]
```

## 推奨する利用フロー

Phase 1 の基本運用は以下の通り。

```mermaid
flowchart TD
    Build["External CI builds image\n(out of scope)"] --> Digest["User writes digest to dev manifest"]
    Digest --> ApplyDev["wataridori apply --env dev"]
    ApplyDev --> VerifyDev["verify dev service"]
    VerifyDev --> Promote["wataridori promote --to prod"]
    Promote --> Push["git push promotion commit"]
    Push --> ApplyProd["wataridori apply --env prod"]
    ApplyProd --> Status["wataridori status --check"]
    Status --> History["wataridori history --env prod"]

    ApplyProd --> Rollback["wataridori rollback --env prod"]
    Rollback --> Drift["status shows drift until manifest catches up"]
```
