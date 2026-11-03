# Requested temporary dashboard demo data

Date: 2026-11-04 (Asia/Tehran). Run ownership ID retains the date the request began: `codex-requested-demo-20261103`.

The user explicitly requested temporary dashboard entries. These requested sample devices are intentionally retained for testing, separate from verification fixtures that are removed after feature checks. No automatic expiration is promised.

## Available examples

| Device | State | Purpose |
| --- | --- | --- |
| demo-web-01 | Accepted, online | Connected-device card and Browser Console entry |
| demo-db-01 | Accepted, offline | Offline connection prevention |
| demo-pending-01 | Pending, offline | Administrator device approval flow |

These devices are isolated Docker containers built from the current project agent source, without privileged mode, host-network access, host files, Docker socket mounts or published ports. The online state is maintained by an actual agent tunnel rather than a forged database heartbeat. The online container is labelled `cloudmatehub.fixture=codex-requested-demo-20261103`; two other owned containers are stopped to supply offline examples. Their operating system is Alpine Linux. The root password is locked; no shared password has been created or distributed. Existing SSH identities, policies and HTTPS requirements continue to apply. No real SSH command was executed for this data setup.

The Community installation permits one namespace. The attempted additional namespace was rejected without creating a record. No sample accounts or namespace memberships were created. The three devices were enrolled into the existing namespace through normal signed device authentication, labelled using the supported custom-field API, and only two were accepted through the supported device-status API. The existing namespace and its owner are never cleanup targets.

Exact planned and actual device IDs, synthetic MAC identities and container IDs were recorded in `2026-11-03-requested-demo-data.json` before approval or status changes. Each device carries `custom_fields.demo_owner=codex-requested-demo-20261103`. Name matching alone is insufficient evidence for removal.

## Verification and cleanup boundaries

The API verified `demo-web-01` accepted/online, `demo-db-01` accepted/offline, and `demo-pending-01` pending/offline. No existing device status, membership, namespace settings, user credentials or server signing keys were changed. A short-lived in-memory administrative API credential was used for the exact owned fixture operations and was never stored or shared. Planned sample-login files were removed when account creation was cancelled.

Before later cleanup, inspect device dependencies for any user-created sessions, tags, assignments or files. Stop only the exact manifest-labelled/ID-matched demo containers. Remove only manifest-owned device records through the supported device API, inspecting dependent recordings/sessions first. Retain all real users, devices, namespaces, keys, sessions and files; preserve ambiguous dependencies and report them. Do not delete the existing namespace or reset volumes/database state.

Live signed-in browser verification displayed the online demo-web-01 and offline demo-db-01 cards in Browser Console. The pending example is available on Devices → Pending. The browser correctly retains its HTTPS requirement for browser-key SSH.
