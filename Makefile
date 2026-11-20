.DEFAULT_GOAL := help

.PHONY: help start run stop test

help:
	@printf '%s\n' 'Available targets:' '  help   Show available targets (default)' '  start  Build and start the app in the background' '  run    Alias for start' '  stop   Stop the app' '  test   Run Go tests'

start:
	docker compose up --build --detach --wait

run: start

stop:
	docker compose stop

test:
	go test ./...
