.PHONY: build clean tool lint help test db-init db-run db-shell db-reset

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

db-init:
	@./scripts/db.sh

db-run:
	@if [ -z "$(SQL)" ]; then \
		echo "[Info]: 用法： make db-run SQL=path/to/file.sql"; \
		exit 1; \
	fi
	@./scripts/db.sh run

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