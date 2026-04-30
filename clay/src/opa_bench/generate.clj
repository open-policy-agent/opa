(ns opa-bench.generate
  (:require [opa-bench.data :as data]
            [scicloj.clay.v2.api :as clay]
            [clojure.java.io :as io]))

(defn generate-benchmark-source! [{:keys [id pkg name]}]
  (let [dir  (io/file "notebooks/benchmarks")
        file (io/file dir (str id ".clj"))]
    (.mkdirs dir)
    (spit file
          (format "(ns benchmarks.%s
  (:require [opa-bench.charts :as charts]
            [opa-bench.style :as style]
            [scicloj.kindly.v4.kind :as kind]))

style/page-style

(style/page-title %s)

;; **Package:** `%s`

;; [Back to index](index.html)

(charts/benchmark-chart %s %s)
"
                  id (pr-str name) pkg (pr-str pkg) (pr-str name)))
    file))

(defn render-all! []
  (println "Generating source files...")
  (doseq [b data/benchmarks-with-ids]
    (generate-benchmark-source! b))
  (println (str "Generated " (count data/benchmarks-with-ids) " source files."))

  (println "Rendering index...")
  (clay/make! {:source-path    "notebooks/index.clj"
               :format         [:html]
               :base-target-path "../docs"
               :title          "OPA Benchmarks"
               :hide-code      true
               :hide-info-line true
               :show           false
               :live-reload    false})

  (println "Rendering benchmark pages...")
  (doseq [b data/benchmarks-with-ids]
    (let [src (str "notebooks/benchmarks/" (:id b) ".clj")]
      (print (str "  " (:id b) "... ")) (flush)
      (clay/make! {:source-path      src
                   :format           [:html]
                   :base-target-path "../docs"
                   :title            (:name b)
                   :hide-code        true
                   :hide-info-line   true
                   :show             false
                   :live-reload      false})
      (println "done.")))

  (println (str "\nDone. " (count data/benchmarks-with-ids) " pages in ../docs/")))

(defn -main [& _args]
  (render-all!)
  (shutdown-agents))
