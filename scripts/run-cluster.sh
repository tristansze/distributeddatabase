#!/usr/bin/env bash
set -e

BIN=bin/kvnode
PEERS="node1=127.0.0.1:9001,node2=127.0.0.1:9002,node3=127.0.0.1:9003"

mkdir -p data

echo "Starting 3-node cluster..."

$BIN --id node1 --raft-port 9001 --api-port 8001 --peers "$PEERS" --data-dir data &
$BIN --id node2 --raft-port 9002 --api-port 8002 --peers "$PEERS" --data-dir data &
$BIN --id node3 --raft-port 9003 --api-port 8003 --peers "$PEERS" --data-dir data &

echo "Cluster started. PIDs: $(jobs -p)"
echo "  Node 1: API=8001 Raft=9001"
echo "  Node 2: API=8002 Raft=9002"
echo "  Node 3: API=8003 Raft=9003"
echo ""
echo "Test: curl localhost:8001/raft/status"
echo "Stop: make stop-cluster"

wait
