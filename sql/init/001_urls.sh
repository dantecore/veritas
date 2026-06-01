#!/bin/sh
set -eu

sed '/-- +goose Down/,$d' /migrations/001_urls.sql |
  psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB"
