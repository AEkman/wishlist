.PHONY: start run stop test

start:
	docker compose up --build --detach --wait

run: start

stop:
	docker compose stop

test:
	go test ./...
