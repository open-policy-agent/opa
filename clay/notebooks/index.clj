(ns index
  (:require [opa-bench.data :as data]
            [opa-bench.charts :as charts]
            [opa-bench.style :as style]))

style/page-style

(style/page-title "OPA Benchmarks")

(charts/index-table data/benchmarks-with-ids)
