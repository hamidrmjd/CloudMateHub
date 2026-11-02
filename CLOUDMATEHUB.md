# CloudMateHub

CloudMateHub is this project's customized remote device workspace, built on ShellHub Community v0.27.0. Upstream source attribution and LICENSE.md are retained.

## Interface

- New cloud-and-terminal identity, wordmark, browser title and SVG favicon.
- Teal/slate console palette, revised navigation sizing, and responsive public layouts.
- Split welcome/sign-in experience and redesigned setup with the original account/device flows.
- Visible product copy and downloaded recovery/session filenames use CloudMateHub.
- Upstream onboarding marketing survey disabled in this local deployment.

Npm workspace names and owned assets use CloudMateHub. Backend API routes, Go module paths, agent environment variables, webhook headers and browser storage identifiers retain upstream compatibility. Existing devices, accounts, volumes and signing keys are retained.

Run `./deployments/cloudmatehub start` and open http://localhost:8080. The deployment command is `deployments/cloudmatehub`; `deployments/shellhub` remains a compatibility alias. The historical baseline report documents the original release, before this customization.

This redesign does not add Cloud/Enterprise backend capabilities. Authenticated device workflows still require account setup and device enrollment.

## Validation

Production UI build and TypeScript checking passed. Six existing test files passed **102 tests**, covering login, setup, app navigation and terminal themes. The live login page was visually checked at normal and 390px phone widths. Signed-in device/terminal workflows were not browser-tested. Details are in `deployments/cloudmatehub-verification.json`.

## VPN / WireGuard

VPN → WireGuard integrates WGDashboard 4.3.4, personal configuration downloads/QR codes, and instance-admin ownership assignment. See `integrations/wgdashboard/README.md` for feature coverage and Ubuntu requirements. The latest feature checks passed 110 UI tests, 19 broker tests, and 4 Go directory tests. No live test users/demo peers were created; temporary fixture databases were removed, and real data was preserved.
