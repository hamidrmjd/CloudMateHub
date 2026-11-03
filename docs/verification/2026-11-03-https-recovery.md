# HTTPS controller deployment and recovery verification

Date: 2026-11-03

## Deployed behavior

The HTTPS controller, internal gateway, console and WGDashboard integration images were built and deployed. All eight deployment services started successfully. The current host IP replaced the obsolete LAN address in the ignored local deployment configuration; public dashboard and SSH ports are published on that address. Recovery is published only on host loopback port 9080.

No production certificate has been supplied or activated. The existing HTTP dashboard remains available until a matching certificate is uploaded and explicitly confirmed. Automatic certificate issuance and renewal remain a later phase.

## Checks

- Gateway Go tests passed, including nine HTTPS-manager tests covering certificate validation, staging, timeout/restart rollback, production certificate isolation, candidate-only confirmation, restore, WebSocket forwarding, corrupt-state recovery, and nonblocking health checks.
- Console TypeScript checking and the targeted HTTPS/settings tests passed. Targeted WireGuard UI tests passed during the preceding integration work.
- VPN broker: 22 tests passed, including Secure cookies behind HTTPS.
- Live dashboard health and the HTTPS settings route returned HTTP 200 after deployment.
- The independent recovery page loaded in the browser. Authorized recovery status reported the current host with no active or pending certificate.
- Anonymous management and recovery requests were refused. Cross-origin recovery changes were refused.
- An invalid certificate/key upload returned HTTP 400. Recovery state remained identical, and dashboard health still returned HTTP 200 afterward.
- Recovery credentials were captured privately for the smoke check and were not printed or embedded in the page. The state directory is mode 0700; the master and recovery key files are mode 0600.
- Host loopback recovery responded; connection to the LAN IP on port 9080 was refused.
- All deployment services were running after verification. No authenticated browser walkthrough of the console or live production-certificate activation is claimed; valid-certificate activation and rollback were exercised in isolated Go tests.

## Test data and cleanup

No persistent test fixtures were created by HTTPS verification. Certificate fixtures use temporary test directories and are torn down by the test framework. The live invalid upload was rejected before any revision was stored. No cleanup was needed. The controller's real service encryption and recovery keys are persistent deployment state and were preserved, as were all real users, devices, namespaces, sessions, settings, signing keys and files. Browser inspection used a separate agent-created tab without modifying real user sessions.

README and repository architecture notes were updated. The configured Obsidian vault `../CloudMateHub/CloudMateHub` is absent in this checkout, so a matching vault update could not be made.
