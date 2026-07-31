.PHONY: build clean tool lint help test statistic-acceptance kafka-init db-init db-run db-shell db-reset

all: build

build:
	go build -v .

tool:
	go tool vet . |& grep -v vendor; true
	gofmt -w .

lint:
	golint ./...

clean:
	rm -rf go-gin-example
	go clean -i .

test:
	go test ./...

statistic-acceptance:
	MLC_STATISTIC_INTEGRATION=1 go test ./internal/consumer/statistic -run TestHGStatisticIntegrationKafkaClickHouseRedisReconcile -count=1 -v

kafka-init:
	@./scripts/kafka_init.sh

db-init:
	@./scripts/db.sh

db-run:
	@if [ -z "$(SQL)" ]; then \
		echo "[Info]: 用法： make db-run SQL=path/to/file.sql"; \
		exit 1; \
	fi
	@./scripts/db.sh run "$(SQL)"

db-shell:
	@./scripts/db.sh shell

db-reset:
	@mysql -uroot -phh109 -e "DROP DaTABASE IF EXISTS HG_MLC_DB;"
	@./scripts/db.sh init

help:
	@echo "make: compile packages and dependencies"
	@echo "make tool: run specified go tool"
	@echo "make lint: golint ./..."
	@echo "make clean: remove object files and cached files"
	@echo "make test: run go tests"
	@echo "make statistic-acceptance: verify Kafka -> ClickHouse -> Redis -> reconciliation"
	@echo "make kafka-init: ensure the local Kafka business topic exists"
