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

(def commits-raw
  (github-fetch "commits?per_page=100"))

(def commits
  (into {}
        (map (fn [{:keys [sha commit author]}]
               [sha {:message (:message commit)
                     :author  (:login author)
                     :date    (get-in commit [:author :date])}]))
        commits-raw))

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
  (filter #(contains? commits (:commit %)) all-rows))

(def benchmarks-in-window
  (into #{} (map (juxt :pkg :name)) rows))

(def latest-tag
  (->> all-rows
       (filter :tag)
       (sort-by :date >)
       first
       :tag))

(def basis
  (into {}
        (comp (filter #(= (:tag %) latest-tag))
              (map (fn [{:keys [pkg name measure value]}]
                     [[pkg name measure] value])))
        all-rows))

(def ratios
  (let [grouped (->> all-rows
                     (sort-by :date >)
                     (group-by (juxt :pkg :name :measure)))]
    (->> grouped
         (keep (fn [[[pkg name measure] vs]]
                 (let [latest-val (:value (first vs))
                       tag-val    (:value (first (filter :tag vs)))]
                   (when (and tag-val (pos? tag-val))
                     {:pkg pkg :name name :measure measure
                      :ratio (/ latest-val tag-val)}))))
         (group-by (juxt :pkg :name))
         (keep (fn [[[pkg name] ms]]
                 (let [m (into {} (map (fn [{:keys [measure ratio]}]
                                         [measure ratio]))
                               ms)]
                   (when (seq m)
                     (merge {:pkg pkg :name name} m)))))
         (sort-by #(get % "NsPerOp" 0)))))

(defn benchmark-id [pkg name]
  (clojure.string/replace (str pkg "_" name) #"[^a-zA-Z0-9]" "-"))

(def benchmarks-with-ids
  (->> ratios
       (filter #(contains? benchmarks-in-window [(:pkg %) (:name %)]))
       (mapv #(assoc % :id (benchmark-id (:pkg %) (:name %))))))
