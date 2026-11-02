# CloudMateHub security policy

CloudMateHub is a development-stage customization of ShellHub Community with a WGDashboard integration. It has not received a comprehensive independent security audit. Do not expose the local preparation deployment directly to the Internet.

## Report privately

Use this repository's **Security → Report a vulnerability** when private reporting is enabled. Do not put credentials, private configurations, user data, or exploit details in a public issue. If private reporting is unavailable, request a private contact channel without disclosing the vulnerability.

Upstream-only issues should also be reported through the relevant project's security policy: ShellHub or WGDashboard. The upstream security policy is archived under `docs/upstream/SECURITY.md`; its response commitments and contacts belong to upstream, not CloudMateHub.

## Deployment practices

- Keep management behind HTTPS and explicit authentication; retain server-side role/ownership checks.
- Keep `.env.override`, signing keys, WireGuard private configurations, API keys, database files and runtime logs out of Git.
- Generate new deployment secrets locally; preserve real data and keys during feature updates.
- Review dependency advisories and pinned container updates before deploying.
- Validate Ubuntu routing, firewall, DNS and WireGuard peer revocation before using the VPN.

See `integrations/wgdashboard/README.md` for the current trust boundaries and limitations.
