# Data and trust

## State ownership

| State | Location | Persistence / handling |
| --- | --- | --- |
| Application records | PostgreSQL | Named `shellhub-custom_postgres-data` volume in the default project |
| Cache and queued jobs | Valkey (`redis` service) | Current Compose disables snapshot/AOF persistence; not the durable source of truth |
| Active connection/session registry | Server memory | Runtime state; recreated after process restart |
| Gateway certificates / state | Gateway `/data` | Named `shellhub-custom_gateway-data` volume; TLS currently disabled locally |
| API signing key pair | Host files mounted as Compose secrets | Retain across feature/UI updates; never publish contents |
| SSH host key | Host file mounted as Compose secret | Retain; changing it affects host identity |
| Runtime configuration / DB password | Ignored `.env.override` | Private local file; examples must contain placeholders only |
| Browser sessions / preferences / vault metadata | Browser storage | Preserve real user state; do not clear globally after tests |

Namespaces scope devices and membership. Application services and store queries, rather than the picture of the network, must enforce that scope. API credentials and device keys are different identities.

## Boundaries

1. Browser input is untrusted; backend authorization is required even if the UI hides an action.
2. The gateway forwards transport context; it is not the domain permission engine.
3. Database and cache have no host-published ports in the local deployment.
4. A remote agent handles operations on its host; enrollment and access must remain explicit.
5. Documentation must contain no passwords, tokens, private keys or real session recordings.

## Test/demo cleanup after features

The user's instruction is to remove **only test/demo data**, not all application data. Identify fixtures by their recorded IDs and ownership. Remove them with supported fixture/API teardown, checking dependencies to avoid deleting real records. Names containing `test` or `demo` alone do not prove ownership.

Never use volume deletion, database reset, key rotation or blanket browser-storage clearing as routine feature cleanup. If no persistent fixtures were created, record that no cleanup was needed. This documentation change creates no application fixtures and deletes no accounts/devices.

The durable project instruction is in repository `AGENTS.md`; the detailed rule is `docs/architecture/TEST-DATA-POLICY.md`.

Related: [03 Server and Domain](03%20Server%20and%20Domain.md), [07 Development and Deployment](07%20Development%20and%20Deployment.md).
