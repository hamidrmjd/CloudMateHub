# WireGuard administrator creation repair

Date: 2026-11-03

## Report and cause

The signed-in Hub administrator saw an unavailable management session. Reproduction in the isolated in-app browser showed native authentication fetches failing with HTTP Forbidden. The VPN broker set Referrer-Policy to no-referrer while requiring Origin, Referer or same-origin Fetch Metadata for cookie requests. Browsers that omit Origin and Fetch Metadata on these GET fetches could not supply the required evidence.

## Repair

The embedded responses now use Referrer-Policy: same-origin. Same-origin fetches preserve their referrer; cross-origin navigation still sends none. Hub identity revalidation, instance-admin checks, HttpOnly/SameSite cookies, session expiry and outside-origin rejection remain enforced. Only the VPN broker was recreated for activation.

## Test-data ownership manifest (before fixture creation)

Production fixtures: none. Preserve all real configurations, peers, assignments, users, settings and keys.

Isolated runtime fixture ownership:

- Container name: `cmh-wg-create-check-20261103`, labelled `cloudmatehub.test=wg-create-20261103`.
- Container network: none, no published ports, no production volumes or secrets.
- Temporary container-only filesystems: /data, /etc/wireguard and /etc/amnezia/amneziawg.
- Temporary test key file: `/tmp/cloudmatehub-wg-debug/fixture-service-key`, containing a test-owned random key, mounted into this isolated container only.
- Interface name: `cmhfix1103`; test subnet `198.18.43.1/24`, listen port 51993, protocol wg.
- Peer label: `cmh-owned-creation-fixture`; address `198.18.43.2/32`. Its exact public identifier must be registered inside the test teardown state before attempting peer creation; private keys must never be printed or committed.
- Ownership ends with supported fixture API teardown and removal of this explicitly owned container; test filesystems are ephemeral and cannot contain real records.

No browser test data is created in the user's real Chrome session. The user signed in to the separate in-app browser for read-only verification. Browser storage is not globally cleared.

## Verification

Twenty-three broker tests passed, including same-origin requests without Fetch Metadata, connection creation proxying, external/lookalike/wrong-scheme referrer rejection, member denial, ownership isolation and expiry/revocation.

Live browser verification: Interfaces and Add interface now render for the Hub administrator without the native login prompt or the Forbidden error. Runtime creation and final cleanup results follow below.

## Final result and cleanup

The real native API in the isolated container successfully created `cmhfix1103`, created the owned peer, returned a valid private client configuration, deleted the peer through its supported API, and deleted the interface through its supported API. The script checked the created peer's exact recorded public identifier and checked that the interface was absent after teardown. No private keys were logged.

The network-disabled test container was stopped and auto-removed, discarding its test-owned temporary filesystems and runtime ownership record. Its disposable host key was unlinked. No production fixtures were created and no real VPN records were removed. Only `vpn-broker` was recreated; all deployment services remain running.

README and the repository WireGuard architecture/integration notes were updated. The requested separate Obsidian vault path is absent; repository architecture notes remain the maintained copy.
