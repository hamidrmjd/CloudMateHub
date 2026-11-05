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
