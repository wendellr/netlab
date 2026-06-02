#!/usr/bin/env bash
set -euo pipefail

students_count="${1:-15}"
base_port="${2:-7681}"
port_step="${3:-10}"
env_dir="${4:-class-env}"

mkdir -p "$env_dir"

for i in $(seq 1 "$students_count"); do
  prefix="aluno$(printf '%02d' "$i")"
  offset=$((i - 1))
  start_port=$((base_port + offset * port_step))

  cat > "$env_dir/$prefix.env" <<EOF
LAB_PREFIX=$prefix
R1_PORT=$start_port
R2_PORT=$((start_port + 1))
R3_PORT=$((start_port + 2))
R4_PORT=$((start_port + 3))
EOF

  docker compose --env-file "$env_dir/$prefix.env" --project-name "$prefix" up -d --build
  echo "${prefix}: http://localhost:${start_port} (R1) / :$((start_port + 1)) (R2) / :$((start_port + 2)) (R3) / :$((start_port + 3)) (R4)"
done
