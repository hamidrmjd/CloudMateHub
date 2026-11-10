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

Check that the rendered image names include hamid001/cloudmatehub-server and the other hamid001/cloudmatehub images. If the registry requires authentication, run docker login --username hamid001 with the same server account that runs Compose.

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

## Automated image updates

For `hamidrmjd/CloudMateHub`, add a Docker Hub token with write access to the
five `hamid001/cloudmatehub-*` repositories as the GitHub Actions repository
secret `CLOUDMATEHUB_DOCKERHUB_TOKEN`. The Publish CloudMateHub images workflow
builds Linux amd64 images on pushes to main and creates a Git tag only after
every image is published. It can also be run manually from GitHub Actions.

On the deployed amd64 server, use the existing Docker Hub Compose override.
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
