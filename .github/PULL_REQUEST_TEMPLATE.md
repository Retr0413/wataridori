## Summary

<!-- What changed and why? -->

## Impact

<!-- Describe user, operator, compatibility, security, or migration impact. -->

## Validation

<!-- List the exact checks and acceptance tests you ran. -->

- [ ] `go test ./...`
- [ ] `npm --prefix web run typecheck` when Web code changed
- [ ] `npm --prefix web run build` when Web code changed
- [ ] `npm --prefix web run test:e2e` when user behavior changed
- [ ] `make gen-check` when protobuf or generated code may be affected

## Checklist

- [ ] The change preserves digest-based promotion and the GitOps state model.
- [ ] Documentation covers new behavior and failure cases.
- [ ] Generated code and `web/dist/` are current when applicable.
- [ ] No credentials, private URLs, production identifiers, or local state are included.
- [ ] Security-sensitive behavior includes negative tests or an explanation.
