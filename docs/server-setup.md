# Wishlist server setup

Use the PostgreSQL 19 server in the `postgres` container. Create a separate `wishlist` database and login for this app.

Use an image version that has completed publication in GHCR.

## Create the database

On the server, open PostgreSQL as its administrator:

```sh
docker exec -it postgres psql -U postgres -d postgres
```

Run these commands in `psql` once:

```sql
CREATE ROLE wishlist LOGIN;
\password wishlist
CREATE DATABASE wishlist OWNER wishlist;
\q
```

The password prompt is for the new `wishlist` login. Use a new password. You can generate a password in another terminal with:

```sh
openssl rand -hex 24
```

Use this password for `WISHLIST_DATABASE_PASSWORD` below. The hex format does not need URL encoding in the connection string.

The database owner can create the app's tables. Goose creates and migrates them when Wishlist starts. You do not need to create tables or install Goose on the server.

## Set the environment variables

In the directory with your existing server Compose file, open `.env` with Vim:

```sh
vim .env
```

Add these variables with your own values:

```dotenv
WISHLIST_FAMILY_PIN=<4-to-12-digits>
WISHLIST_DATABASE_PASSWORD=<password-for-the-wishlist-login>
```

`WISHLIST_FAMILY_PIN` is the app's family PIN. `WISHLIST_DATABASE_PASSWORD` is the PostgreSQL password. They are separate credentials.

The example is also in `deploy/production.env.example`. It contains no credentials.

## Add the Compose service

Copy the `wishlist` service from `deploy/compose.production.yaml` into the existing Compose file under `services:`. The service must share a Docker network with `postgres`. With no custom networks, services in the same Compose project use its default network.

The service uses port `8050` on the host's loopback interface and port `8080` in the container. Nginx on the host can proxy to `http://127.0.0.1:8050`.

Wishlist stores images and app data in PostgreSQL, so the app needs no `/data` volume. Include the `wishlist` database in the server's backups.

## Start Wishlist

After the image has been published, run these commands from the server Compose directory:

```sh
docker compose config --quiet
docker compose pull wishlist
docker compose up -d wishlist
docker compose logs --tail=100 wishlist
curl -sS -D - -o /dev/null http://127.0.0.1:8050/
```

A redirect to `/unlock` is expected before PIN authentication.

Set the A record for `wishlist.andreasekman.com` to the server address. Configure Nginx and HTTPS for that hostname, with the proxy target above. Allow request bodies of at least `10m` for image uploads.

## Later releases

Change the image version in Compose after a release, then run `docker compose pull wishlist` and `docker compose up -d wishlist`.

References: [Compose environment variables](https://docs.docker.com/compose/how-tos/environment-variables/variable-interpolation/), [PostgreSQL database creation](https://www.postgresql.org/docs/current/sql-createdatabase.html).
