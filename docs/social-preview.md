# Social link previews

Pages include Open Graph and Twitter card metadata. The preview uses the same branded artwork for all lists. List contents and purchase marks are not part of the preview metadata.

The JPEG is `internal/web/static/share-card-v1.jpg`: 1200 × 630 pixels, about 127 KiB. It is built into the app and available without a PIN. Preview URLs use `BASE_URL`, with the request origin as the local-development fallback.

Unlock and administration pages use the app root as their preview URL. Shared lists retain their own URL. Purchase forms use the list URL.

After deployment, existing Messenger previews can remain cached. Use [Meta's Sharing Debugger](https://developers.facebook.com/tools/debug/) to request a new scrape. Test the public root URL first. The preview can change after the next scrape; old messages may retain their previous preview.

The asset URL includes `v1`. Use a new filename for a future replacement, so cached shares can retain the previous image.

## Image source

The built-in `imagegen` tool created the artwork from the existing `wishlist-hero.webp` image. The output was resized and encoded as a JPEG for delivery. The homepage artwork was not replaced.

The generation prompt was:

Create an Open Graph social sharing banner for the Swedish family wishlist web app Önskelistan. Use the attached image as an EDIT TARGET and style source: retain the recognisable polished pastel gift boxes, lavender satin ribbon, coral top gift, yellow and mint lower gifts, soft purple backdrop, cream stars and warm photographic 3D paper texture. Recompose into a wide landscape social preview image, ideally exactly 1200 by 630 pixels, 1.91:1. Left half: clean warm ivory #fcf9f4 background, generous whitespace, dark plum elegant editorial serif typography. Exact large title text: “Önskelistan”. Under it exact Swedish tagline on two lines: “Små önskningar.” then “Stora leenden.”. Right half: richly detailed gift composition against lavender purple, filling the area but preserving the entire main bow, tasteful cream stars, the lower gifts grounded in soft shadows. Refined playful Nordic family brand, legible as a Messenger or WhatsApp preview thumbnail, polished balanced layout. Keep text at least 70px from all edges. No website URL, no extra text, no child names, no private gift entries, no app interface, no external logos or watermark. Preserve Swedish Ö and å accurately.
