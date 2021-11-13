#!/bin/sh

[ -z "${PUID}" ] && usermod -u "${PUID}" yarnd
[ -z "${PGID}" ] && groupmod -g "${PGID}" yarnd

printf "Switching UID=%s and GID=%s\n" "${PUID}" "${PGID}"

su -s /bin/sh -c "exec $*"
