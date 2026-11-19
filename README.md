# Wishlist

Wishlist is a self-hosted app for families. Children rank gifts by choosing between two pictures. Relatives can mark gifts as bought with a shared link.

## Run with Docker Compose

Install Docker with the Compose plugin. From the project directory, run `make start` or `make run`. Open http://127.0.0.1:8080 on the Docker host.

The app uses the Compose volume `wishlist_data` for its SQLite database. This database stores wishlists, images, rankings, and purchase marks. Run `make stop` to stop the app. Run `make start` to start it again with the same data.

Docker publishes the app on the host's loopback address. For a deployment behind a reverse proxy on the same host, forward requests to `127.0.0.1:8080`. Set `BASE_URL` to the public HTTPS origin to generate share links for that domain, for example `BASE_URL=https://wishlist.example.org make start`. Set `PORT` to change the host port.

Anyone who can reach the app can change wishlists until PIN support is available. A shared link shows the full wishlist and lets visitors mark or unmark gifts as bought. Purchase marks appear in the shared view.

## Develop and test

Use Go 1.26 or later. Run `go run ./cmd/wishlist` to start the app. Set `LISTEN_ADDR` to change the default address `:8080`. Set `DATABASE_PATH` to change the default SQLite file `wishlists.db`.

Run `go test ./...` or `make test`. Each Godog scenario uses a fresh in-memory SQLite database. See the [acceptance scenarios](features/). The PIN scenarios have the `@wip` tag and are deferred.
