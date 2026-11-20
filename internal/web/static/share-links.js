export function installShareLinks(doc, nav) {
  doc.addEventListener('click', async (event) => {
    const copyButton = event.target?.closest?.('[data-copy-share]');
    if (copyButton) {
      event.preventDefault();
      const details = copyButton.closest('details');
      const input = details.querySelector('input[type="url"]');
      const status = details.querySelector('[data-share-status]');
      status.textContent = '';
      try {
        await nav.clipboard.writeText(input.value);
        status.textContent = 'Länken är kopierad.';
      } catch {
        input.focus();
        input.select();
        status.textContent = 'Kopiera länken manuellt.';
      }
      return;
    }

    const summary = event.target?.closest?.('.share-action > summary');
    if (!summary || typeof nav.share !== 'function') return;
    event.preventDefault();
    const details = summary.parentElement;
    const status = details.querySelector('[data-share-status]');
    status.textContent = '';
    const input = details.querySelector('input[type="url"]');
    try {
      await nav.share({ url: input.value });
    } catch (error) {
      if (error?.name === 'AbortError') return;
      details.open = true;
      status.textContent = 'Det gick inte att dela. Kopiera länken istället.';
    }
  });
}

if (globalThis.document) {
  installShareLinks(globalThis.document, globalThis.navigator);
}
