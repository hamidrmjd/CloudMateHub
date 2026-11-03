# Active Directory, OpenLDAP and User Management verification

Date: 2026-11-03

## Delivered behavior

Independent administrator AD/OpenLDAP connection panels support encrypted transport, private CA trust, service-account/base-DN checks and a user sign-in dry-run. A separate User Management tab lists account sources and creates local members. Login explicitly selects Local, Active Directory or OpenLDAP when directory connections are enabled. Both directory types can operate together. Local sign-in is independent.

Directory users have immutable external identity, no automatic email merge, no initial administrator privileges and no automatic namespace/device access. First-sign-in provisioning requires an explicit option. Session validation checks the enabled connection, directory existence and group eligibility with at most a 60-second successful-check cache. Directory password step-up supports the SSH identity approval flow and requires trusted HTTPS.

Migration 051 is additive. Bind credentials are encrypted in directory-state; submitted user passwords are never saved. Disabling or changing connections preserves all existing account and namespace records.

## Checks

- Go directory/application/authentication middleware suites passed, plus focused existing local authentication, user-token authentication and web re-authentication tests.
- Eleven directory tests (with scenario subtests) cover encrypted settings/restart/redaction/permissions; least-privilege provisioning; stable identity after login-name change; local email collision; wrong password, ambiguous results, disabled provisioning, unavailable database and missing group; disabling/replacing a connection; filter escaping; mandatory AD disabled-account filtering and nested group OIDs; attribute/URL/CA validation; credential preservation boundaries; account-name limits; anonymous/forged-admin rejection; trusted TLS and mandatory StartTLS with no credential leakage.
- A separate service test verifies directory-token denial when its verifier is absent/offline, acceptance with valid directory proof and independence of local accounts.
- Console TypeScript checks and 92 tests in six files passed, covering local and directory sign-in, directory MFA challenges, namespace guards, SSH approvals, separate connection panels, source labels, admin visibility and HTTP secret blocking.
- No real Active Directory/OpenLDAP endpoint or credentials were supplied. Real provider connection tests remain for the administrator after HTTPS activation.

## Deployment

The server and console production images built and were activated. All eight services are healthy. The host address changed since the previous release; ignored local deployment settings were updated to `192.168.1.105`, with dashboard port 8080 and SSH port 2222.

Live smoke checks confirmed HTTP 200 for User Management, Directory connections, the console entry script and both new page assets. The public connection list is empty while profiles remain disabled. Anonymous/forged-admin requests to directory settings and user management receive 401. A directory sign-in request over HTTP with a forged HTTPS forwarding header still receives 400 through the real gateway. Directory-state is mode 0700 and its key/encrypted settings are mode 0600. PostgreSQL contains the new origin/authentication enum values.

No authenticated live browser or real directory integration test is claimed. Real connection testing remains available in the administrator panel after trusted HTTPS is activated.

## Test ownership and cleanup

No persistent production fixtures were created. Exact fake database identifiers are `fixture-user` and `local-admin`; their ownership is confined to directory test in-memory stores. Frontend identities use `directory-fixture-admin`, `fixture-admin`, `fixture.local`, `fixture.ad`, `fixture.ldap`, and mock tokens confined to isolated test processes. TLS listener ports are dynamically allocated by test-owned listeners and closed by teardown. Files use Go testing temporary directories registered through `t.TempDir()` and removed automatically. No real accounts, namespaces, sessions, files, keys or browser storage were cleared. No production cleanup was needed.

## Documentation

README and repository architecture notes were updated, including 11 Directory Authentication and Users. The requested Obsidian vault path `../CloudMateHub/CloudMateHub` is absent; these repository notes are ready to import into it. Secrets and recovery links are excluded.
