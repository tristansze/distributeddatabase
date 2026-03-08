#!/usr/bin/env bash
echo "Stopping kvnode processes..."
pkill -f 'kvnode --id' 2>/dev/null || echo "No kvnode processes found."
