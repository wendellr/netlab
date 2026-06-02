#!/usr/bin/env bash
set -euo pipefail

students_count="${1:-15}"
env_dir="${2:-class-env}"

for i in $(seq 1 "$students_count"); do
  prefix="aluno$(printf '%02d' "$i")"
  env_file="$env_dir/$prefix.env"

  if [[ -f "$env_file" ]]; then
    docker compose --env-file "$env_file" --project-name "$prefix" down
  else
    echo "Skipping $prefix: $env_file not found"
  fi
done
