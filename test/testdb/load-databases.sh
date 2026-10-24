#!/bin/sh
set -eu

for file in /db/*.sql; do
	name=$(basename "$file" .sql)
	exists=$(psql -U "$POSTGRES_USER" -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname = '$name'")
	if [ -z "$exists" ]; then
		psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d postgres -c "CREATE DATABASE \"$name\""
	fi
	psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$name" -f "$file"
done
