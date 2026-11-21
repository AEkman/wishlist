export function installImagePreview(doc) {
  const dialog = doc.getElementById('wish-image-dialog');
  const image = doc.getElementById('wish-image-large');
  const title = doc.getElementById('wish-image-title');
  const rankAction = doc.getElementById('wish-image-rank');
  let opener;
  dialog?.addEventListener('close', () => {
    doc.documentElement.classList.remove('image-preview-open');
    image.removeAttribute('src');
    if (opener?.isConnected) opener.focus();
    opener = null;
  });
  dialog?.addEventListener('click', (event) => {
    if (event.target !== dialog) return;
    const bounds = dialog.getBoundingClientRect();
    if (event.clientX < bounds.left || event.clientX > bounds.right || event.clientY < bounds.top || event.clientY > bounds.bottom) dialog.close();
  });
  doc.addEventListener('click', (event) => {
    if (event.defaultPrevented || event.button > 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    const trigger = event.target?.closest?.('[data-image-preview]');
    if (!trigger || typeof dialog?.showModal !== 'function') return;
    event.preventDefault();
    opener = trigger;
    image.src = trigger.href;
    image.alt = trigger.dataset.imageTitle;
    title.textContent = trigger.dataset.imageTitle;
    if (rankAction) {
      rankAction.hidden = !trigger.dataset.rankUrl;
      if (trigger.dataset.rankUrl) rankAction.href = trigger.dataset.rankUrl;
      else rankAction.removeAttribute('href');
    }
    dialog.showModal();
    dialog.querySelector('[data-image-close]').focus();
    doc.documentElement.classList.add('image-preview-open');
  });
}
if (globalThis.document) installImagePreview(globalThis.document);
