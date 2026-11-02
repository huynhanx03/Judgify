#!/bin/sh
set -eu

root="$(cd "$(dirname "$0")/.." && pwd)"
api_env="$root/api/.env"
certs="$root/api/certs"
private_key="$certs/private_key.pem"
public_key="$certs/public_key.pem"

command -v openssl >/dev/null 2>&1 || {
	echo "openssl is required for local setup" >&2
	exit 1
}

copy_env() {
	[ -f "$2" ] || {
		cp "$1" "$2"
		chmod 600 "$2"
	}
}

copy_env "$root/.env.example" "$root/.env"
copy_env "$root/api/.env.example" "$api_env"

if ! grep -q '^JWT_SECRET=.' "$api_env" ||
	grep -q '^JWT_SECRET=replace-with-' "$api_env"; then
	secret="$(openssl rand -hex 32)"
	temporary="$(mktemp "$api_env.XXXXXX")"
	awk -v secret="$secret" '
		/^JWT_SECRET=/ { print "JWT_SECRET=" secret; found = 1; next }
		{ print }
		END { if (!found) print "JWT_SECRET=" secret }
	' "$api_env" >"$temporary"
	chmod 600 "$temporary"
	mv "$temporary" "$api_env"
fi

mkdir -p "$certs"
umask 077

if [ ! -e "$private_key" ] && [ ! -e "$public_key" ]; then
	openssl genpkey -algorithm RSA \
		-pkeyopt rsa_keygen_bits:3072 \
		-out "$private_key"
	openssl pkey -in "$private_key" -pubout -out "$public_key"
elif [ -s "$private_key" ] && [ ! -e "$public_key" ]; then
	openssl pkey -in "$private_key" -pubout -out "$public_key"
elif [ ! -s "$private_key" ] || [ ! -s "$public_key" ]; then
	echo "incomplete local JWT key pair in $certs" >&2
	exit 1
fi

chmod 600 "$private_key"
chmod 644 "$public_key"
echo "Local env and JWT keys are ready."
