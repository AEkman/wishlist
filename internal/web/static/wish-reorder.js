// Pointer events let the same drag handle work with a mouse or a touch screen.
export function installWishReorder(doc, request = globalThis.fetch) {
  let drag;
  let saving = false;
  let scrollFrame;
  const viewport = doc.defaultView;
  const announce = message => {
    const status = doc.querySelector('[data-reorder-status]');
    if (status) status.textContent = message;
  };
  const updateRanks = list => {
    [...list.children].forEach((card, index) => {
      card.dataset.rank = String(index + 1);
      card.querySelector('.rank-number').textContent = String(index + 1);
    });
  };
  const selection = event => {
    const handle = event.target.closest?.('[data-drag-handle]');
    const card = handle?.closest('[data-wish-id]');
    const list = card?.parentElement;
    return list?.dataset.reorderUrl ? { handle, card, list, before: [...list.children] } : null;
  };
  const save = async state => {
    const rank = [...state.list.children].indexOf(state.card) + 1;
    if (rank === state.before.indexOf(state.card) + 1) return;
    saving = true;
    announce('Sparar ordningen …');
    try {
      const response = await request(state.list.dataset.reorderUrl, {
        method: 'POST',
        body: new URLSearchParams({ wish: state.card.dataset.wishId, rank: String(rank) }),
        credentials: 'same-origin',
      });
      if (!response.ok) throw new Error('Save failed');
      announce('Ordningen är sparad.');
    } catch {
      state.list.append(...state.before);
      updateRanks(state.list);
      announce('Ordningen kunde inte sparas. Försök igen.');
    } finally {
      saving = false;
      state.handle.focus();
    }
  };
  doc.addEventListener('keydown', async event => {
    if (saving || drag || !['ArrowUp', 'ArrowDown'].includes(event.key)) return;
    const state = selection(event);
    if (!state) return;
    event.preventDefault();
    const current = state.before.indexOf(state.card);
    const next = current + (event.key === 'ArrowUp' ? -1 : 1);
    if (next < 0 || next >= state.before.length) return;
    const rest = state.before.filter(card => card !== state.card);
    state.list.insertBefore(state.card, rest[next] || null);
    updateRanks(state.list);
    await save(state);
  });
  const moveToPointer = () => {
    const target = doc.elementFromPoint(drag.x, drag.y)?.closest('[data-wish-id]');
    if (!target || target === drag.card || target.parentElement !== drag.list) return;
    const bounds = target.getBoundingClientRect();
    const rest = [...drag.list.children].filter(card => card !== drag.card);
    const index = rest.indexOf(target) + (drag.y > bounds.top + bounds.height / 2 ? 1 : 0);
    drag.list.insertBefore(drag.card, rest[index] || null);
    updateRanks(drag.list);
  };
  const scrollAtEdge = () => {
    if (!drag) return;
    const edge = 64;
    const speed = 12;
    const delta = drag.y < edge ? -speed : drag.y > viewport.innerHeight - edge ? speed : 0;
    if (delta) {
      viewport.scrollBy(0, delta);
      moveToPointer();
    }
    scrollFrame = viewport.requestAnimationFrame(scrollAtEdge);
  };
  const stopScrolling = () => {
    if (scrollFrame !== undefined) viewport.cancelAnimationFrame(scrollFrame);
    scrollFrame = undefined;
  };
  doc.addEventListener('pointerdown', event => {
    if (saving || drag || event.button !== 0) return;
    const state = selection(event);
    if (!state) return;
    event.preventDefault();
    drag = { ...state, pointerId: event.pointerId, x: event.clientX, y: event.clientY };
    // The list stays in place while its cards move.
    state.list.setPointerCapture(event.pointerId);
    if (viewport?.requestAnimationFrame) scrollFrame = viewport.requestAnimationFrame(scrollAtEdge);
    state.card.classList.add('is-dragging');
  });
  doc.addEventListener('pointermove', event => {
    if (!drag || event.pointerId !== drag.pointerId) return;
    drag.x = event.clientX;
    drag.y = event.clientY;
    moveToPointer();
  });
  doc.addEventListener('pointerup', async event => {
    if (!drag || event.pointerId !== drag.pointerId) return;
    const state = drag;
    stopScrolling();
    drag = null;
    state.card.classList.remove('is-dragging');
    await save(state);
  });
  const cancelDrag = event => {
    if (!drag || event.pointerId !== drag.pointerId) return;
    stopScrolling();
    drag.list.append(...drag.before);
    updateRanks(drag.list);
    drag.card.classList.remove('is-dragging');
    drag = null;
  };
  doc.addEventListener('pointercancel', cancelDrag);
  doc.addEventListener('lostpointercapture', cancelDrag);
}

if (typeof document !== 'undefined') installWishReorder(document);
