# Data and trust

## State ownership

| State | Location | Persistence / handling |
| --- | --- | --- |
| Application records | PostgreSQL | Named `shellhub-custom_postgres-data` volume in the default project |
| Recording archive settings and keys | Separate `recording-state` volume | Encrypted destinations plus master key and per-seat manifests; protected backups required |
| Recording archives | `recording-files` local volume, mounted NFS, or S3 | Admin-selected destination; PostgreSQL retains captured output as a recovery copy |
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

HTTPS certificate revisions, encrypted TLS keys, the encryption master key, the private recovery key, and last-good configuration live in a separate `https-state` volume. Back up the whole volume privately; never put its keys or recovery links in Git or this vault. Certificate restore changes only HTTPS state, not the application database. See [09 HTTPS and Recovery](09%20HTTPS%20and%20Recovery.md).

Server-side SSH recording and admin-selected local/NFS/S3 archive storage are described in [[10 Session Recording and Storage]]. PostgreSQL retains a recovery copy; destination changes preserve existing recordings.

Local, Active Directory and OpenLDAP account trust and administrator user management are described in [[11 Directory Authentication and Users]]. Bind credentials are encrypted in the separate `directory-state` volume. User passwords are not persisted. Directory accounts use immutable external identifiers and are never merged into local accounts by email. Directory disabling invalidates token checks without deleting accounts or namespaces.

## Temporary SSH grants

Allow policies can store an optional expiration timestamp in PostgreSQL. The SSH policy evaluator skips expired grants for new sessions. Existing policies have no expiration, and deny rules continue to take precedence. Expiration does not terminate an open session. The policy form accepts the browser's local time and sends an absolute timestamp to the server. No persistent test fixtures were created.
