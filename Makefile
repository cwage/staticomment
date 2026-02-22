.PHONY: build run shell stop test test-clean

build:
	docker build -t staticomment .

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
