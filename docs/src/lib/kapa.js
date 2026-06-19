let buttonHidden = false;

function hideKapaButton() {
  if (buttonHidden) return;
  const container = document.getElementById("kapa-widget-container");
  const button = container?.shadowRoot?.getElementById("kapa-button");
  if (button) {
    button.style.display = "none";
    buttonHidden = true;
    return;
  }
  // Container or button not yet rendered — watch body for it to appear.
  // MutationObserver cannot see into shadow roots, so we observe the light DOM
  // for the container itself and then access its shadowRoot directly.
  const observer = new MutationObserver(() => {
    const c = document.getElementById("kapa-widget-container");
    const b = c?.shadowRoot?.getElementById("kapa-button");
    if (b) {
      b.style.display = "none";
      buttonHidden = true;
      observer.disconnect();
    }
  });
  observer.observe(document.body, { childList: true, subtree: true });
}

// Runs after each page navigation — the kapa widget re-renders on route changes
// so we re-check each time. The module-level flag short-circuits once hidden.
export function onRouteDidUpdate() {
  hideKapaButton();
}
