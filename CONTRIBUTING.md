# Contributing to CloudMateHub

Use focused branches and pull requests. Explain the problem, resulting behavior, validation and known limits. Preserve Apache-2.0 licensing and upstream attribution.

## Local setup

Follow README.md and `deployments/README.md`. Never commit private environment files, runtime data, credentials or generated client/build output. Use the existing Compose wrapper and pinned integration image.

## Before submitting

1. Read `AGENTS.md` and the relevant architecture notes.
2. Keep business authorization on the backend, including ownership checks for private VPN material.
3. Run the meaningful checks for the affected layers. VPN broker tests are in `integrations/wgdashboard/test_broker.py`, directory tests in `server/vpn`, and UI tests in `ui/apps/console/src/pages/vpn/__tests__`. The container check recipes are in `deployments/ui-check.Dockerfile` and `integrations/wgdashboard/Tests.Dockerfile`.
4. Remove only test/demo fixtures explicitly owned by your change; preserve real records and signing keys.
5. Update README.md and architecture documentation when flows or deployment change.
6. Review the diff for secrets and unrelated changes.

The archived upstream contributing guide is `docs/upstream/CONTRIBUTING.md`; upstream release automation and private enterprise infrastructure are not required for CloudMateHub contributions.
