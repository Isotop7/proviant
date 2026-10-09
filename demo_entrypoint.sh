#!/bin/sh
set -e

if [ "$PROVIANT_DEMO_MODE" = "true" ]; then
    if [ -f "/seed/demo_seed.db" ]; then
        if [ ! -f "/app/data/proviant.db" ]; then
            echo "Demo mode: copying seed database..."
            cp /seed/demo_seed.db /app/data/proviant.db
            chmod 644 /app/data/proviant.db
        fi
    fi
fi

# The shipped config carries no tokenPassword and the server refuses to start
# without one. In demo mode, generate an ephemeral key when the deployer
# provided neither the env var nor a usable config-file value: a value baked
# into this public image would be as forgeable as the old placeholder. The
# env var outranks the config file in viper, so generating one over a
# configured password would silently replace it and invalidate sessions on
# every restart — which is why only unusable values (empty, quoted-empty or
# the retired placeholder the server now rejects at startup) count as unset.
# Restart without a configured key also invalidates sessions — acceptable for
# a demo; outside demo mode a missing password stays a startup error.
tokenpassword="$(
    grep -Ei '^[[:space:]]*tokenPassword:' /app/config.yaml 2>/dev/null |
        head -n 1 |
        sed -e 's/^[^:]*:[[:space:]]*//' -e 's/[[:space:]]*#.*$//' -e 's/^["'"'"']//' -e 's/["'"'"']$//'
)"
if [ "$PROVIANT_DEMO_MODE" = "true" ] &&
    [ -z "$PROVIANT_SERVER_AUTHENTICATION_TOKENPASSWORD" ] &&
    { [ -z "$tokenpassword" ] || [ "$tokenpassword" = "secret key" ]; }; then
    PROVIANT_SERVER_AUTHENTICATION_TOKENPASSWORD=$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')
    export PROVIANT_SERVER_AUTHENTICATION_TOKENPASSWORD
fi

exec /app/proviant
