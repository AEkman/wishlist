-- +goose Up
CREATE TABLE guided_rankings (
    wishlist_id TEXT PRIMARY KEY REFERENCES wishlists(id),
    state TEXT NOT NULL
);

-- +goose Down
DROP TABLE guided_rankings;
