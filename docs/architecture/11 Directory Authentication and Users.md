# Directory Authentication and Users

CloudMateHub supports Local, Active Directory and OpenLDAP accounts in Community. Active Directory and OpenLDAP are independent connections and may be enabled together. Existing SAML accounts are preserved.

## Administrator panels

- **Settings → Directory connections** (`/settings/directories`): separate AD/OpenLDAP tabs, redacted saved settings, service-account connection test, optional user sign-in test, CA trust, search/attribute mapping, required group and explicit account creation on first sign-in.
- **User Management** (`/users`): instance administrator account directory with source labels, search, pagination and local account creation. Directory accounts are created on successful sign-in when enabled; no bulk import or background synchronization is performed. Namespace membership remains managed through Team and invitations.

Both panels enforce live instance-administrator authorization on the server. Client navigation visibility is only presentation. New local and directory accounts have no administrator privileges, no namespace creation allowance and no device access. Existing account roles and data are unchanged.

Self-hosted Community deployments can enable `CLOUDMATEHUB_MULTI_WORKSPACE=true`. Initial setup then leaves the system unbound to a single workspace. On upgrade, startup clears an existing single-workspace binding without changing the workspace or its memberships. Instance administrators can create additional workspaces and invite directory accounts to them. Workspace tokens and assets remain scoped to Hub membership.

## Sign-in and identity

The login page offers Local plus enabled directory connections. Directory authentication is explicit; local sign-in never falls back to a directory. A service account searches for exactly one user, then the server binds as the resolved user with the submitted password. Blank passwords and ambiguous searches are refused. Passwords used to sign in are never persisted.

Directory identities use the connection's persistent ID plus the raw immutable `objectGUID` (AD) or `entryUUID` (OpenLDAP). Hub usernames have a directory prefix and stable hash suffix, avoiding collisions with local usernames. Email collisions fail rather than merging accounts. A valid email attribute is required; AD defaults to `userPrincipalName`, which administrators can map to `mail` if UPNs are not deliverable email addresses. Changing the server, base DN or permanent-ID mapping creates a new connection identity and prevents previous accounts from automatically relinking; it never deletes or reassigns those accounts. Restore the complete saved directory-state backup if recovering an earlier connection identity.

AD disabled accounts are excluded independently of the editable user filter. Optional AD group restriction supports nested membership. OpenLDAP group checks support `member`, `uniqueMember` and `memberUid`; no universal OpenLDAP disabled-account attribute is assumed. Directory password policies remain enforced by user bind.

Normal Hub tokens carry the directory origin and retain existing MFA challenge handling. Every authenticated directory token is checked against the active connection and current Hub account. Directory existence and group eligibility are checked using immutable ID with a successful-check cache of at most 60 seconds. Disabling/changing the connection invalidates checks immediately; directory outages fail closed after cached checks expire. Local administrator sign-in remains independent. Already established SSH connections are not forcibly terminated. SSH key authorization and namespace access remain Hub policy.

Sensitive SSH identity re-authentication verifies a directory password against that directory, with failure lockouts. Directory password submissions, including re-authentication, require trusted HTTPS. Sign-in failure lockouts apply by source address and by identifier. Tests use the same TLS and group verification as login; the user sign-in test never provisions an account.

## Transport and persistence

`ldaps://` uses TLS from connection start. `ldap://` requires successful StartTLS before any credentials are sent, with no plaintext fallback. TLS 1.2 or later, certificate chain trust and server hostname validation are mandatory. A private CA PEM can supplement system roots; certificate verification cannot be disabled. Network operations have bounded deadlines.

`server/directory` persists encrypted settings in `/var/lib/cloudmatehub/directory` on the new `directory-state` volume. The directory is mode 0700; `master.key` and `directories.enc` are mode 0600. Back up this volume privately along with PostgreSQL. Losing or replacing its key/settings prevents directory accounts from authenticating; preserve the whole volume. User passwords are never saved, and bind passwords are never returned by the settings API. `CLOUDMATEHUB_DIRECTORY_STATE` can override the server path.

Migration 051 adds AD/LDAP origin values and the LDAP authentication method without modifying existing users. Its rollback preserves additive enum values to avoid deleting accounts. Do not remove volumes or reset the database during deployment.

## API surface

| Endpoint | Access / purpose |
| --- | --- |
| GET `/api/auth/directories` | Public enabled connection names/types only |
| POST `/api/auth/directory` | Public HTTPS credential proof and Hub token/MFA challenge |
| GET `/api/directories` | Administrator redacted settings |
| PUT `/api/directories/:kind` | Administrator verify and enable, or disable while preserving settings |
| POST `/api/directories/:kind/test` | Administrator TLS/bind/base-DN test |
| POST `/api/directories/:kind/test-login` | Administrator user/password/group dry-run; no provisioning |
| GET `/api/user-management/users` | Administrator paginated Hub users; source labels |
| POST `/api/user-management/users` | Administrator HTTPS local account creation |

## Operational limits

A matching trusted HTTPS certificate must be activated before entering directory credentials. This feature does not obtain certificates or configure a customer's directory server. No real directory was supplied during implementation; protocol, authentication and UI tests use isolated fixtures. Configure and test each real connection before enabling it. Enable first-sign-in account creation explicitly. For access revocation, remove group eligibility or disable the directory; existing SSH streams follow their normal lifecycle.

Related: [[06 Data and Trust]], [[09 HTTPS and Recovery]], [[07 Development and Deployment]].

## Assigned websites

The authenticated website catalog stores its configuration in `websites.json` in the directory-state volume. Instance admins manage entries. The server matches assignments against the authenticated Hub account, current namespace membership, and directory membership resolved using the account's stable directory identifier. Active Directory checks nested membership. OpenLDAP checks direct membership via member, uniqueMember, or memberUid. Directory errors fail closed for the affected assignment. Direct links use the user's browser and network.

No persistent test fixtures were created, so no cleanup was needed.

Instance administrators can create local users and namespace groups from User Management over HTTP. Directory authentication and directory credential setup still require trusted HTTPS. HTTP does not encrypt local account passwords.

The Websites form stores organization-wide visibility as a namespace assignment for the selected workspace and uses User Management search to resolve Hub usernames to stable account IDs. This change required no persistent test fixtures.

A website marked `through_hub` opens an interactive remote browser on the signed-in `/websites/:id/view` route. The console posts start, input, frame, navigation and close operations to `/api/assigned-websites/:id/browser`. The server rechecks identity and current assignments for every operation, rejects API keys, and derives a renderer session key from the authenticated user, website ID and random tab ID. Client-provided ownership is never trusted. Responses are marked no-store.

The optional worker runs Chromium without a Docker network. Each session has its own browser context and temporary cookies. Playwright handles mouse, keyboard, scroll and navigation events; Chromium Fetch interception handles each HTTP redirect hop; bounded JPEG frames return to the Hub viewer. New-page links replace the displayed remote page. The website never executes inside the employee's Hub origin. Four sessions are allowed; idle sessions expire after five minutes, all sessions after one hour. Closing the viewer closes its isolated browser context, with idle expiry handling abrupt tab closures.

All public HTTP/HTTPS resource requests pass through a dedicated Unix socket to the Hub's SSRF-guarded transport. DNS destinations, including redirects and resources, cannot resolve to private addresses. The broker allows bounded GET/HEAD/POST traffic and selected website headers/cookies, but never Hub authorization or cookies. Downloads are rejected by content disposition and Chromium download cancellation, file choosers receive no files, and WebSockets/service workers are blocked. Request and byte budgets limit each session. Assignments govern entry points; links may navigate to other public domains. The older sanitized preview endpoint remains available but is no longer the interactive viewer. Local origin-specific TLS relays are deployment configuration and retain original-host certificate verification.

No persistent verification fixtures are required for interactive browsing. In-memory catalogs and isolated temporary browser contexts are used for tests; existing users, websites, namespaces and browser storage are preserved.

The localhost verification environment uses exact-host TLS relays for Kladex, NVIDIA and MDN where direct workstation egress fails. These private `.env.override` settings are not published deployment defaults, and retain destination certificate verification.
