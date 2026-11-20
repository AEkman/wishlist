const photoLongEdge = 1600;

function initializeWishCamera(root) {
  const panel = root.querySelector("[data-wish-camera]");
  if (!panel?.hidden) return;

  const form = panel.closest("form");
  const imageInput = form.querySelector("#wish-image");
  const photo = form.querySelector("[data-camera-photo]");
  const dialog = panel.querySelector("dialog");
  const video = dialog.querySelector("video");
  const open = panel.querySelector("[data-camera-open]");
  const snapshot = dialog.querySelector("[data-camera-snapshot]");
  const note = panel.querySelector("[data-camera-note]");
  const progress = dialog.querySelector("[data-camera-loading]");
  const lifecycle = new AbortController();
  let activeRequest;

  function closeCamera() {
    activeRequest = undefined;
    video.srcObject?.getTracks().forEach(track => track.stop());
    video.srcObject = null;
    if (dialog.open) dialog.close();
  }

  function reportCameraError(message) {
    closeCamera();
    note.textContent = message;
  }

  open.addEventListener("click", async () => {
    note.textContent = "";
    if (!window.isSecureContext) {
      note.textContent = "Kameran kräver en säker anslutning (HTTPS). Du kan också välja en bild.";
      return;
    }
    if (!navigator.mediaDevices?.getUserMedia || !dialog.showModal || typeof DataTransfer !== "function") {
      note.textContent = "Kameran är inte tillgänglig i den här webbläsaren. Välj en bild i stället.";
      return;
    }

    const request = Symbol();
    activeRequest = request;
    snapshot.disabled = true;
    progress.textContent = "Väntar på kameran …";
    dialog.showModal();
    try {
      const stream = await navigator.mediaDevices.getUserMedia({
        audio: false,
        video: { facingMode: { ideal: "environment" }, width: { ideal: photoLongEdge } },
      });
      if (activeRequest !== request) {
        stream.getTracks().forEach(track => track.stop());
        return;
      }
      stream.getVideoTracks()[0].addEventListener("ended", () => {
        if (activeRequest === request) reportCameraError("Kameran kopplades från. Försök igen eller välj en bild.");
      });
      video.srcObject = stream;
      await video.play();
      if (activeRequest === request) {
        progress.textContent = "";
        snapshot.disabled = false;
      }
    } catch (error) {
      if (activeRequest !== request) return;
      const message = error.name === "NotAllowedError"
        ? "Tillåt kameran i webbläsaren eller välj en bild."
        : "Kunde inte öppna kameran. Försök igen eller välj en bild.";
      reportCameraError(message);
    }
  });

  snapshot.addEventListener("click", async () => {
    const request = activeRequest;
    snapshot.disabled = true;
    progress.textContent = "Tar bilden …";
    try {
      const width = video.videoWidth;
      const height = video.videoHeight;
      if (!width || !height) throw new Error("Camera has no frame");

      const scale = Math.min(1, photoLongEdge / Math.max(width, height));
      const canvas = document.createElement("canvas");
      canvas.width = Math.max(1, Math.round(width * scale));
      canvas.height = Math.max(1, Math.round(height * scale));
      canvas.getContext("2d").drawImage(video, 0, 0, canvas.width, canvas.height);
      const blob = await new Promise(resolve => canvas.toBlob(resolve, "image/jpeg", 0.88));
      if (activeRequest !== request) return;
      if (!blob || blob.type !== "image/jpeg") throw new Error("Could not encode JPEG");

      const transfer = new DataTransfer();
      transfer.items.add(new File([blob], "kamera.jpg", { type: blob.type }));
      imageInput.files = transfer.files;
      canvas.setAttribute("role", "img");
      canvas.setAttribute("aria-label", "Bild från kameran");
      photo.replaceChildren(canvas);
      photo.hidden = false;
      closeCamera();
      note.textContent = "Bilden är redo att spara.";
    } catch {
      if (activeRequest !== request) return;
      progress.textContent = "Kunde inte ta bilden. Försök igen.";
      snapshot.disabled = false;
    }
  });

  dialog.querySelector("[data-camera-close]").addEventListener("click", closeCamera);
  dialog.addEventListener("cancel", event => {
    event.preventDefault();
    closeCamera();
  });
  imageInput.addEventListener("change", () => {
    photo.hidden = true;
    photo.replaceChildren();
    note.textContent = "";
  });
  window.addEventListener("pagehide", closeCamera, { signal: lifecycle.signal });
  form.addEventListener("htmx:before:cleanup", event => {
    if (event.target !== form) return;
    closeCamera();
    lifecycle.abort();
  });
  panel.hidden = false;
}

document.body.addEventListener("htmx:after:process", event => initializeWishCamera(event.target));
initializeWishCamera(document);
