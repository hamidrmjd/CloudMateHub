# Deploy CloudMateHub on Ubuntu

Use Ubuntu 24.04 or newer on amd64 or arm64 with Docker Engine, Docker Compose v2.24.4 or newer, Git, Make, and OpenSSL. Run as a user with Docker access.

## Fresh installation

```bash
git clone git@github.com:hamidrmjd/CloudMateHub.git
cd CloudMateHub
(
  umask 077
  set -C
  db_password=$(openssl rand -hex 32) || exit 1
  sed "s/REPLACE_WITH_RANDOM_PASSWORD/$db_password/" deployments/ubuntu/env.example > .env.override
)
```

In `.env.override`, replace `REPLACE_WITH_PUBLISHED_TAG` with a tag published under `hamidrazavi000`.

Check that the rendered image names include hamidrazavi000/cloudmatehub-server and the other hamidrazavi000/cloudmatehub images. If the registry requires authentication, run docker login --username hamidrazavi000 with the same server account that runs Compose.

```bash
./bin/docker-compose config --images
./bin/docker-compose pull &&
  ./deployments/cloudmatehub start &&
  ./deployments/cloudmatehub status
```

From your computer, forward the dashboard port through your server's SSH login.

```bash
ssh -N -L 8080:127.0.0.1:8080 ubuntu@SERVER_IP
```

Open [initial setup](http://localhost:8080/setup) and create the first administrator. For LAN access after setup, set SHELLHUB_BIND_ADDRESS and SHELLHUB_DOMAIN to the server's LAN IP in .env.override and rerun the start command.

## Existing installation

Run git pull --ff-only and add the three Docker Hub settings from [env.example](env.example) to your existing .env.override. Preserve the other settings, keys and volumes. Run the image check, pull and start commands above.

## Release process

Set `CLOUDMATEHUB_VERSION` in the tracked `.env` to the next `MAJOR.MINOR.PATCH` version. Bump major for incompatible changes, minor for compatible features, and patch for compatible fixes. CloudMateHub uses the same `vMAJOR.MINOR.PATCH` tag for all five application images. Leave the upstream compatibility version in `SHELLHUB_VERSION`.

1. Test the source-built images on the workstation and review API, Compose, and database migration changes. For a release with a data migration, verify that the existing backup can be restored before rollout.
2. Commit the version change with the approved release. Merging the version bump to `main` triggers publication. The workflow also supports manual dispatch on `main`.
3. Wait for all five registry manifests and the requested platforms to be verified. The workflow then creates the annotated Git tag `cloudmatehub-vMAJOR.MINOR.PATCH`, which makes the release eligible for automatic deployment.
4. Use a new version for any change to a published image. If publication stops after uploading only part of a release, leave that version unmarked and bump the version before retrying.

Use `python3 deployments/publish-dockerhub.py hamidrazavi000 --tag v0.28.0 --dry-run` to check the image names for version `0.28.0`. The command defaults to Linux amd64 and arm64. The automated workflow currently publishes amd64. A manual Docker Hub upload alone does not create the Git release marker.

## Automated image updates

For `hamidrmjd/CloudMateHub`, add a Docker Hub token with write access to the
five `hamidrazavi000/cloudmatehub-*` repositories as the GitHub Actions repository
secret `CLOUDMATEHUB_DOCKERHUB_TOKEN`. The Publish CloudMateHub images workflow
builds Linux amd64 images after a version bump reaches main and verifies every
image before marking the release ready.

On the deployed amd64 server, update the checkout to get the SemVer-aware updater and use the existing Docker Hub Compose override. The updater installs newer releases and supports the transition from the old CI-run tags.
From `/opt/CloudMateHub/CloudMateHub`, install and enable the timer:

```sh
sudo install -m 0644 deployments/ubuntu/cloudmatehub-update.service /etc/systemd/system/
sudo install -m 0644 deployments/ubuntu/cloudmatehub-update.timer /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now cloudmatehub-update.timer
```

The service account must be able to read the Git remote and pull the images
without prompts. Check updates with `journalctl -u cloudmatehub-update.service`.
The updater pulls the complete image set before starting containers and saves
the new tag only after Compose reports readiness. A startup failure attempts to
restart the prior images. This does not reverse database migrations. Pause the
timer before a manual rollback or a release that needs a coordinated data or
Compose configuration change. The timer deploys images from the existing
checkout; update the checkout separately when the release requires new Compose
configuration.
