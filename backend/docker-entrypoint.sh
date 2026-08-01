#!/bin/sh
set -eu

mkdir -p /app/storage/model-assets
chown -R appuser:appuser /app/storage

exec su-exec appuser /app/taverna
