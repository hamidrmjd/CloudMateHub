# CloudMateHub

A self-hosted workspace for remote Linux device access, built on **ShellHub Community v0.27.0**. CloudMateHub adds its own identity, teal/slate visual system, responsive sign-in/setup layouts and console styling while retaining the upstream API and agent compatibility.

## Run locally

Requires Docker Desktop on this Mac, or Docker Engine + Compose on Ubuntu.

```sh
./deployments/shellhub build
./deployments/shellhub start
./deployments/shellhub status
./deployments/shellhub stop
```

Open **http://localhost:8080**. SSH ingress uses **localhost:2222**. Local bindings are loopback-only. `stop` preserves application data. Use the configured `localhost` hostname, not an unconfigured IP Host header.

The source directory, Compose project (`shellhub-custom`), package imports and protocol identifiers retain their existing names for compatibility. The visible product is CloudMateHub.

## Architecture picture

![CloudMateHub architecture](docs/architecture/assets/cloudmatehub-architecture.png)

[Scalable SVG](docs/architecture/assets/cloudmatehub-architecture.svg) · [Mermaid source](docs/architecture/overview.mmd)

```mermaid
flowchart LR
    Browser["Browser · CloudMateHub console"] -->|"HTTP / WebSocket · localhost:8080"| Gateway
    SSH["SSH / SCP / SFTP client"] -->|"localhost:2222 → server:2222"| Server
    subgraph Stack["Local Docker Compose · shellhub-custom"]
      Gateway["gateway · Go + Caddy"] -->|"UI and config"| UI["ui · static-web-server"]
      Gateway -->|"API + terminal + tunnel routes"| Server["server · Go / Echo
API + SSH + worker"]
      Server -->|"Durable records"| PG[("PostgreSQL 18")]
      Server -->|"Cache + jobs"| Redis[("Valkey · Redis protocol")]
    end
    Agent["Linux device agent
not deployed in this local stack"] -->|"Outbound reverse tunnel via gateway"| Gateway
    Server -.->|"Session carried over established tunnel"| Agent

```

The browser reaches the gateway, which serves the UI and forwards API/terminal/tunnel requests to the Go server. Native SSH clients connect directly to the server's published SSH port. The device agent initiates an outbound reverse connection; it is not an extra service in the default local stack.

The server owns **API + authorization + SSH/tunnel/session coordination + an in-process Asynq worker**. PostgreSQL stores durable records; Valkey supplies a Redis-compatible cache and queue. A separate worker or OpenAPI container is not part of this local production stack.

## Source layout

| Path | Responsibility |
| --- | --- |
| `ui/apps/console` | React/TypeScript console; hooks, stores, pages and browser terminal |
| `ui/packages/design-system` | Shared primitives; console overrides provide CloudMateHub styling |
| `server/api/routes`, `services`, `store/pg` | HTTP adapters → business operations → PostgreSQL |
| `server/ssh`, `server/app` | SSH/tunnels/session lifecycle and process composition |
| `gateway` | Go/Caddy HTTP routing and optional TLS |
| `agent` | Managed-host agent; device identity and outbound tunnels |
| `pkg` | Shared models, API, cache and job contracts |
| `openapi` | API specification and build-time client generation |
| `deployments` | Source-build overrides, platform examples and checks |
| `docs/architecture` | Detailed architecture notes, diagrams and cleanup policy |

## Read the architecture

- [System overview](docs/architecture/01%20System%20Overview.md)
- [Console and design](docs/architecture/02%20Console%20and%20Design.md)
- [Server and domain](docs/architecture/03%20Server%20and%20Domain.md)
- [Gateway and connections](docs/architecture/04%20Gateway%20and%20Connections.md)
- [Device agent](docs/architecture/05%20Device%20Agent.md)
- [Data and trust](docs/architecture/06%20Data%20and%20Trust.md)
- [Development and deployment](docs/architecture/07%20Development%20and%20Deployment.md)

**Obsidian:** open the `CloudMateHub` vault at `../CloudMateHub/CloudMateHub`, then open `CloudMateHub Architecture.md` or `CloudMateHub Architecture.canvas`. The vault includes linked notes for Graph view, editable canvas connections, Mermaid diagrams and PNG/SVG attachments.

## VPN / WireGuard

**VPN → WireGuard** adds personal `.conf` downloads/QR codes, instance-admin peer assignment, and the embedded **WGDashboard 4.3.4** management workspace. Ordinary users receive only their assigned configurations; management is checked on the backend. Namespace admin roles alone do not grant instance management.

[Integration, upstream feature coverage and Ubuntu requirements](integrations/wgdashboard/README.md).

This is local management preparation: a reachable Ubuntu host and VPN endpoint are still required. No public management or VPN UDP port is enabled on this Mac. Unassigning a download does not revoke an already-installed peer; restrict/delete the peer in administration to revoke its network access.

The diagram above describes the base device-access stack. The current deployment adds two internal containers: the VPN broker and WGDashboard, with separate persistent state. API/SSH/worker ownership in the existing server is unchanged.

## Data and feature changes

After each feature, remove **only the test/demo fixtures that the feature created and explicitly owns**. Preserve real accounts, devices, settings, sessions, keys, browser state and Docker volumes. Do not use a database reset or `down -v` as cleanup. Record fixture IDs and cleanup results; if no persistent fixtures were created, no cleanup is needed.

See [project instructions](AGENTS.md) and [test/demo data policy](docs/architecture/TEST-DATA-POLICY.md). No application fixtures were created or deleted for this documentation update.

## Deployment and current limits

[Deployment instructions](deployments/README.md) cover the existing source-build setup. Separate `deployments/macos` and `deployments/ubuntu` directories hold platform notes/examples. Remote Ubuntu deployment needs the actual host, DNS/TLS and firewall configuration. The local agent is not automatically installed or enrolled.

The UI build and type checking passed; **110 selected UI tests**, **19 VPN broker tests**, and **4 user-directory tests** passed. The live login screen was checked at desktop and phone widths. Real enrolled-device SSH/web-terminal/SCP/SFTP workflows remain unverified; a complete security audit and full upstream test suite have not been performed. Cloud/Enterprise-only backend extensions are not added by this customization.

## Upstream and license

Based on [ShellHub](https://github.com/shellhub-io/shellhub), release `v0.27.0`, commit `fdcc51e4fdad55440b8cfa3061b2729a50deea7b`. Original source is developed by O.S. Systems and contributors. The [Apache-2.0 license](LICENSE.md) and upstream attribution remain intact. The original README is preserved in [docs/upstream/README.shellhub.md](docs/upstream/README.shellhub.md).

[CloudMateHub customization notes](CLOUDMATEHUB.md) · [Security policy](SECURITY.md)
