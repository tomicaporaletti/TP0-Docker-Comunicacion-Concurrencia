#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 2 ]]; then
  echo "Uso: $0 <archivo_salida> <cantidad_clientes>"
  echo "Ej : $0 docker-compose-dev.yaml 5"
  exit 1
fi

OUTPUT_FILE="$1"
NUM_CLIENTS="$2"

cat > "$OUTPUT_FILE" <<'YAML'
name: tp0
services:
  server:
    container_name: server
    image: server:latest
    entrypoint: python3 /main.py
    environment:
      - PYTHONUNBUFFERED=1
      - LOGGING_LEVEL=DEBUG
      - CONFIG_FILE=/config/config.ini
    networks:
      - testing_net
    volumes:
      - ./server/config.ini:/config/config.ini:ro
YAML



for i in $(seq 1 "$NUM_CLIENTS"); do
  cat >> "$OUTPUT_FILE" <<YAML

  client${i}:
    container_name: client${i}
    image: client:latest
    entrypoint: /client
    environment:
      - CLI_ID=${i}
      - CLI_LOG_LEVEL=DEBUG
      - CONFIG_FILE=/config/config.yaml
    volumes:
      - ./client/config.yaml:/config/config.yaml:ro
    networks:
      - testing_net
    depends_on:
      - server
YAML
done

cat >> "$OUTPUT_FILE" <<'YAML'
networks:
  testing_net:
    ipam:
      driver: default
      config:
        - subnet: 172.25.125.0/24
YAML

echo "Archivo generado con exito en: $OUTPUT_FILE"