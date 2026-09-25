(ns opa-bench.data
  (:require [clojure.data.json :as json]
            [clojure.java.io :as io]
            [babashka.http-client :as http]))

(defn- github-fetch [path]
  (let [headers (cond-> {"Accept" "application/vnd.github.v3+json"}
                  (System/getenv "GITHUB_TOKEN")
                  (assoc "Authorization" (str "token " (System/getenv "GITHUB_TOKEN"))))
        resp (http/get (str "https://api.github.com/repos/open-policy-agent/opa/" path)
                       {:headers headers})]
    (json/read-str (:body resp) :key-fn keyword)))

(def benchlab-raw
  (let [f (io/file "../benchlab.json")]
    (if (.exists f)
      (json/read-str (slurp f) :key-fn keyword)
      [])))

(def ^:private max-commit-pages
  "Safety bound on pagination: 20 pages of 100 is 2000 commits of main."
  20)

(defn- fetch-commits-covering
  "Commit metadata paged back far enough to cover every commit in `wanted`.

   Nightly heads are a sparse subset of main, so a fixed slice of main would
   hold only some of them.

   Stops at max-commit-pages so that a commit which is no longer reachable from
   main (rebased away, force-pushed) can't cause unbounded paging. Anything
   still unresolved is reported and simply plots without commit details."
  [wanted]
  (loop [page 1 acc []]
    (let [batch (github-fetch (str "commits?per_page=100&page=" page))
          acc   (into acc batch)
          seen  (into #{} (map :sha) acc)
          missing (remove seen wanted)]
      (cond
        (empty? missing) acc
        (empty? batch)   (do (println (format "warning: %d benchmarked commit(s) are not reachable from main; their points will plot without commit details"
                                              (count missing)))
                             acc)
        (>= page max-commit-pages)
        (do (println (format "warning: %d benchmarked commit(s) not found in the last %d commits of main; their points will plot without commit details"
                             (count missing) (* 100 max-commit-pages)))
            acc)
        :else (recur (inc page) acc)))))

(def commits-raw
  (fetch-commits-covering (into #{} (map :head) benchlab-raw)))

(def commits
  (into {}
        (map (fn [{:keys [sha commit author]}]
               [sha {:message (:message commit)
                     :author  (:login author)
                     :date    (get-in commit [:author :date])}]))
        commits-raw))

(defn commit-info
  "Metadata for `sha`, falling back to a placeholder rather than nil.

   A point whose commit metadata could not be fetched should still appear on the
   chart with a thinner hover panel; dropping it instead loses a real
   measurement to an unrelated API shortfall."
  [sha]
  (or (commits sha)
      {:message "(commit details unavailable)"
       :author  "unknown"
       :date    nil}))

(def commits-ordered
  "All known commits on main, oldest first. The GitHub API returns newest-first."
  (->> commits-raw (map :sha) reverse vec))

(def tags-raw
  (github-fetch "tags?per_page=100"))

(def tag-map
  (into {} (map (fn [{:keys [name commit]}] [(:sha commit) name])) tags-raw))

(def latest-tag
  "The tag the newest night was measured against."
  (:baseline_tag (last (sort-by :date benchlab-raw))))

(defn benchmark-id [pkg name]
  (clojure.string/replace (str pkg "_" name) #"[^a-zA-Z0-9]" "-"))

(def benchlab-nights
  "Nights anchored to the same tag the charts are, oldest first.

   A night measured against a different baseline reports percentages relative to
   another commit, so plotting it on this axis would be quietly wrong by however
   much the two baselines differ. Such nights are dropped rather than rescaled,
   because that offset is not knowable from this data -- it takes a run that
   measures both baselines together. After a release the charts restart from
   the first night measured against the new tag."
  (let [usable  (filter #(= (:baseline_tag %) latest-tag) benchlab-raw)
        dropped (- (count benchlab-raw) (count usable))]
    (when (pos? dropped)
      (println (format "note: ignoring %d benchlab night(s) not anchored to %s"
                       dropped latest-tag)))
    (vec (sort-by :date usable))))

(def benchlab-series
  "[pkg name measure] -> points, oldest first.

   :ci-pct is the confidence interval benchstat reports for the HEAD arm alone,
   not an interval on the delta -- benchstat does not expose the latter. It is
   drawn as an error bar because it is the best available indication of spread,
   but :significant is the trustworthy verdict on whether anything moved.

   Only :vs_baseline is read here. bench-nightly strips the other two
   comparisons from every night but the newest, since they exist for alerting and
   for computing that night's calibration, both of which have already happened by
   the time a night reaches this file."
  (->> (for [night (reverse benchlab-nights)
             r     (:results night)]
         [[(:pkg r) (:name r) (:measure r)]
          {:date        (:date night)
           :commit      (:head night)
           :ratio       (+ 1 (/ (double (get-in r [:vs_baseline :pct] 0)) 100))
           :ci-pct      (double (or (:head_ci_pct r) 0))
           :significant (boolean (get-in r [:vs_baseline :significant]))
           :calibration (:calibration night)}])
       ;; Nights are walked newest first and each point conj'd onto the front of
       ;; its benchmark's list, so every list comes out oldest first.
       (reduce (fn [acc [k point]] (update acc k conj point)) {})))

(def benchlab-latest-ratios
  "[pkg name measure] -> the most recent night's ratio."
  (into {}
        (map (fn [[k points]] [k (:ratio (last points))]))
        benchlab-series))

(def benchlab-keys
  "[pkg name] pairs the nightly experiment covers."
  (into #{} (map (fn [[pkg name _measure]] [pkg name])) (keys benchlab-series)))

(def benchlab-sparklines
  "[pkg name] -> NsPerOp ratio history from the nightly experiment, oldest first."
  (into {}
        (keep (fn [[[pkg name measure] points]]
                (when (= measure "NsPerOp")
                  [[pkg name] (mapv :ratio points)])))
        benchlab-series))

(def benchlab-benchmarks-with-ids
  "The dashboard's benchmark list: :pkg :name :id :spark plus a ratio per measure."
  (->> benchlab-keys
       (map (fn [[pkg name]]
              (merge {:pkg pkg :name name :id (benchmark-id pkg name)
                      :spark (get benchlab-sparklines [pkg name])}
                     (into {}
                           (keep (fn [measure]
                                   (when-let [r (get benchlab-latest-ratios [pkg name measure])]
                                     [measure r])))
                           ["NsPerOp" "AllocsPerOp" "BytesPerOp"]))))
       (sort-by #(get % "NsPerOp" 0))))
