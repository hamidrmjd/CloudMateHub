# Development and deployment

## Repository map

| Directory | Purpose |
| --- | --- |
| `ui/apps/console` | CloudMateHub React console |
| `ui/packages/design-system` | Shared UI primitives and theme foundation |
| `ui/apps/docs`, `ui/apps/website` | Upstream documentation/website source, not separate services in this local stack |
| `server` | API, services, PostgreSQL adapters, SSH and worker composition |
| `gateway` | HTTP gateway / Caddy configuration |
| `agent` | Managed-host agent |
| `pkg` | Shared Go models and infrastructure/contracts |
| `openapi` | API specification, generation tooling and related Go module |
| `tests` | Integration/conformance helpers and tests |
| `deployments` | Our source-build overrides, platform examples and checks |
| `docs/architecture` | Repository architecture source, images and cleanup policy |

## Build and runtime are different

Node/npm build the React application; the runtime UI container serves static files. Go compiles the server and gateway. OpenAPI is bundled and generates the console client during the UI build; no standalone OpenAPI container is active in the local production stack.

`bin/docker-compose` combines `.env`, `versions.env`, private `.env.override`, PostgreSQL configuration and `deployments/compose.yml`. The project remains named `shellhub-custom` and the source folder `cloudmatehub` for continuity; the product interface is CloudMateHub.

```sh
./deployments/cloudmatehub build
./deployments/cloudmatehub start
./deployments/cloudmatehub status
./deployments/cloudmatehub stop
```

`start` generates missing keys and waits for health checks. `stop` preserves data. Do not use `down -v` for ordinary updates or test cleanup.

Our Go Dockerfile copies add a build-time dependency-download fallback for the network's proxy restrictions. This does not change API contracts or turn off Go dependency checksums. UI compatibility imports and protocol environment names intentionally retain `cloudmatehub` identifiers.

## Feature workflow

1. State the behavior and affected ownership layer.
2. Register fixture IDs before creating persistent test/demo records.
3. Implement the backend/spec/client/UI pieces that actually change.
4. Run meaningful relevant checks and verify the local UI when it changes.
5. Remove only explicitly owned fixtures and record the result.
6. Update the README and these linked notes if architecture or flows changed.

## Ubuntu status

`deployments/ubuntu` contains an environment example and handoff instructions for Ubuntu 24.04+. Build containers for the target CPU. A remote deployment, public TLS/DNS, and real device enrollment require the actual host details and have not been verified here.

Related: [01 System Overview](01%20System%20Overview.md), [02 Console and Design](02%20Console%20and%20Design.md), [06 Data and Trust](06%20Data%20and%20Trust.md).

Directory support adds the persistent `directory-state` server volume and additive migration 051. Include its complete contents in private backups, preserve existing volumes, and activate trusted HTTPS before entering directory credentials. See [[11 Directory Authentication and Users]].

Requested temporary demo devices may run as separately labelled agent containers outside Compose. The current requested sample set is documented in `docs/verification/2026-11-04-requested-demo-data.md`, with exact ownership in its JSON manifest. Its online device is a real isolated agent tunnel; offline and pending examples are intentionally stopped. These sample containers are not production Compose services. Cleanup must inspect dependencies and match exact IDs/labels; the existing namespace and real data must be preserved.

CloudMateHub deployment uses the `cloudmatehub` Compose project and locally built `cloudmatehub/server:local`, `cloudmatehub/ui:local`, and `cloudmatehub/gateway:local` images. Core application builds require no published ShellHub application images. Standard infrastructure and WGDashboard dependencies retain their upstream attribution. Existing installations may set `CLOUDMATEHUB_VOLUME_PREFIX` and `CLOUDMATEHUB_EXISTING_VOLUMES=true` to reuse their original data volumes; preserve signing keys and never reset volumes during migration.
