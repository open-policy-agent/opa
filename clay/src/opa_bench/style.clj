(ns opa-bench.style
  (:require [scicloj.kindly.v4.kind :as kind]))

(def page-style
  (kind/hiccup
    [:style "
@import url('https://fonts.googleapis.com/css2?family=Go+Mono&display=swap');

:root {
  --yellow: #ffffea;
  --blue: #eaffff;
  --tag-bg: #e1e1cf;
  --border: #888;
  --fg: #000;
  --fg-muted: #444;
  --link: #268bd2;
  --chart-bg: #ffffea;
  --chart-grid: #e0e0c8;
  --chart-baseline: #aaa;
  --tag-line: rgba(0,0,0,0.15);
}

body {
  font-family: 'Go Mono', monospace !important;
  font-size: 13px;
  background: var(--yellow) !important;
  color: var(--fg) !important;
  margin: 0;
  padding: 12px 16px;
}

h1 {
  background: var(--blue) !important;
  color: var(--fg) !important;
  margin: -12px -16px 12px -16px;
  padding: 6px 16px;
  border-bottom: 1px solid var(--border);
  font-size: 15px;
  font-weight: bold;
  letter-spacing: 0;
}

a { color: var(--link) !important; text-decoration: none; }
a:hover { text-decoration: underline; }

p, code { font-family: 'Go Mono', monospace !important; font-size: 13px; }
code { background: var(--blue); padding: 1px 4px; }

#commit-info {
  font-family: 'Go Mono', monospace !important;
  font-size: 12px;
  background: var(--blue) !important;
  border: 1px solid var(--border) !important;
  color: var(--fg) !important;
}

/* datatables */
table.dataTable { font-size: 12px !important; }
table.dataTable thead th {
  background: var(--blue) !important;
  color: var(--fg) !important;
  border-bottom: 1px solid var(--border) !important;
  font-weight: bold;
}
table.dataTable tbody tr { background: var(--yellow) !important; }
table.dataTable tbody tr:hover { background: var(--tag-bg) !important; }
table.dataTable tbody td { border-color: var(--tag-bg) !important; }
.dataTables_wrapper { color: var(--fg) !important; font-size: 12px; }
.dataTables_wrapper .dataTables_filter input,
.dataTables_wrapper .dataTables_length select {
  background: var(--blue) !important;
  color: var(--fg) !important;
  border: 1px solid var(--border) !important;
  font-family: 'Go Mono', monospace !important;
  font-size: 12px;
}
.dataTables_wrapper .dataTables_paginate .paginate_button {
  color: var(--fg-muted) !important;
  font-size: 12px;
}
.dataTables_wrapper .dataTables_paginate .paginate_button.current {
  background: var(--blue) !important;
  color: var(--fg) !important;
  border: 1px solid var(--border) !important;
}
.dataTables_wrapper .dataTables_info { color: var(--fg-muted) !important; }
"]))

(defn page-title [text]
  (kind/hiccup [:h1 text]))
