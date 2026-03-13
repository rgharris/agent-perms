agent-perms currently supports gh, git, go, kubectl, and pulumi. Here are the best candidates to add
next, ranked by impact:

## Top Candidates

### 1. docker — Clear security boundary

  - read: docker ps, docker images, docker inspect, docker logs
  - write:local: docker build, docker run, docker pull
  - admin:local: docker rm, docker rmi, docker system prune
  - admin:remote: docker push

### 2. Cloud CLIs (aws, az, gcloud) — High blast-radius

  These have the most dangerous write/admin tiers:
  - read: aws s3 ls, gcloud projects list
  - write:remote: aws s3 cp, gcloud deploy
  - admin:remote: aws s3 rm, gcloud projects delete

### 3. npm/npx/yarn — High frequency

  - read: npx tsc --noEmit, yarn lint, npm run lint, npm ls
  - write:local: npm install, yarn install, npm run build, yarn build
  - admin:remote: npm publish, yarn publish

### 4. make — Tricky but valuable

  Very high frequency. Tiering would need to be target-based or passthrough (classify by what the
  target runs). Could start simple:
  - read: make lint, make test, make check
  - write:local: make build, make format, make ensure, make mocks
  - admin:local: make clean

### 5. golangci-lint — Common in Go projects

  - read: golangci-lint run
  - write:local: golangci-lint run --fix
  - admin:local: golangci-lint cache clean

### 6. uv (Python)

  - read: uv run -m pytest, uv run -m ruff check, uv run -m mypy
  - write:local: uv sync, uv pip install, uv run -m ruff format
  - admin:remote: uv run twine upload

## Future Agent Platforms

agent-perms currently targets Claude Code and Codex CLI. These platforms have
weaker built-in command classification and represent expansion opportunities:

### Cursor

  - Has `beforeShellExecution` hooks and `.cursor/rules/` instruction files
  - Lacks robust OS-level sandboxing (confirmed by 2026 security analyses)
  - No semantic command classification
  - Integration path: instruction-based (like Codex AGENTS.md) + hook-based

### Windsurf

  - Has admin controls for enterprise
  - Weak built-in granularity for CLI command classification
  - Integration path: instruction-based

### Gemini CLI

  - Has `tools.core` prefix restriction and Docker/Seatbelt sandbox
  - `tools.core` is strong at the binary level but lacks subcommand+flag awareness
  - Integration path: instruction-based + sandbox complement
