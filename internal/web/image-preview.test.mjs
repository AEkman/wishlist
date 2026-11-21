import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
const source = await readFile(new URL('./static/image-preview.js', import.meta.url), 'utf8');
const { installImagePreview } = await import('data:text/javascript;base64,' + Buffer.from(source).toString('base64'));
function fixture() {
  const listeners = new Map();
  const dialogListeners = new Map();
  const classes = new Set();
  const image = { src: '', alt: '', removeAttribute(name) { this[name] = ''; } };
  const title = { textContent: '' };
  const rankAction = { hidden: true, href: '', removeAttribute(name) { this[name] = ''; } };
  const closeButton = { focused: false, focus() { this.focused = true; } };
  const dialog = { open: false, showModal() { this.open = true; }, close() { this.open = false; dialogListeners.get('close')?.(); }, addEventListener(type, fn) { dialogListeners.set(type, fn); }, querySelector() { return closeButton; }, getBoundingClientRect() { return { left: 100, right: 700, top: 100, bottom: 600 }; } };
  const elements = { 'wish-image-dialog': dialog, 'wish-image-large': image, 'wish-image-title': title, 'wish-image-rank': rankAction };
  const doc = { getElementById(id) { return elements[id]; }, addEventListener(type, fn) { listeners.set(type, fn); }, documentElement: { classList: { add(name) { classes.add(name); }, remove(name) { classes.delete(name); } } } };
  installImagePreview(doc);
  const trigger = { href: 'https://wishlist.example/images/wish?v=2', dataset: { imageTitle: 'Cykel' }, isConnected: true, focused: false, focus() { this.focused = true; } };
  const event = { defaultPrevented: false, button: 0, preventDefault() { this.defaultPrevented = true; }, target: { closest(selector) { return selector === '[data-image-preview]' ? trigger : null; } } };
  return { doc, dialog, image, title, rankAction, closeButton, trigger, event, classes, listeners, dialogListeners, click() { return listeners.get('click')(event); } };
}
test('opens the selected image at full size with its title', () => {
  const view = fixture(); view.click();
  assert.equal(view.event.defaultPrevented, true);
  assert.equal(view.dialog.open, true);
  assert.equal(view.image.src, view.trigger.href);
  assert.equal(view.image.alt, 'Cykel');
  assert.equal(view.title.textContent, 'Cykel');
  assert.equal(view.closeButton.focused, true);
  assert.equal(view.classes.has('image-preview-open'), true);
});

test('closing restores focus and releases the image and page scroll', () => {
  const view = fixture(); view.click(); view.dialog.close();
  assert.equal(view.trigger.focused, true);
  assert.equal(view.image.src, '');
  assert.equal(view.classes.has('image-preview-open'), false);
});

test('clicking the backdrop closes the image while clicks inside leave it open', () => {
 const view = fixture(); view.click();
 view.dialogListeners.get('click')?.({ target: view.dialog, clientX: 200, clientY: 200 });
 assert.equal(view.dialog.open, true);
 view.dialogListeners.get('click')?.({ target: view.dialog, clientX: 10, clientY: 10 });
 assert.equal(view.dialog.open, false);
 assert.equal(view.trigger.focused, true);
});

test('modified image links retain normal browser navigation', () => {
 for (const changes of [{ metaKey: true }, { ctrlKey: true }, { shiftKey: true }, { altKey: true }, { button: 1 }, { defaultPrevented: true }]) {
  const view = fixture(); Object.assign(view.event, changes); view.click(); assert.equal(view.dialog.open, false);
 }
});

test('unsupported dialogs leave the full-image link usable', () => {
 const view = fixture(); view.dialog.showModal = undefined; view.click();
 assert.equal(view.event.defaultPrevented, false);
});

test('new images after navigation use their own title and URL', () => {
 const view = fixture();
 view.trigger.href = 'https://wishlist.example/images/new';
 view.trigger.dataset.imageTitle = 'Bok';
 view.click();
 assert.equal(view.image.src, view.trigger.href);
 assert.equal(view.title.textContent, 'Bok');
 view.trigger.isConnected = false;
 view.dialog.close();
 assert.equal(view.trigger.focused, false);
});

test('private ranked images offer five comparisons and other previews hide the action', () => {
 const view = fixture();
 view.trigger.dataset.rankUrl = '/lists/list?focus=wish';
 view.click();
 assert.equal(view.rankAction.hidden, false);
 assert.equal(view.rankAction.href, '/lists/list?focus=wish');
 view.dialog.close();
 delete view.trigger.dataset.rankUrl;
 view.event.defaultPrevented = false;
 view.click();
 assert.equal(view.rankAction.hidden, true);
 assert.equal(view.rankAction.href, '');
});
