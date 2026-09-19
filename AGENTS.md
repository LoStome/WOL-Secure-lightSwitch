# Repository Guidelines

## Project Structure & Module Organization

- `backend/` contains the Go server. `main.go` defines routes and startup; the other Go files contain authentication, persistence, ping, SSH, and WOL logic.
- `frontend/src/` contains the React/TypeScript UI. Put HTTP logic in `services/`, UI in `components/`, and global styling in `index.css`.
- `data/hosts.yaml.example` documents device configuration. Runtime files such as `data/hosts.yaml` and `data/secure-switch.db` must remain untracked.
- `Dockerfile` builds both applications; `docker-compose.yml` defines deployment. README images live under `assets/readMe/`.

## Build, Test, and Development Commands

Run backend commands from `backend/`:

- `go run .` starts the API on port `7500` by default.
- `go test ./...` compiles packages and runs tests.
- `go vet ./...` runs standard static checks.
- `gofmt -w *.go` formats Go source before committing.

Run frontend commands from `frontend/`:

- `npm ci` installs the locked dependency set.
- `npm run dev` starts Vite with `/api` proxied to port 7500.
- `npm run build` type-checks and creates `dist/`.
- `npm run lint` runs ESLint.

Use `docker compose config --quiet` to validate Compose and `docker compose up --build` for an integrated local deployment.

## Coding Style & Naming Conventions

Use `gofmt` and idiomatic Go error handling. Exported identifiers use `PascalCase`; internal identifiers use `camelCase`. Keep HTTP handlers thin.

React components and files use `PascalCase`; hooks, functions, and variables use `camelCase`. Prefer explicit TypeScript types over `any`, and centralize API calls in `src/services/api.ts`.

## Testing Guidelines

No automated tests are currently committed. Add Go tests as `*_test.go` and frontend tests as `*.test.tsx` if a runner is introduced. Prioritize authorization, initial-admin setup, malformed YAML, WOL packets, and SSH failures. Never contact real LAN devices or overwrite `data/`; use temporary databases and fake network dependencies. Include a regression test with bug fixes.

## Commit & Pull Request Guidelines

History uses short summaries in English and Italian; prefer imperative English, for example `Fix SSH command timeout` or `feat: add device validation`.

Pull requests should explain the behavior change, security impact, and validation commands run. Link relevant issues and include screenshots for visible frontend changes. Do not commit generated builds, credentials, private keys, runtime databases, or a real `hosts.yaml`.

## Security & Configuration Tips

Set a long random `JWT_SECRET`; never rely on the development fallback. Prefer dedicated SSH keys and narrowly scoped sudoers rules. Preserve server-side role and device-assignment checks for every device action.

## Improvement Plan

- Before starting work, read `docs/ANALISI.md`.
- Work on one issue at a time, identified by its stable ID.
- After each fix, run the relevant tests, update the issue status in `docs/ANALISI.md`, and create a separate commit containing the issue ID in its message.
- Do not modify anything outside the requested issue's scope.

The report lists unresolved vulnerabilities. Committing and pushing it would disclose them before they are fixed. Use one of these approaches:
