# VantageOS Architecture and Implementation Plan

## Goal

VantageOS is a security-focused router appliance for Raspberry Pi 5.

## Launch Scope

- 5 GHz only at launch
- 2.4 GHz legacy support planned later
- no NetworkManager
- USB gadget remains debug-only

## Chosen Stack

- Frontend: Vite + React
- Package manager: pnpm for the frontend, Nix flakes for packaging
- Web server: nginx
- Backend: Go
- State store: file-backed JSON under `/var/lib`
- Router control plane: systemd-networkd, hostapd, nftables, dnsmasq
- Remote admin: Tailscale

## Repository Boundary

- The UI/backend application code lives in `control-plane/`.
- The Yocto repo only carries integration recipes, service manifests, and deployment wiring.
- The control-plane app is a normal in-repo subdirectory, not a git submodule.

## Directory Layout

- `control-plane/frontend/` - Vite + React SPA source
- `control-plane/backend/` - Flask API and SQLite store
- `control-plane/deploy/` - nginx and systemd snippets
- `layers/meta-vantageos/` - Yocto integration layer

## Network Model

- `Main` = trusted personal devices
- `IoT` = isolated devices and appliances
- `Main` and `IoT` are separate zones
- later expansion will add `Main-Legacy` and `IoT-Legacy`

## Authentication Model

- one permanent unique credential per device
- optional temporary onboarding fallback credential
- hostapd instrumentation will report which credential was used
- fallback is revoked after stable enrollment

## UI Flow

- dashboard
- add device
- device detail
- network detail
- logs
- settings
- apply / rollback
- guided first-run setup
- configuration history and revision timeline

## UI Style Direction

- Prefer a Tailwind + shadcn-style component system if the UI is rebuilt from scratch.
- Keep the current custom styling only as an interim implementation.
- Use polished primitives for stepper, dialog, card, tabs, dropdown, and toast behavior.

## Runtime Shape

- nginx serves the static SPA
- Go serves the local API on `127.0.0.1`
- SPA never writes config directly
- backend owns validation, rendering, and apply/rollback

## Implementation Order

1. Create the `control-plane/` app directory.
2. Build the Go API, persisted state store, and device credential flow.
3. Build the Vite + React SPA.
4. Wire the deployable static build into nginx.
5. Package the backend and frontend with Nix flakes.
6. Integrate the backend service into Yocto.
7. Add hostapd instrumentation and per-device PSK attribution.
8. Wire router services and hardening.
9. Add Tailscale enrollment and rollback flow.
10. Expand to 2.4 GHz legacy support later.

## Yocto Integration

- Build the frontend to static files.
- Install the SPA into the nginx web root.
- Package the backend as a systemd service.
- Keep the control-plane code in `control-plane/` and let Yocto consume its build outputs.

## Working Checklist

- See `docs/implementation-checklist.md` for the granular implementation tracker.
- Check items off there as the system is implemented and verified.
