<!-- imprint pack (6 rules) -->
---
id: r-2026-09-11-001
claim: This repository is implemented in Go only; do not add Python packages or bindings
scope:
    - go
    - proj:imprint
confidence: 0.95
status: active
reinforcement_count: 1
created_at: 2026-09-11T08:01:38Z
updated_at: 2026-09-11T08:01:38Z
last_touched_at: 2026-09-11T08:01:38Z
supersedes: []
related: []
conflicts_with: []
evidence_log:
    - at: 2026-09-11T08:01:38Z
      kind: original
      text: 没有Python代码，纯Go
    - at: 2026-09-11T08:01:38Z
      kind: reinforce
      text: 'user restated: 没有Python代码，纯Go'
---
---
id: r-2026-09-11-002
claim: Build with CGO_ENABLED=0 as a static binary with no runtime dependencies
scope:
    - go
    - build
confidence: 0.85
status: active
reinforcement_count: 0
created_at: 2026-09-11T08:01:38Z
updated_at: 2026-09-11T08:01:38Z
last_touched_at: 2026-09-11T08:01:38Z
supersedes: []
related: []
conflicts_with: []
evidence_log:
    - at: 2026-09-11T08:01:38Z
      kind: original
      text: Go static binary, zero runtime
---
---
id: r-2026-09-11-003
claim: Pack multiple rules into imprint-NNNN.md shards under memory/; start a new shard at 32768 lines or 1 MiB
scope:
    - imprint
    - storage
confidence: 0.85
status: active
reinforcement_count: 0
created_at: 2026-09-11T08:01:38Z
updated_at: 2026-09-11T08:01:38Z
last_touched_at: 2026-09-11T08:01:38Z
supersedes: []
related: []
conflicts_with: []
evidence_log:
    - at: 2026-09-11T08:01:38Z
      kind: original
      text: User corrections → portable packed markdown
---
---
id: r-2026-09-11-004
claim: CLI flag parsing must stay in the Go standard library; do not add Cobra
scope:
    - go
    - cli
confidence: 0.8
status: active
reinforcement_count: 0
created_at: 2026-09-11T08:01:38Z
updated_at: 2026-09-11T08:01:38Z
last_touched_at: 2026-09-11T08:01:38Z
supersedes: []
related: []
conflicts_with: []
evidence_log:
    - at: 2026-09-11T08:01:38Z
      kind: original
      text: CLI 用标准库 + 轻量子命令解析，避免 Cobra
---
---
id: r-2026-09-11-005
claim: When referring to this system, always call it imprint, never memory store
scope:
    - git
    - branding
confidence: 0.7
status: active
reinforcement_count: 0
created_at: 2026-09-11T08:01:38Z
updated_at: 2026-09-11T08:01:38Z
last_touched_at: 2026-09-11T08:01:38Z
supersedes: []
related: []
conflicts_with: []
evidence_log:
    - at: 2026-09-11T08:01:38Z
      kind: original
      text: always call it imprint
---
---
id: r-2026-09-11-007
claim: Go exported names use PascalCase; unexported names use camelCase
scope:
    - go
    - naming
confidence: 0.6
status: active
reinforcement_count: 0
created_at: 2026-09-11T08:01:38Z
updated_at: 2026-09-11T08:01:38Z
last_touched_at: 2026-09-11T08:01:38Z
supersedes:
    - r-2026-09-11-006
related:
    - r-2026-09-11-006
conflicts_with: []
evidence_log:
    - at: 2026-09-11T08:01:38Z
      kind: original
      text: exported PascalCase, unexported camelCase
    - at: 2026-09-11T08:01:38Z
      kind: supersede
      text: narrowed to exported vs unexported
---
