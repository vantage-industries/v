# VantageOS Backend

Go service and state store for the VantageOS control plane.

## Commands

- Test: `go test ./...`
- Run locally: `go run ./cmd/vantageos-api`
- Build: `go build ./cmd/vantageos-api`
- Nix build: `nix build .#backend`

## Runtime

- Default state path: `/var/lib/vantageos/vantageos-state.json`
- Override with `VANTAGEOS_STATE_PATH`
