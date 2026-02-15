#!/bin/sh
# Entrypoint script: adjusts UID/GID of the mush user to match PUID/PGID,
# fixes ownership on data directories, then drops privileges to run the server.

PUID=${PUID:-1000}
PGID=${PGID:-1000}

# Adjust group
if [ "$(id -g mush)" != "$PGID" ]; then
    delgroup mush 2>/dev/null
    addgroup -g "$PGID" mush
fi

# Adjust user
if [ "$(id -u mush)" != "$PUID" ]; then
    deluser mush 2>/dev/null
    adduser -D -h /game -u "$PUID" -G mush mush
fi

# Ensure the mush user is in the right group
adduser mush mush 2>/dev/null

# Fix ownership on writable directories
chown -R mush:mush /game/data /game/certs /game/seed 2>/dev/null

echo "Starting as UID=$(id -u mush) GID=$(id -g mush) TZ=${TZ:-UTC}"

# Drop privileges and exec the server
exec su-exec mush gotinymush "$@"
