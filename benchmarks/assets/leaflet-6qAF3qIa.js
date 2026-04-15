import * as _esm from "https://cdn.jsdelivr.net/npm/leaflet/+esm";
import { Icon } from "https://cdn.jsdelivr.net/npm/leaflet/+esm";
export * from "https://cdn.jsdelivr.net/npm/leaflet/+esm";
Icon.Default.imagePath = "https://cdn.jsdelivr.net/npm/leaflet/dist/images/";
const link = document.createElement("link");
link.rel = "stylesheet";
link.type = "text/css";
link.href = "https://cdn.jsdelivr.net/npm/leaflet/dist/leaflet.css";
const loaded = new Promise((resolve, reject) => {
  link.onload = resolve;
  link.onerror = reject;
});
document.head.appendChild(link);
await loaded;
