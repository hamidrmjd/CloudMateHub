# Client browser SSH console verification

Date: 2026-11-03

## Delivered behavior

SSH → Browser Console (`/console`) lists accepted devices within the active namespace, supports device search and pagination, refreshes online status, opens the existing SSH connection dialog and resumes current-namespace terminals. Offline targets are disabled. Namespace observers get a permission explanation; Operator and higher roles use existing DeviceConnect authorization and SSH policies. Pending selection resets when the user or namespace changes.

No new backend authentication or shell path is added. The existing single-use browser SSH bridge, credentials/identity handling and server-side recording remain in use. New-identity namespaces require HTTPS or localhost for browser WebCrypto keys; the page explains this and blocks unsupported plaintext browser-key attempts.

## Verification

Console TypeScript checking and 36 tests in five focused console/connection dialog/navigation/terminal store/terminal component files passed. Coverage includes non-admin Operator access, offline prevention, namespace-scoped terminal resumption, observer denial, clearing selections on namespace change, HTTPS requirements, accepted-device API parameters, searching, pagination, and empty states.

Deployment and live browser results are appended after activation. No real SSH commands are run on a user's device for this UI feature.

## Test-data ownership and cleanup

No persistent production fixtures are created. UI tests use in-memory mock devices `fixture-online` / `fixture-offline`, session `fixture-session`, and an explicitly foreign `foreign-session` under `other-tenant`; all exist only in isolated test processes. No test SSH identities, sessions, accounts, device enrollments, namespace changes or VPN peers are created in production. No production cleanup is required. Real browser storage, accounts, devices, recordings, namespace membership, signing keys and files are preserved.

README and repository Console/Connections architecture notes are updated. The requested separate Obsidian vault path `../CloudMateHub/CloudMateHub` is absent; the maintained repository notes are ready to import.

## Deployment and live verification

The production console image built and only the UI service was recreated; its health check passed. Live `/console`, the entry script and the new BrowserConsole page asset returned HTTP 200. The signed-in isolated in-app browser displayed the new SSH sidebar link, Browser Console heading, current-namespace device card and offline prevention without console errors.

The actual current device was offline and the current namespace uses browser-key identities over a plain HTTP address. The UI correctly explains that trusted HTTPS (or localhost) is required and disables an unsupported connection. No real SSH session was opened, no command was executed on a real device, and no new persistent SSH identity was created during verification. Real end-to-end SSH requires the device to reconnect and the administrator to activate trusted HTTPS. All existing records and signing keys are preserved.
