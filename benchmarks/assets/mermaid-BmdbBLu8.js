import mer from "https://cdn.jsdelivr.net/npm/mermaid/dist/mermaid.esm.min.mjs/+esm";
let nextId = 0;
const scheme = getComputedStyle(document.body).getPropertyValue("color-scheme");
const theme = scheme === "dark" ? "dark" : "neutral";
mer.initialize({ startOnLoad: false, securityLevel: "loose", theme });
const mermaid = async (template, ...values) => {
  const source = String.raw.call(String, template, ...values);
  const root = document.createElement("div");
  root.innerHTML = (await mer.render(`mermaid-${++nextId}`, source)).svg;
  return root.removeChild(root.firstChild);
};
export {
  mermaid
};
