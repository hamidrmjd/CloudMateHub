# Baseline verification

Original ShellHub Community **v0.27.0**, commit `fdcc51e4fdad55440b8cfa3061b2729a50deea7b`. Application source verified unchanged against the release. The only modified tracked upstream file is `.dockerignore`; deployment additions are in this directory.

- Server, UI and gateway built successfully from the source checkout.
- All five services running. Server, UI, gateway and PostgreSQL pass their health checks; Valkey has no upstream health check.
- Setup, health, system information and UI configuration routes return HTTP 200 at the configured `localhost` hostname. The gateway rejects the unconfigured `127.0.0.1` hostname with 404; use localhost.
- Backend reports v0.27.0 and incomplete initial setup; UI configuration reports Community edition.
- Original setup wizard visually verified in the browser. Initial administrator account remains for you to create.
- Compose configuration verified to bind only localhost HTTP 8080 and SSH 2222, with persistent PostgreSQL storage. Generated private keys and local environment file have private permissions.

No real device has been enrolled. Authenticated web terminal, SSH, SCP, SFTP, permission and fleet-management workflows have not been exercised. No remote Ubuntu machine was supplied. The full upstream test suite and a comprehensive security audit were not run. This is a source-preserving baseline, not a claim of exhaustive behavioral equivalence.

Build-only network fallback is documented in README.md. Release dependency versions/checksums were not edited. Build output contains upstream npm deprecation/audit notices and bundle-size warnings; no dependency upgrades were applied because this request is to retain the baseline.

Detailed results: verification.json, services.json and build.log.
