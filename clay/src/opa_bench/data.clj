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

(def benchmarks-raw
  (json/read-str (slurp (io/file "../benchmarks.json")) :key-fn keyword))

(def data-commits
  "Every commit benchmarks.json holds results for."
  (into #{} (map :Version) benchmarks-raw))

(def ^:private max-commit-pages
  "Safety bound on pagination: 20 pages of 100 is 2000 commits of main."
  20)

(defn- fetch-commits-covering
  "Commit metadata paged back far enough to cover every commit in `wanted`.

   Fetching a fixed slice of main instead would make the charts' extent depend
   on the repo's commit velocity rather than on how many runs benchmarks.json
   keeps: as soon as benchmarks are sampled on a schedule rather than run per
   push, benchmarked commits are a sparse subset of main and a fixed slice
   holds only a fraction of the available runs.

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
  (fetch-commits-covering data-commits))

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

(defn flatten-benchmarks [raw]
  (for [entry raw
        :let [commit (:Version entry)
              date   (:Date entry)
              tag    (tag-map commit)]
        suite (:Suites entry)
        :let [pkg (clojure.string/replace (:Pkg suite)
                                          "github.com/open-policy-agent/opa" ".")]
        bench (:Benchmarks suite)
        [measure value] {"NsPerOp"    (:NsPerOp bench)
                         "AllocsPerOp" (get-in bench [:Mem :AllocsPerOp])
                         "BytesPerOp"  (get-in bench [:Mem :BytesPerOp])}]
    {:commit  commit
     :date    date
     :pkg     pkg
     :name    (:Name bench)
     :tag     tag
     :measure measure
     :value   (or value 0)}))

(def all-rows (flatten-benchmarks benchmarks-raw))

(def rows
  "Every measurement in benchmarks.json.

   Deliberately unfiltered. This used to drop rows whose commit fell outside the
   fetched commit-metadata slice, which silently discarded measurements the
   charts had every right to draw -- and would have discarded most of them once
   benchmarks stopped being run per push. commit-info supplies a fallback for
   missing metadata instead, so the plotted extent is governed by the run
   history benchmarks.json keeps (INPUT_PRUNE_COUNT) and nothing else."
  all-rows)

(def benchmarks-with-data
  (into #{} (map (juxt :pkg :name)) rows))

(def commit-order
  "commit sha -> position in main's history, oldest first."
  (into {} (map-indexed (fn [i sha] [sha i])) commits-ordered))

(def basis-commit
  "The commit every chart is anchored to: the newest tagged commit on main.

   Derived from main's history rather than from the runs. Picking the newest tag
   that happens to have a run would leave the charts anchored to the previous
   release for as long as the new release's commit went unbenchmarked -- and
   since the nightly experiment resolves its own baseline from git, the two would
   disagree and every night's results would be discarded as mis-anchored.

   Tags cut on a release branch are absent from main's history and so are
   correctly ignored."
  (->> commits-ordered (filter tag-map) last))

(def latest-tag (tag-map basis-commit))

(def basis-measured?
  "Whether basis-commit was itself benchmarked, or the anchor is approximated."
  (contains? (into #{} (map :commit) all-rows) basis-commit))

(defn- basis-row
  "The measurement to divide by: the one taken nearest the anchor in main's
   history, preferring one at or after it.

   A release can be tagged without its commit ever being benchmarked -- a run may
   have been skipped, or the tag may simply be newer than every stored run -- and
   the charts still need something to anchor to. Searching forward alone is not
   enough: when the tag postdates the whole history, that finds nothing, basis
   comes out empty, and since page generation is gated on it the site renders no
   pages at all. Rows whose commit is not in main's fetched history are skipped,
   since they cannot be ordered against the anchor."
  [rows]
  (when-let [anchor (get commit-order basis-commit)]
    (let [ordered (->> rows
                       (keep (fn [r] (when-let [p (commit-order (:commit r))] [p r])))
                       (sort-by first))]
      (or (second (first (filter #(>= (first %) anchor) ordered)))
          (second (last (filter #(< (first %) anchor) ordered)))))))

(def basis
  (into {}
        (keep (fn [[k rows]]
                (when-let [r (basis-row rows)]
                  [k (:value r)])))
        (group-by (juxt :pkg :name :measure) all-rows)))

(when-not basis-measured?
  (println (format "note: %s (%s) has no benchmark run of its own; anchoring to the nearest run instead"
                   latest-tag (subs (or basis-commit "?") 0 (min 7 (count (or basis-commit "?")))))))

(def ratios
  (->> all-rows
       (sort-by :date >)
       (group-by (juxt :pkg :name :measure))
       (keep (fn [[[pkg name measure] vs]]
               ;; Divide by the shared basis rather than by whichever tagged row
               ;; this benchmark happens to have, so the index table and the
               ;; charts report the same number.
               (let [latest-val (:value (first vs))
                     base       (get basis [pkg name measure])]
                 (when (and base (pos? base))
                   {:pkg pkg :name name :measure measure
                    :ratio (/ latest-val base)}))))
       (group-by (juxt :pkg :name))
       (keep (fn [[[pkg name] ms]]
               (let [m (into {} (map (fn [{:keys [measure ratio]}]
                                       [measure ratio]))
                             ms)]
                 (when (seq m)
                   (merge {:pkg pkg :name name} m)))))
       (sort-by #(get % "NsPerOp" 0))))

(defn benchmark-id [pkg name]
  (clojure.string/replace (str pkg "_" name) #"[^a-zA-Z0-9]" "-"))

(def sparklines
  (->> rows
       (filter #(= (:measure %) "NsPerOp"))
       (group-by (juxt :pkg :name))
       (into {}
             (map (fn [[k vs]]
                    [k (mapv :value (sort-by :date vs))])))))

(def benchmarks-with-ids
  (->> ratios
       (filter #(contains? benchmarks-with-data [(:pkg %) (:name %)]))
       (mapv #(assoc % :id (benchmark-id (:pkg %) (:name %))
                        :spark (get sparklines [(:pkg %) (:name %)])))))
