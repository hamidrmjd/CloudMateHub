# Branding and native SSH approval links

Date: 2026-11-03

## Changes

Project-owned copy, npm workspaces, icon/theme assets, documentation routes, and deployment command use CloudMateHub. The old deployment command remains an alias. Licenses, upstream attribution and published dependency URLs remain intact. Existing Go imports, environment variables, agent/SSH protocol names, installer package identities, browser/vault storage keys and deployed network/volume identities retain compatibility.

Native SSH banners use the published HTTP/HTTPS browser port. Signed-out and expired sessions preserve the full approval destination through login. The token issuer and approval authorization rules are unchanged.

## Verification

- Backend and shared API regression suites passed, including mapped HTTP/HTTPS ports and existing token issuer behavior.
- Console type checking passed; all 2,772 console tests and 247 design-system tests passed.
- Website: type checking and 117 tests passed. Documentation: 47 tests passed.
- Live browser reproduction confirmed the original link omitted port 8080 and lost its destination at sign-in.
- Live verification: an unknown temporary key produced the correct branded `http://<LAN-IP>:8080/ssh-identities/new/<code>` link; the isolated browser retained that destination through sign-in. The existing real enrolled key completed SSH with remote `true` (exit 0). The original device agent reauthenticated successfully. All deployed service health checks passed.

## Data ownership and cleanup

Automated checks use mocks and ephemeral build-container state. The live unknown-key check used one explicitly owned temporary key, registered before use in the ignored private `deployments/ssh-approval-fixtures.log` manifest with its exact fingerprint and approval code. The key was never enrolled. Its exact local files were removed; the pending approval expired and the supported scheduled cleanup removed it, confirmed by an exact-code count of zero. No demo accounts, namespaces, devices or peers were created. No production records, real browser sessions, or signing keys were deleted or reset.

Live SSH inspection uses the user's existing requested connection and client credentials. Any pending approval is part of that real login workflow, not a disposable fixture; it must be preserved and expires under the application's existing policy. No new credential will be approved on the user's behalf.

The configured Obsidian vault `../CloudMateHub/CloudMateHub` is absent on this checkout. Matching repository architecture notes (02 and 04) were updated.

The broad service integration suite requires a dedicated Docker/database test environment and was not completed inside the build container. Its missing test environment was distinct from the passing app/SSH/shared API suites and the targeted installed-agent compatibility tests.

## Terminal prompt follow-up

The remaining terminal label came from the keyboard-interactive challenge title, not an outdated Docker image. Renamed the shared server/browser bridge titles to `cloudmatehub-approval` and `cloudmatehub-approval-denied`, updated integration expectations, rebuilt and activated the server image. App, SSH, browser-terminal and installed-agent compatibility checks passed. A live SSH attempt without offering a key displayed `cloudmatehub-approval-denied` and the correct console URL. That check created no persistent fixture or enrolled key; no additional cleanup was needed.

## Saved welcome banner follow-up

Located the old welcome text in the existing namespace announcement. Backed up the exact prior value to an ignored private deployment log, updated only that field with an old-value condition, and verified a real interactive SSH shell displayed the CloudMateHub title and body with no ShellHub text. Updated the default for new namespaces and passed backend/SSH/banner regression checks. No persistent test fixtures were created; no cleanup was needed. Existing real session records were retained.
