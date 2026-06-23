#!/bin/sh
# Bootstrap Mosquitto dynamic security plugin on first run.
set -e

DYNSEC_FILE=/mosquitto/data/dynamic-security.json

if [ -z "${MQTT_USERNAME}" ]; then
    echo "ERROR: MQTT_USERNAME is required" >&2
    exit 1
fi

if [ -z "${MQTT_PASSWORD}" ]; then
    echo "ERROR: MQTT_PASSWORD is required" >&2
    exit 1
fi

# Initialise dynsec config only on first boot (file absent = fresh volume).
if [ ! -f "${DYNSEC_FILE}" ]; then
    echo "Initialising dynamic security config..."
    mosquitto_ctrl dynsec init "${DYNSEC_FILE}" "${MQTT_USERNAME}" "${MQTT_PASSWORD}"
    echo "Dynamic security initialised with admin user: ${MQTT_USERNAME}"
fi

exec /docker-entrypoint.sh "$@"
