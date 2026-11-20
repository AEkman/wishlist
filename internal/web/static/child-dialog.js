document.addEventListener("click", event => {
  const open = event.target.closest("[data-add-child-open]");
  if (open) {
    const dialog = document.getElementById(open.getAttribute("aria-controls"));
    dialog.showModal();
    dialog.querySelector("input").focus();
    return;
  }
  const close = event.target.closest("[data-add-child-close]");
  if (close) close.closest("dialog").close();
});
