# Repository Guidelines

## Project Structure & Module Organization
This repository is a Wails desktop app for managing Codex `auth.json` profiles on Windows. Root Go files (`main.go`, `app.go`, `tray.go`, `api_types.go`) wire the app, tray, and Wails bindings. Business logic lives under `internal/` by domain: `config`, `profile`, `switcher`, `detector`, `audit`, and small helpers such as `fsx`, `paths`, and `util`. The React + TypeScript UI is in `frontend/src`; static assets and build config live in `frontend/`. Build outputs are written to `build/`. OMX planning and local workflow state live under `.omx/` and should not be treated as application source.

## Build, Test, and Development Commands
- `wails dev` — run the desktop app in development mode with live frontend rebuilds.
- `cd frontend; npm install` — install UI dependencies.
- `cd frontend; npm run build` — type-check and build the frontend bundle.
- `go test ./...` — run all Go unit tests.
- `wails build` — create a production Windows executable at `build/bin/codex-profile-manager.exe`.

## Coding Style & Naming Conventions
Use Go defaults: tabs, `gofmt` formatting, exported names in PascalCase, internal helpers in camelCase. Keep packages focused by domain and prefer small service types over cross-package globals. In React/TypeScript, use functional components, 2-space indentation, and camelCase for variables/functions. Keep CSS in `frontend/src/App.css` consistent with existing variable-based theming; extend existing tokens before adding new one-off colors.

## Testing Guidelines
Go tests live next to the code as `*_test.go` (for example, `internal/profile/service_test.go`). Name tests with `TestXxx` and target behavior, not implementation details. Before submitting changes, run `go test ./...` and `cd frontend; npm run build`. For UI-facing changes, include a screenshot or short note describing the visual result.

## Commit & Pull Request Guidelines
Local Git history is not available in this workspace, so use clear imperative commits such as `feat: add tray quick switch` or `fix: preserve active profile on duplicate`. Keep commits scoped and reviewable. PRs should include: a short summary, affected areas, verification steps, linked issue/task if any, and screenshots for UI changes.

## Security & Configuration Notes
Profiles are intentionally stored in plaintext. Never commit real `auth.json` contents, tokens, or local AppData files. Test with sanitized sample data only.
