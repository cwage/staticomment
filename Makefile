.PHONY: build run shell stop

build:
	docker build -t staticomment .

run:
	docker compose up -d

shell:
	docker compose exec staticomment sh

stop:
	docker compose down
