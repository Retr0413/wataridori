# Release process

Wataridori releases binaries through GitHub Releases and container images
through GitHub Container Registry. Releases are maintainer operations, not an
automatic consequence of merging a pull request.

## Release contract

- Release tags use Semantic Versioning, such as `v0.1.0` or `v0.1.0-rc.1`.
- The tagged commit must be contained in `master`.
- GoReleaser and every GitHub Action are pinned to reviewed versions.
- Release archives include SHA-256 checksums and SPDX JSON SBOMs.
- Container images use an immutable version tag. Wataridori does not publish a
  mutable `latest` tag.
- GitHub Releases are created as drafts and require a final maintainer review.

## Prepare

1. Confirm all intended pull requests are merged and required checks pass.
2. Update `CHANGELOG.md` for the release.
3. Run the same checks used by pull requests:

   ```sh
   go test ./...
   npm --prefix web run typecheck
   npm --prefix web run build
   make gen-check
   ```

4. Confirm the release snapshot check passes on the final pull request.
5. Pull the latest `master` and ensure the worktree is clean.

## Create

Create an annotated tag at the reviewed `master` commit and push only that tag:

```sh
git switch master
git pull --ff-only origin master
git tag -a v0.1.0 -m "Wataridori v0.1.0"
git push origin v0.1.0
```

The release workflow rejects tags that are not SemVer or do not point to a
commit contained in `master`.

## Review and publish

The workflow creates a draft GitHub Release. Before publishing it:

1. Confirm every expected OS and architecture archive is present.
2. Download an archive and verify it against `checksums.txt`.
3. Confirm each archive has a corresponding SBOM.
4. Inspect the generated release notes and `CHANGELOG.md`.
5. Confirm the versioned GHCR image exists and record its digest.
6. Smoke-test `wataridori version` and one non-mutating CLI command.
7. Publish the draft release manually.

## Failed releases

Do not move or reuse a published version tag. If released artifacts are
incorrect, mark the GitHub Release as affected, document the impact, and publish
a new patch version. A failed draft may be deleted before publication, but its
tag should only be recreated when no artifacts were made public.
