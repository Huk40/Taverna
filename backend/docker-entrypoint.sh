#!/bin/sh
set -eu

mkdir -p /app/storage/model-assets /app/storage/export-artifacts
chown -R appuser:appuser /app/storage

exec su-exec appuser /app/taverna
