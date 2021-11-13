#!/bin/sh

[ -n "${PUID}" ] && usermod -u "${PUID}" yarnd
[ -n "${PGID}" ] && groupmod -g "${PGID}" yarnd

printf "Fixing ownership of default /data volume...\n"
chown -R yarnd:yarnd /data

printf "Switching UID=%s and GID=%s\n" "${PUID}" "${PGID}"
exec su-exec yarnd:yarnd "$@"
