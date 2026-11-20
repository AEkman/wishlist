import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const source = await readFile(new URL('./static/share-links.js', import.meta.url), 'utf8');
const { installShareLinks } = await import('data:text/javascript;base64,' + Buffer.from(source).toString('base64'));

function fixture(nav, kind = 'summary') {
  let listener;
  const input = { value: 'https://wishlist.example/family/token', focused: false, selected: false,
    focus() { this.focused = true; }, select() { this.selected = true; } };
  const status = { textContent: '' };
  const details = { open: false, querySelector(selector) { return selector === 'input[type="url"]' ? input : status; } };
  const summary = { parentElement: details };
  const copy = { closest() { return details; } };
  const doc = { addEventListener(type, callback) { assert.equal(type, 'click'); listener = callback; } };
  installShareLinks(doc, nav);
  const event = { prevented: false, preventDefault() { this.prevented = true; }, target: {
    closest(selector) { if (selector === '[data-copy-share]') return kind === 'copy' ? copy : null;
      return kind === 'summary' ? summary : null; }
  } };
  return { input, status, details, event, click: () => listener(event) };
}

test('opens the native share sheet with only the public URL', async () => {
  let shared;
  const view = fixture({ async share(data) { shared = data; } });
  await view.click();
  assert.deepEqual(shared, { url: view.input.value });
  assert.equal(view.event.prevented, true);
  assert.equal(view.status.textContent, '');
});

test('cancelling native sharing does not show an error or open the fallback', async () => {
  const view = fixture({ async share() { throw Object.assign(new Error('cancel'), { name: 'AbortError' }); } });
  await view.click();
  assert.equal(view.details.open, false);
  assert.equal(view.status.textContent, '');
});

test('native share errors expose the copy fallback', async () => {
  const view = fixture({ async share() { throw new Error('denied'); } });
  await view.click();
  assert.equal(view.details.open, true);
  assert.match(view.status.textContent, /Kopiera länken/);
});

test('copies the URL and announces completion', async () => {
  let copied;
  const view = fixture({ clipboard: { async writeText(value) { copied = value; } } }, 'copy');
  await view.click();
  assert.equal(copied, view.input.value);
  assert.equal(view.status.textContent, 'Länken är kopierad.');
  assert.equal(view.event.prevented, true);
});

test('selects the URL for manual copying when clipboard access fails', async () => {
  const view = fixture({ clipboard: { async writeText() { throw new Error('denied'); } } }, 'copy');
  await view.click();
  assert.equal(view.input.focused, true);
  assert.equal(view.input.selected, true);
  assert.equal(view.status.textContent, 'Kopiera länken manuellt.');
});

test('leaves details usable when native sharing is unavailable', async () => {
  const view = fixture({});
  await view.click();
  assert.equal(view.event.prevented, false);
});

test('selects the URL when the clipboard API is unavailable', async () => {
  const view = fixture({}, 'copy');
  await view.click();
  assert.equal(view.input.selected, true);
  assert.equal(view.status.textContent, 'Kopiera länken manuellt.');
});

test('uses controls inserted after listener installation', async () => {
  let listener;
  let shared;
  installShareLinks({ addEventListener(_, callback) { listener = callback; } }, { async share(data) { shared = data; } });
  const details = { querySelector(selector) { return selector === 'input[type="url"]' ? { value: 'https://wishlist.example/new' } : { textContent: '' }; } };
  const summary = { parentElement: details };
  await listener({ preventDefault() {}, target: { closest(selector) { return selector === '[data-copy-share]' ? null : summary; } } });
  assert.deepEqual(shared, { url: 'https://wishlist.example/new' });
});
