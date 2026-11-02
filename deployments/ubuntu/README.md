# Ubuntu deployment

Prerequisites: Ubuntu 24.04+ with Docker Engine, Docker Compose v2.24.4+ (for `!override`), Git, Make and OpenSSL. Do not copy macOS container images to a different CPU architecture; build on the target.

Copy this source checkout to the Ubuntu Project directory, excluding `.git`, `.env.override`, private keys and deployment logs. Preserve upstream LICENSE.md. Copy `deployments/ubuntu/env.example` to `.env.override` and replace `REPLACE_WITH_RANDOM_PASSWORD` with a fresh value, for example from `openssl rand -hex 32`. Set file permissions to 600.

Run `./deployments/shellhub build` then `./deployments/shellhub start`. The initial defaults bind HTTP 8080 and SSH 2222 to localhost only. Use an SSH port forward to reach initial setup, or deliberately configure a trusted interface/domain and HTTPS following the upstream deployment documentation before granting remote access. Never expose the first-admin setup publicly.

Production DNS, HTTPS, firewall rules, backups and a reachable public/internal hostname require the actual server details. This baseline does not enroll a host agent or modify host SSH. Each future target platform/application can be given its own directory.
