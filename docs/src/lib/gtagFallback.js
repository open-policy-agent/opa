// Google's gtag script can be missing locally (blocked by an ad-blocker, or
// not injected outside production builds), which makes the official
// @docusaurus/plugin-google-gtag client module throw "window.gtag is not a
// function" and trigger the full-screen dev error overlay. Stub it out so a
// missing gtag just logs instead of crashing the page.
if (typeof window !== "undefined" && typeof window.gtag !== "function") {
  window.gtag = (...args) => {
    console.log("[gtag stub]", ...args);
  };
}
