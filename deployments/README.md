> **Current customization:** The interface has been redesigned as CloudMateHub. See [CLOUDMATEHUB.md](../CLOUDMATEHUB.md). The release-baseline notes below describe the starting point, before customization.

# Original ShellHub baseline

This checkout is ShellHub Community v0.27.0 at commit fdcc51e4fdad55440b8cfa3061b2729a50deea7b. Original server, UI, gateway, and agent source is retained without feature changes, along with Apache-2.0 licensing and copyright notices. The customization branch is `custom/cloudmatehub-baseline`.

## Local Mac

From this checkout, with Docker Desktop running:

```sh
./deployments/cloudmatehub build
./deployments/cloudmatehub start
./deployments/cloudmatehub status
./deployments/cloudmatehub stop
```

Open http://localhost:8080/setup for the original initial admin setup. The SSH gateway uses localhost:2222. No administrator password is preconfigured. Create the initial account yourself. Device pairing and SSH require an actual Linux agent/device.

The ignored `.env.override` holds a unique local database password. Never share it or the generated `api_private_key`, `api_public_key`, and `ssh_private_key`. Persistent database/gateway volumes belong to the isolated `shellhub-custom` Compose project. Stopping preserves data. Do not run `down -v` unless you intend to delete its data.

`compose.yml` builds the original production source from this checkout, without the development Docker socket mount. It adds persistent storage and binds both SSH and HTTP to the selected interface; the local default is 127.0.0.1. The `.dockerignore` addition keeps local deployment credentials/reports out of the image build context. No application features or styles have been changed. The server/gateway Dockerfile copies in this directory add build-only Git and direct-origin Go proxy fallback for dependency downloads that return HTTP 403. This Mac uses the HTTPS goproxy.io mirror, falling back to the standard proxy and source origin, with the release go.sum hashes enforced; upstream application source and dependency checksums are unchanged.

## Ubuntu 24.04 and later

See `ubuntu/README.md`. This same Linux container stack can be built on Ubuntu for the target architecture. No remote Ubuntu machine has been supplied, so remote deployment and device enrollment are not claimed.

## Scope

This baseline matches the public Community repository. Cloud/Enterprise-only services are separate upstream components and are not recreated by this checkout. Feature changes can be added after this baseline is accepted.
