(ns index
  (:require [opa-bench.data :as data]
            [opa-bench.charts :as charts]
            [opa-bench.style :as style]))

style/page-style

(style/glitch-title "OPA Benchmarks")

(charts/index-table data/benchmarks-with-ids)
