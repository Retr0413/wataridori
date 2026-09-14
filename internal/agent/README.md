# Agent architecture

This directory contains the framework-free Go implementation of Wataridori's
SRE agent and autonomous recovery loop. It follows a layered architecture so
that model reasoning cannot bypass recovery policy or call production mutation
APIs directly.

## Layers

```text
presentation
    |
    v
application
    |
    v
domain

infrastructure -- implements ports owned by application
```

- `domain` owns recovery states, policies, proposals, authorized plans,
  evidence metadata, action types, and invariants. It has no model, cloud,
  storage, or presentation dependencies.
- `application` owns recovery use cases, agent-loop orchestration, context
  assembly, evidence selection, budgets, and ports for external dependencies.
- `infrastructure` implements application ports for model providers, GitHub,
  Cloud Run and Cloud Logging evidence, Git operations, and durable storage.
- `presentation` exposes the application use cases to the CLI, reusable GitHub
  Actions workflows, the controller, and Connect RPC without containing agent
  or recovery policy logic.

## Dependency rules

1. Dependencies point inward: presentation and infrastructure may depend on
   application and domain; application may depend on domain; domain depends
   only on the Go standard library.
2. Interfaces for external systems are defined by the application layer that
   consumes them. Infrastructure packages implement those interfaces.
3. Existing Wataridori use cases such as rollback and deployment verification
   remain in `internal/core` and are consumed through narrow application ports.
4. Model-provider SDK types must not cross the infrastructure boundary.
5. The composition root remains `cmd/wataridori`, preserving the single-binary
   distribution model.

## Context boundaries

Go's `context.Context` is reserved for cancellation, deadlines, and tracing.
Recovery scope, policy, evidence, actor, and model input are passed as explicit
typed values; they must not be hidden in context values.

The application layer is the only layer allowed to decide what information is
inserted into model context. Context is assembled in this order:

1. versioned system instructions and output contract;
2. immutable recovery scope and observation time;
3. recovery-policy snapshot, allowed tools, allowed actions, and budgets;
4. structured observations from Git and Cloud Run;
5. redacted, source-labelled untrusted evidence such as logs, diffs, pull
   request text, and runbooks;
6. bounded read-only tool results collected during the current recovery run.

Infrastructure adapters serialize this normalized input into provider-specific
requests but must not add instructions or select evidence. Tool results pass
through validation, redaction, provenance labelling, and budget checks before
being appended to model context.

Natural-language model output is never executable. The agent returns a typed
recovery proposal; deterministic application and domain policy produce a
fingerprinted authorized plan; only that plan may be sent to existing core
mutation use cases.

## Initial non-goals

- multi-agent coordination;
- long-term conversational memory or vector search;
- dynamic plugins or tool discovery;
- arbitrary shell, URL, or cloud API tools;
- direct model access to production credentials;
- provider-owned conversation state as the only recovery record.
