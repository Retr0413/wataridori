# Protobuf API

`proto/` is the single source of truth for the Connect RPC API.

`wataridori/v1/wataridori.proto` defines `DeploymentService`, including:

- environment discovery
- status and inventory
- apply
- promotion plan and execution
- rollback plan and execution
- activity history
- Cloud Run revision timeline

## Generate code

Generated Go code is committed in `gen/`; generated TypeScript is committed in
`web/src/gen/`.

```sh
make tools
make gen
```

CI runs `make gen-check` and fails if generated output is stale.

Configuration:

- [`buf.yaml`](../buf.yaml): lint and breaking-change policy
- [`buf.gen.yaml`](../buf.gen.yaml): Go and TypeScript generation
