(ns opa-bench.style
  (:require [scicloj.kindly.v4.kind :as kind]))

(def page-style
  (kind/hiccup
    [:style "
@import url('https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;700&display=swap');

:root {
  --pink: #e619a0;
  --cyan: #0bc;
  --purple: #b388ff;
  --bg: #fff;
  --bg-alt: #f8f8f8;
  --fg: #333;
  --fg-muted: #888;
  --border: #e0e0e0;
  --chart-bg: #fff;
  --chart-grid: #eee;
  --chart-baseline: #ccc;
  --tag-line: rgba(230,25,160,0.25);
}

@media (prefers-color-scheme: dark) {
  :root {
    --bg: #111;
    --bg-alt: #1a1a2e;
    --fg: #ddd;
    --fg-muted: #777;
    --border: #333;
    --chart-bg: #111;
    --chart-grid: #282828;
    --chart-baseline: #444;
    --tag-line: rgba(230,25,160,0.35);
  }
}

body {
  font-family: 'JetBrains Mono', monospace !important;
  background: var(--bg) !important;
  color: var(--fg) !important;
}

h1 { color: var(--pink) !important; letter-spacing: 1px; }
p, li { color: var(--fg) !important; }
code { color: var(--pink) !important; }

a { color: var(--pink) !important; text-decoration: none; }
a:hover { text-decoration: underline; }

#commit-info {
  font-family: 'JetBrains Mono', monospace !important;
  background: var(--bg-alt) !important;
  border: 1px solid var(--border) !important;
  border-left: 3px solid var(--pink) !important;
  color: var(--fg) !important;
}

/* glitch title */
.glitch {
  position: relative;
  display: inline-block;
}
.glitch::before, .glitch::after {
  content: attr(data-text);
  position: absolute;
  top: 0; left: 0;
  width: 100%; height: 100%;
  overflow: hidden;
  pointer-events: none;
}
.glitch::before {
  color: var(--cyan);
  clip-path: inset(0 0 60% 0);
  animation: glitch-top 3s infinite linear alternate-reverse;
}
.glitch::after {
  color: var(--pink);
  clip-path: inset(60% 0 0 0);
  animation: glitch-bottom 4s infinite linear alternate-reverse;
}
@keyframes glitch-top {
  0%, 92%  { transform: translate(0); }
  93%      { transform: translate(-2px, -1px); }
  95%      { transform: translate(2px, 1px); }
  97%      { transform: translate(-1px, 0); }
  100%     { transform: translate(0); }
}
@keyframes glitch-bottom {
  0%, 90%  { transform: translate(0); }
  91%      { transform: translate(1px, 1px); }
  94%      { transform: translate(-2px, -1px); }
  97%      { transform: translate(1px, 0); }
  100%     { transform: translate(0); }
}

/* datatables */
table.dataTable thead th {
  background: var(--bg-alt) !important;
  color: var(--fg) !important;
  border-bottom: 2px solid var(--pink) !important;
}
table.dataTable tbody tr { background: var(--bg) !important; }
table.dataTable tbody tr:hover { background: var(--bg-alt) !important; }
table.dataTable tbody td { border-color: var(--border) !important; color: var(--fg) !important; }
.dataTables_wrapper { color: var(--fg) !important; }
.dataTables_wrapper .dataTables_filter input,
.dataTables_wrapper .dataTables_length select {
  background: var(--bg-alt) !important;
  color: var(--fg) !important;
  border: 1px solid var(--border) !important;
  border-radius: 3px;
}
.dataTables_wrapper .dataTables_paginate .paginate_button {
  color: var(--fg-muted) !important;
}
.dataTables_wrapper .dataTables_paginate .paginate_button.current {
  background: var(--pink) !important;
  color: white !important;
  border: none !important;
  border-radius: 3px;
}
.dataTables_wrapper .dataTables_info { color: var(--fg-muted) !important; }
"]))

(defn glitch-title [text]
  (kind/hiccup
    [:h1 {:class "glitch" :data-text text} text]))
