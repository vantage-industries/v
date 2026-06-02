# VantageOS Control Plane

This directory contains the standalone web UI and local backend for VantageOS.

## Layout

- `frontend/` - Vite + React SPA
- `backend/` - Go API and file-backed state store
- `deploy/` - nginx and systemd integration snippets
- `flake.nix` - Nix package and dev-shell entrypoint

## Launch Scope

- 5 GHz only
- `Main` and `IoT` zones
- per-device PSKs
- secure QR onboarding with optional fallback credential

## Development

- Frontend: `pnpm --dir frontend dev`
- Frontend build: `pnpm --dir frontend build`
- Backend test: `go test ./backend/...`
- Backend runtime: `go run ./backend/cmd/vantageos-api`
- Flake build: `nix build .#backend`
- Flake shell: `nix develop`

## Yocto

The Yocto layer consumes this directory through the `vantageos-control-plane` recipe.
