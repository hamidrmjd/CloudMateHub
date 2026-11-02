#!/bin/sh
set -eu
umask 077
/opt/wgdashboard/src/venv/bin/python /opt/cloudmatehub/initialize.py
exec /bin/bash /entrypoint.sh
