# VantageOS Implementation Checklist

> **Superseded.** This tracker is for the original `control-plane/` app described in
> `architecture.md`, retired in favor of **security-hub**. Kept as a historical record; none of
> these items are being worked toward any more. security-hub tracks its own progress in
> `v/security-hub/backend/SYSTEM-INTEGRATION.md` (Steps 1-9) inside the submodule.

This is the working tracker for implementation. Check items off only after they are built and verified.

## Scope Freeze

- [ ] Keep v1 limited to Raspberry Pi 5.
- [ ] Keep v1 limited to 5 GHz Wi-Fi.
- [ ] Keep 2.4 GHz legacy support for phase 2.
- [ ] Keep USB gadget debug-only.
- [ ] Keep NetworkManager out of the product image.
- [ ] Keep `control-plane/` as the application boundary.
- [ ] Keep `Main` and `IoT` as the initial zone model.
- [ ] Keep per-device PSKs mandatory.
- [ ] Keep onboarding separate from operational mode.
- [ ] Keep the router local-first.

## Boot and Onboarding State Machine

- [x] Add a persisted bootstrap state record.
- [x] Add an explicit `first_run` state.
- [x] Add an explicit `onboarding_portal` state.
- [x] Add an explicit `wan_probe` state.
- [x] Add an explicit `guided_setup` state.
- [x] Add an explicit `apply_pending` state.
- [x] Add an explicit `verify_candidate` state.
- [x] Add an explicit `operational` state.
- [x] Add an explicit `recovery` state.
- [x] Seed bootstrap state on first boot only.
- [x] Stop using device count as the first-run heuristic.
- [x] Persist state transitions durably.
- [x] Make transitions idempotent.
- [x] Expose setup state in the bootstrap API.
- [x] Expose setup state in the status API.
- [x] Resume the correct state after reboot.

## Setup Access and Captive Portal

- [ ] Create a dedicated onboarding SSID.
- [ ] Keep the onboarding AP up regardless of WAN status.
- [x] Serve the onboarding portal over HTTP.
- [x] Add captive portal detection responses for major OSes.
- [x] Add an explicit local URL such as `vantageos.local`.
- [x] Add a fallback IP-based URL in the UI.
- [x] Add onboarding DNS behavior that stays local-only.
- [x] Keep the setup portal isolated from WAN traffic.
- [x] Show clear first-connection instructions in the portal.
- [ ] Disable the onboarding SSID after successful setup.

## WAN Handling

- [ ] Mark WAN optional for boot.
- [ ] Add `RequiredForOnline=no` for the WAN interface.
- [ ] Remove global `network-online.target` as a setup blocker.
- [ ] Add a non-blocking WAN probe service.
- [ ] Match WAN by explicit path or MAC.
- [ ] Detect carrier separately from routability.
- [ ] Support DHCP WAN as the default.
- [ ] Support static WAN as an advanced option.
- [ ] Defer PPPoE to later or advanced mode.
- [ ] Record WAN state in the UI.
- [ ] Retry WAN detection on link events.
- [ ] Keep onboarding local even when WAN is absent.
- [ ] Queue WAN-dependent tasks until WAN is available.

## Recovery and GPIO Reset

- [ ] Choose the reset GPIO pin.
- [ ] Add a `gpio-keys` device tree entry.
- [ ] Add a debounce interval for the reset button.
- [ ] Add a long-press threshold in userspace.
- [ ] Add LED feedback while the button is held.
- [ ] Add a reset handler daemon.
- [ ] Make reset wipe writable state only.
- [ ] Return to onboarding after reset.
- [ ] Add a recovery AP profile.
- [ ] Keep the recovery AP isolated from the normal zones.
- [ ] Keep the recovery portal minimal.
- [ ] Keep recovery independent of WAN and main config.
- [ ] Document Pi EEPROM recovery as the last resort.

## Network Zones and Wi-Fi

- [x] Define the `Main` subnet.
- [x] Define the `IoT` subnet.
- [ ] Add per-zone firewall policy.
- [x] Add per-device credential storage.
- [x] Add secure QR payload generation.
- [x] Add temporary fallback credential generation.
- [x] Store credential IDs and metadata.
- [x] Record secure-enrolled state.
- [x] Record fallback-enrolled state.
- [ ] Define stable-join criteria.
- [ ] Add hostapd auth attribution hooks.
- [ ] Add hostapd config generation from backend state.
- [ ] Keep 5 GHz only in the v1 configuration.
- [ ] Keep legacy 2.4 GHz SSIDs as future-only.
- [ ] Ensure AP startup does not depend on WAN.

## Configuration Lifecycle and Rollback

- [x] Add an immutable config revision table.
- [x] Add a separate active revision pointer.
- [ ] Add a candidate revision or apply-attempt record.
- [ ] Add an apply deadline timer.
- [ ] Add a manual confirm action.
- [ ] Add automatic rollback on missed confirm.
- [ ] Add atomic config file generation.
- [ ] Use `fsync` for file writes.
- [ ] Use `fsync` for directory updates.
- [ ] Validate configs before applying them.
- [ ] Run health checks after applying.
- [x] Keep the previous-good snapshot available.
- [x] Record rollback events.
- [ ] Expose revision history in the UI.
- [ ] Make apply and rollback idempotent.
- [ ] Make revision state testable with injected clock and filesystem.

## Control Plane Backend

- [x] Implement the backend in Go.
- [x] Persist backend state in a file-backed store under `/var/lib`.
- [x] Implement a health endpoint.
- [x] Implement the bootstrap endpoint.
- [x] Implement the status endpoint.
- [x] Implement networks list and edit endpoints.
- [x] Implement devices list and create endpoints.
- [x] Implement credential issuance.
- [x] Implement credential revocation.
- [x] Implement enrollment finalization.
- [x] Implement config history.
- [x] Implement config apply.
- [x] Implement config rollback.
- [x] Implement Tailscale status.
- [x] Implement Tailscale enrollment.
- [x] Implement an event stream.
- [x] Store persistent state under `/var/lib`.
- [x] Return clear validation errors.
- [ ] Add backend request logging.
- [x] Add backend unit tests.

## Frontend Setup UX

- [ ] Keep the frontend in Vite + React.
- [ ] Migrate the final design system to Tailwind and shadcn-style primitives.
- [ ] Add a route-based wizard.
- [ ] Add a welcome screen.
- [ ] Add a WAN status screen.
- [ ] Add a router naming screen.
- [ ] Add a network selection screen.
- [ ] Add a first-device screen.
- [ ] Add a QR/password screen.
- [ ] Add a Tailscale screen.
- [ ] Add an apply and verify screen.
- [ ] Add a finish screen.
- [ ] Add a progress stepper.
- [ ] Add resume-after-reboot support.
- [ ] Add loading states.
- [ ] Add empty states.
- [ ] Add error states.
- [ ] Add mobile layout behavior.
- [ ] Add accessibility semantics.
- [ ] Add confirmation dialogs for destructive actions.
- [ ] Add toast notifications.
- [ ] Add a configuration history timeline.
- [ ] Add device detail views.
- [ ] Add network detail views.
- [ ] Add logs and diagnostics views.
- [ ] Persist setup drafts across reloads.

## Tailscale and Remote Admin

- [ ] Remove plaintext auth keys from the repo.
- [ ] Add a Tailscale enrollment flow.
- [ ] Add a Tailscale disconnect flow.
- [ ] Add a route advertisement option.
- [x] Add Tailscale status reporting in the UI.
- [ ] Keep remote admin off the public WAN.
- [ ] Keep Tailscale optional during first-run setup.
- [x] Surface Tailscale status in the setup checklist.
- [ ] Add a fallback when Tailscale is unavailable.

## Packaging and Yocto Integration

- [ ] Keep the control plane under `control-plane/`.
- [x] Add a Nix flake for the control plane package outputs.
- [x] Add a Nix dev shell for backend and frontend work.
- [x] Build the frontend static bundle in Yocto.
- [x] Install the frontend bundle into the nginx web root.
- [x] Package the backend into the image.
- [x] Install the nginx site config.
- [x] Install the systemd unit for the backend.
- [x] Enable nginx in the image.
- [x] Enable the backend service in the image.
- [x] Keep hostapd service ordering conflict-free.
- [ ] Keep the USB gadget out of production builds.
- [ ] Pin build tool versions.
- [ ] Keep production and development image settings separate.

## Security Hardening

- [ ] Remove the hardcoded root password from the production path.
- [ ] Remove the baked-in SSH key from the production path.
- [ ] Disable root password login.
- [ ] Replace autologin with recovery-only access.
- [ ] Bind the backend API to localhost only.
- [ ] Keep nginx as a local reverse proxy only.
- [ ] Add a default-deny nftables policy.
- [ ] Add per-zone firewall isolation.
- [ ] Keep the onboarding network isolated.
- [ ] Keep management off the public WAN.
- [ ] Use least-privilege service units.
- [ ] Store secrets under `/var/lib`.
- [ ] Add audit logs for security actions.
- [ ] Add explicit confirmation for destructive actions.

## Observability and Verification

- [ ] Add a service health summary.
- [ ] Add a WAN health summary.
- [ ] Add an AP health summary.
- [ ] Add a device enrollment timeline.
- [ ] Add a config revision timeline.
- [ ] Add log filtering and search.
- [ ] Add an exportable diagnostics bundle.
- [ ] Add a first-boot smoke test.
- [ ] Add a WAN-absent boot test.
- [ ] Add a GPIO reset test.
- [ ] Add a recovery AP test.
- [ ] Add a rollback timeout test.
- [ ] Add a secure enrollment test.
- [ ] Add a packaging build test.
- [ ] Add an end-to-end Pi 5 smoke test.
