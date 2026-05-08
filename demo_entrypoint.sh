#!/bin/sh
set -e

if [ "$PROVIANT_DEMO_MODE" = "true" ]; then
    if [ -f "/seed/demo_seed.db" ]; then
        if [ ! -f "/app/data/proviant.db" ] || [ "$NIGHTLY_RESET" = "true" ]; then
            echo "Demo mode: copying seed database..."
            cp /seed/demo_seed.db /app/data/proviant.db
            chmod 644 /app/data/proviant.db
        fi
    fi
fi

exec /app/proviant