import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
const source = await readFile(new URL('./static/wish-reorder.js', import.meta.url), 'utf8');
const { installWishReorder = () => {} } = await import('data:text/javascript;base64,' + Buffer.from(source).toString('base64'));
function fixture(ok = true) {
 const listeners = new Map(), requests = [];
 const status = { textContent: '' };
 const list = { dataset: { reorderUrl: '/lists/list/ranked/move' }, children: [], setPointerCapture() {}, insertBefore(card, before) { this.children = this.children.filter(item => item !== card); const i = before ? this.children.indexOf(before) : this.children.length; this.children.splice(i, 0, card); }, append(...cards) { this.children = [...cards]; } };
 const cards = ['a', 'b', 'c'].map(id => ({ dataset: { wishId: id }, parentElement: list, classList: { add() {}, remove() {} }, querySelector() { return this.number; }, number: { textContent: '' }, getBoundingClientRect() { return { top: list.children.indexOf(this) * 100, height: 100 }; } }));
 list.children = [...cards];
 const handle = { setPointerCapture() {}, closest(selector) { return selector === '[data-drag-handle]' ? this : cards[1]; }, setPointerCapture() {}, focus() {} };
 const doc = { addEventListener(type, fn) { listeners.set(type, fn); }, querySelector() { return status; }, elementFromPoint() { return { closest() { return cards[2]; } }; } };
 installWishReorder(doc, async (url, options) => { requests.push({ url, options }); return { ok }; });
 const event = { target: handle, key: 'ArrowDown', button: 0, pointerId: 1, clientX: 5, clientY: 290, preventDefault() {} };
 return { cards, list, status, requests, event, dispatch(type) { return listeners.get(type)?.(event); } };
}
test('arrow keys move a wish and save its new rank', async () => {
 const view = fixture(); await view.dispatch('keydown');
 assert.deepEqual(view.list.children.map(card => card.dataset.wishId), ['a', 'c', 'b']);
 assert.equal(view.requests[0].url, '/lists/list/ranked/move');
 assert.equal(view.requests[0].options.body.get('wish'), 'b');
 assert.equal(view.requests[0].options.body.get('rank'), '3');
 assert.equal(view.cards[1].number.textContent, '3');
});
test('pointer dragging saves a wish below the pointed card', async () => {
 const view = fixture(); view.dispatch('pointerdown'); view.dispatch('pointermove'); await view.dispatch('pointerup');
 assert.deepEqual(view.list.children.map(card => card.dataset.wishId), ['a', 'c', 'b']);
 assert.equal(view.requests[0].options.body.get('rank'), '3');
});
test('a failed save restores the previous order and reports the failure', async () => {
 const view = fixture(false); await view.dispatch('keydown');
 assert.deepEqual(view.list.children.map(card => card.dataset.wishId), ['a', 'b', 'c']);
 assert.match(view.status.textContent, /inte/);
});
test('cancelled dragging restores the order without saving', () => {
 const view = fixture(); view.dispatch('pointerdown'); view.dispatch('pointermove'); view.dispatch('pointercancel');
 assert.deepEqual(view.list.children.map(card => card.dataset.wishId), ['a', 'b', 'c']);
 assert.equal(view.requests.length, 0);
});
test('dragging near the viewport edge scrolls a long list and stops on release', async () => {
 const listeners = new Map();
 let frame, scroll = 0, cancelled = false;
 const list = { dataset: { reorderUrl: '/move' }, children: [], setPointerCapture() {} };
 const card = { parentElement: list, classList: { add() {}, remove() {} } };
 list.children = [card];
 const handle = { setPointerCapture() {}, closest(selector) { return selector === '[data-drag-handle]' ? this : card; } };
 const doc = { addEventListener(type, fn) { listeners.set(type, fn); }, defaultView: { innerHeight: 800, requestAnimationFrame(fn) { frame = fn; return 1; }, cancelAnimationFrame() { cancelled = true; }, scrollBy(x, y) { scroll += y; } }, elementFromPoint() { return null; } };
 installWishReorder(doc);
 const event = { target: handle, pointerId: 1, button: 0, clientX: 10, clientY: 795, preventDefault() {} };
 listeners.get('pointerdown')(event);
 assert.equal(typeof frame, 'function');
 frame();
 assert.ok(scroll > 0);
 await listeners.get('pointerup')(event);
 assert.equal(cancelled, true);
});
