# Security policy

## Supported versions

Wataridori has not published a stable release. Until `v0.1.0`, only the current
`master` branch receives security fixes.

| Version | Supported |
|---|---|
| `master` | Yes |
| untagged historical commits | No |
| pre-release local builds | Best effort |

## Report a vulnerability

Do not report vulnerabilities in a public issue or discussion.

Use the repository's
[private vulnerability reporting](https://github.com/Retr0413/wataridori/security/advisories/new)
form. Include:

- affected version or commit
- impact and attack prerequisites
- reproduction steps or a minimal proof of concept
- suggested mitigation, if known

Remove credentials, personal data, and production resource identifiers from the
report.

The maintainer will acknowledge a report as availability permits, validate the
impact, prepare a fix, and coordinate disclosure. This is a volunteer project
and does not currently offer a response-time SLA.

## Deployment warning

`wataridori serve` does not yet implement application-level authentication or
authorization. Do not expose it directly to the public internet. Use localhost
for development or an authenticated proxy such as Google Cloud IAP.

The default SQLite history store is local and is not a durable shared audit
store for a multi-instance Cloud Run deployment.

## Credential handling

Wataridori relies on Application Default Credentials and registry or Git
credentials supplied by the operator.

- use least-privilege service accounts
- prefer Workload Identity to long-lived service-account keys
- provide tokens through the runtime secret mechanism
- never place credentials in manifests, command output, issues, or test fixtures
- rotate any credential that may have been committed or logged

## Scope

Examples of security issues:

- authentication or authorization bypass
- command or path traversal
- unintended mutation outside the manifest repository
- unsafe handling of Git or registry credentials
- digest validation bypass
- rollback or approval integrity failure
- cross-site scripting in the embedded Web UI

General bugs and feature requests may use the public issue templates.
