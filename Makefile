BUILD_DIR := bin

.PHONY: build clean run-cluster stop-cluster

build:
	go build -o $(BUILD_DIR)/kvnode ./cmd/server

clean:
	rm -rf $(BUILD_DIR) data

run-cluster: build
	@bash scripts/run-cluster.sh

stop-cluster:
	@bash scripts/stop-cluster.sh
