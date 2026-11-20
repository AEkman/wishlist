-- +goose Up

 CREATE TABLE IF NOT EXISTS device_sessions (
  token_hash BYTEA PRIMARY KEY,
  expires_at BIGINT NOT NULL
 );
 CREATE TABLE IF NOT EXISTS children (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  number BIGINT GENERATED ALWAYS AS IDENTITY UNIQUE
 );
 CREATE TABLE IF NOT EXISTS wishlists (
  id TEXT PRIMARY KEY,
  child_id TEXT NOT NULL REFERENCES children(id),
  title TEXT NOT NULL,
  share_id TEXT NOT NULL UNIQUE,
  family_visible INTEGER NOT NULL DEFAULT 0 CHECK (family_visible IN (0, 1)),
  created_order BIGINT GENERATED ALWAYS AS IDENTITY
 );
 CREATE INDEX IF NOT EXISTS wishlists_child_id ON wishlists(child_id);
 CREATE TABLE IF NOT EXISTS wishes (
  id TEXT PRIMARY KEY,
  wishlist_id TEXT NOT NULL REFERENCES wishlists(id),
  title TEXT NOT NULL,
  description TEXT NOT NULL,
  media_type TEXT NOT NULL,
  image BYTEA NOT NULL,
  position INTEGER NOT NULL,
  buyer TEXT NOT NULL DEFAULT '',
  image_revision INTEGER NOT NULL DEFAULT 0
 );
 CREATE INDEX IF NOT EXISTS wishes_rank ON wishes(wishlist_id, position);
 CREATE TABLE IF NOT EXISTS family_sharing (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  share_id TEXT NOT NULL UNIQUE
 );

INSERT INTO family_sharing (id, share_id)
VALUES (1, gen_random_uuid()::text)
ON CONFLICT (id) DO NOTHING;

DROP TABLE IF EXISTS wishlist_schema_migrations;

-- +goose Down
DROP TABLE family_sharing;
DROP TABLE wishes;
DROP TABLE wishlists;
DROP TABLE children;
DROP TABLE device_sessions;
