.PHONY: build run shell stop test test-unit test-clean

build:
	docker build -t staticomment .

# Go unit tests, run in a container so no local Go toolchain is required.
test-unit:
	docker run --rm -v "$(CURDIR)":/src -w /src golang:1.23-alpine \
		sh -c 'test -z "$$(gofmt -l .)" && go vet ./... && go test ./... -count=1'

run:
	docker compose up -d

shell:
	docker compose exec staticomment sh

stop:
	docker compose down

test:
	docker compose -f test/docker-compose.test.yml build
	docker compose -f test/docker-compose.test.yml run --rm test-runner; \
	EXIT_CODE=$$?; \
	docker compose -f test/docker-compose.test.yml down -v; \
	exit $$EXIT_CODE

test-clean:
	docker compose -f test/docker-compose.test.yml down -v --rmi local
