.PHONY: start run stop logs test

start:
	docker compose up --build --detach --wait

run: start

stop:
	docker compose stop

logs:
	docker compose logs --follow --tail=30 wishlist

test:
	go test ./...
