import katex from "https://cdn.jsdelivr.net/npm/katex/+esm";
const link = document.createElement("link");
link.href = "https://cdn.jsdelivr.net/npm/katex/dist/katex.min.css";
link.rel = "stylesheet";
document.head.appendChild(link);
const tex = Object.assign(renderer(), {
  options: renderer,
  block: renderer({ displayMode: true })
});
function renderer(options) {
  return function(template, ...values) {
    const source = String.raw.call(String, template, ...values);
    const root = document.createElement("div");
    katex.render(source, root, { ...options, output: "html" });
    return root.removeChild(root.firstChild);
  };
}
export {
  tex
};
