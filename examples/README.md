# Manifest examples

These directories are starting points for a separate Wataridori manifest
repository.

| Directory | Topology |
|---|---|
| [`simple/`](simple/) | Development and production share one Artifact Registry repository |
| [`split-registry/`](split-registry/) | Promotion copies the digest into a production repository |

## Use

1. Copy one directory into a new Git repository.
2. Replace `gcp.project`, `gcp.region`, service accounts, and images.
3. Resolve a digest:

   ```sh
   crane digest REGION-docker.pkg.dev/PROJECT/REPOSITORY/IMAGE:TAG
   ```

4. Store the immutable result as `IMAGE@sha256:...`.
5. Run:

   ```sh
   wataridori apply --env dev
   wataridori promote --to prod
   git push
   wataridori apply --env prod
   wataridori status
   ```

## Different physical service names

Use one logical `name` across environments and set `cloudRunName` separately:

```yaml
name: my-api
cloudRunName: my-api-dev
image: ...
```

Promotion matches `name`, not `cloudRunName`.

## Secret Manager environment variables

```yaml
env:
  - name: LOG_LEVEL
    value: debug
  - name: JWT_SECRET
    secret: my-api-jwt-dev
    version: latest
```

Apply manages the supported environment-variable set declaratively. Include
every required value or secret reference in the manifest.
