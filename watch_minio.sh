#!/bin/bash
# monitor_storage.sh
while true; do
    echo "$(date '+%Y/%m/%d %H:%M:%S') $(mc du --recursive local/descargas 2>/dev/null | tail -1)"
    sleep 30
done
