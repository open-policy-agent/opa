(ns opa-bench.charts
  (:require [scicloj.kindly.v4.kind :as kind]
            [clojure.data.json :as json]
            [clojure.string :as str]
            [scicloj.plotje.api :as pj]
            [opa-bench.data :as data]))

(def measure-labels
  {"NsPerOp"    "time"
   "AllocsPerOp" "allocations"
   "BytesPerOp" "memory"})

(def measure-order
  "Fixed trace order, so a measure keeps its colour between renders."
  ["NsPerOp" "AllocsPerOp" "BytesPerOp"])

(def measure-colors
  {"NsPerOp"     "#268bd2"
   "AllocsPerOp" "#d33682"
   "BytesPerOp"  "#859900"})

(defn- commit-url [sha]
  (str "https://github.com/open-policy-agent/opa/commit/" sha))

(defn- interval-commits
  "Runs of commits carrying no measurement for a benchmark, sitting strictly
   between two commits that do carry one. Runs before the first or after the
   last data point are ignored, since those just mean the benchmark hadn't been
   added yet / hasn't been sampled yet.

   Read this as \"which commits landed between these two samples\" rather than
   \"which commits are missing data\". The latter only holds when every commit is
   benchmarked; once benchmarks are sampled on a schedule, most commits carry no
   measurement by design and the useful question becomes which changes a jump
   between two samples could be attributed to."
  [commits-ordered bench-commit-set]
  (loop [shas commits-ordered intervals [] current-run [] last-known nil]
    (if (empty? shas)
      intervals
      (let [sha (first shas)]
        (if (contains? bench-commit-set sha)
          (recur (rest shas)
                 (if (and last-known (seq current-run))
                   (conj intervals {:after last-known :before sha :commits current-run})
                   intervals)
                 []
                 sha)
          (recur (rest shas) intervals (conj current-run sha) last-known))))))

(defn- commit-title
  "First line of a commit message: git's convention for the title/subject.
   Interval lists can run to dozens of commits, so only the title is shown."
  [message]
  (first (str/split-lines message)))

(defn- commit-entry [sha]
  {:sha     sha
   :message (commit-title (data/commit-message sha))
   :url     (commit-url sha)})

(defn- intervals-by-commit
  "Maps the sha of the sample ending an interval to
   {:after sha :commits [{:sha :message :url} ...]}, so the UI can look up
   'what landed between the previous sample and this hovered one'."
  [intervals]
  (into {}
        (map (fn [{:keys [after before commits]}]
               [before {:after after :commits (mapv commit-entry commits)}]))
        intervals))

(def ^:private acme-theme
  {:bg "#ffffea" :grid "#e0e0c8" :font-size 11})

(defn- night-instant [epoch-seconds]
  (java.util.Date. (* 1000 (long epoch-seconds))))

(def ^:private interval-js "
(function() {
  var box = document.getElementById('%s');
  var commitsByRow = %s;
  document.querySelector('.plotje-plot').addEventListener('mouseover', function(e) {
    var el = e.target.closest('[data-row-idx]');
    if (!el) return;
    var commits = commitsByRow[+el.getAttribute('data-row-idx')];
    box.innerHTML = '';
    box.style.display = commits ? '' : 'none';
    if (!commits) return;
    var ul = document.createElement('ul');
    commits.forEach(function(c) {
      var li = document.createElement('li');
      var a = document.createElement('a');
      a.href = c.url;
      a.target = '_blank';
      a.textContent = c.sha.slice(0, 7);
      if (c.head) li.className = 'head';
      li.appendChild(a);
      li.appendChild(document.createTextNode(' ' + c.message));
      ul.appendChild(li);
    });
    box.appendChild(ul);
  });
})();
")

(defn- verdict [p]
  (if (:significant p) "significant" "within noise"))

(defn- tooltip-text [measure p commits since]
  (str/join "\n"
            (concat
              [(format "%s: %+.2f%% vs %s (%s)" (measure-labels measure)
                       (* 100 (- (:ratio p) 1)) data/latest-tag (verdict p))
               ;; benchstat's interval for the head's own samples, not for the delta.
               (format "spread of samples: +/-%.1f%%" (* (:ratio p) (:ci-pct p)))]
              (when-let [c (:calibration p)]
                [(format "night drift %.2f%%" (double (:median_abs_drift_pct c)))])
              (when-let [n (some-> commits count)]
                [(format "%d commit%s since %s" n (if (= n 1) "" "s") since)]))))

(defn- head-entry [sha]
  (assoc (commit-entry sha) :head true))

(defn- baseline-row
  "The release itself: zero by definition, since every night measures it
   alongside the head. Nil when the tag commit's date is unknown."
  [measure]
  (when-let [date (data/commit-dates data/latest-baseline-sha)]
    {:date    date
     :measure (measure-labels measure)
     :pct     0.0
     :verdict "baseline"
     :tip     (str data/latest-tag " (baseline)")
     :commits [(head-entry data/latest-baseline-sha)]}))

(defn- shown?
  "Time always plots. Another measure plots only once it has moved: a series
   pinned at zero adds a line that hides the others and says nothing."
  [measure points]
  (and (seq points)
       (or (= measure "NsPerOp")
           (some #(not= 1.0 (double (:ratio %))) points))))

(defn- benchlab-rows
  "One row per measure and night, in the order the interval script indexes them."
  [series intervals-map]
  (vec (for [measure measure-order
             :when (shown? measure (get series measure))
             row (cons (baseline-row measure)
                       (for [p (get series measure)
                             :let [pct     (* 100 (- (:ratio p) 1))
                                   {:keys [after commits]} (get intervals-map (:commit p))
                                   since   (if (= after data/latest-baseline-sha)
                                             data/latest-tag
                                             "previous night")]]
                         {:date    (night-instant (:date p))
                          :measure (measure-labels measure)
                          :pct     pct
                          :verdict (verdict p)
                          :tip     (tooltip-text measure p commits since)
                          ;; The night's own commit closes the list, marked as the sampled one.
                          :commits (conj (vec commits) (head-entry (:commit p)))}))
             :when row]
         row)))

(def ^:private max-date-ticks 20)

(defn- date-ticks
  "Ticks at the nights themselves, labelled by day and thinned to fit."
  [rows]
  (let [nights (->> rows (map :date) distinct sort vec)
        step   (max 1 (long (Math/ceil (/ (count nights) (double max-date-ticks)))))
        shown  (vec (take-nth step nights))
        fmt    (java.time.format.DateTimeFormatter/ofPattern "MMM d" java.util.Locale/ENGLISH)]
    {:breaks      shown
     :tick-labels (mapv #(.format fmt (.atZone (.toInstant ^java.util.Date %) java.time.ZoneOffset/UTC))
                        shown)}))

(def ^:private verdict-shapes
  "Filled where benchstat found a change, hollow where it did not."
  {"significant" :circle "within noise" :circle-open "baseline" :square})

(def ^:private min-y-span
  "Half-width in percent the y-axis never goes below, so noise stays flat."
  5.0)

(defn- y-half-span
  "How far, in percent, the y-axis reaches either side of zero."
  [pcts]
  (max min-y-span (* 1.1 (apply max 0.0 (map #(Math/abs (double %)) pcts)))))

(defn- y-scale
  "Symmetric about zero, at least min-y-span either way, ticked in whole
   percents."
  [rows]
  (let [m      (y-half-span (map :pct rows))
        step   (first (filter #(<= (/ (* 2 m) %) 8) [1 2 5 10 20 50 100 200 500 1000]))
        k      (long (Math/floor (/ m step)))
        breaks (mapv #(* step %) (range (- k) (inc k)))]
    {:domain      [(- m) m]
     :breaks      breaks
     :tick-labels (mapv #(cond (pos? %) (format "+%d%%" %) (neg? %) (format "%d%%" %) :else "0%")
                        breaks)}))

(defn- benchlab-pose [rows]
  (let [measures (into [] (comp (map :measure) (distinct)) rows)
        verdicts (filterv (set (map :verdict rows)) ["significant" "within noise" "baseline"])]
    (-> (into {} (map (fn [k] [k (mapv k rows)])) [:date :measure :pct :verdict :tip])
        (pj/lay-line :date :pct {:color :measure})
        (pj/lay-point :date :pct {:color :measure :shape :verdict :size 8 :tooltip :tip})
        (pj/lay-rule-h {:y-intercept 0})
        (pj/scale :color {:domain measures})
        (pj/scale :shape {:domain verdicts :values (mapv verdict-shapes verdicts) :label "verdict"})
        (pj/scale :x (date-ticks rows))
        (pj/scale :y (y-scale rows))
        (pj/options {:width 1200 :height 400
                     :x-label "" :y-label (str "% vs " data/latest-tag)
                     :theme acme-theme
                     :rule-color "#aaa"
                     :color-values (into {} (map (fn [m] [(measure-labels m) (measure-colors m)]))
                                         measure-order)}))))

(defn benchmark-chart [pkg bench-name]
  (let [series        (into {}
                            (keep (fn [measure]
                                    (when-let [ps (seq (data/benchlab-series [pkg bench-name measure]))]
                                      [measure (vec ps)])))
                            measure-order)
        night-shas    (into #{data/latest-baseline-sha}
                            (comp (mapcat val) (map :commit))
                            series)
        intervals-map (intervals-by-commit
                        (interval-commits data/commits-ordered night-shas))
        rows          (benchlab-rows series intervals-map)
        box-id        "interval-box"]
    ;; The pose is plotted here and its parts spliced into our fragment: a
    ;; fragment nested in a fragment is printed as data rather than rendered.
    (let [plot (pj/plot (benchlab-pose rows))]
      (kind/fragment
        (-> [(kind/hiccup
               [:div
                [:h3 {:style "font-size:14px;margin:0 0 2px 0"} "Nightly benchlab run"]
                [:p {:style "font-size:12px;margin:0 0 6px 0;opacity:0.75"}
                 (str "Percent difference from " data/latest-tag
                      ", with both measured side by side on one machine each night. "
                      "A filled marker is a change benchstat found significant, a hollow one "
                      "is within noise. Hover a point for its sample spread and the commits "
                      "that landed since the previous night's run.")]])]
            (into (if (= :kind/fragment (:kindly/kind (meta plot))) plot [plot]))
            (conj (kind/hiccup
                    [:div
                     [:div {:id box-id :class "interval-box" :style "display:none"}]
                     [:script {:type "text/javascript"}
                      (format interval-js box-id (json/write-str (mapv :commits rows)))]])))))))

(def ^:private tint-amplitude
  "Ratio deviation from 1.0 at which a significant table cell is fully tinted.
   Most benchlab NsPerOp changes are single-digit percent, so a wider scale
   would leave them all pale."
  0.05)

(defn- tint
  "Green for t < 0 (faster) to red for t > 0 (slower), saturating at +/-1."
  [t]
  (let [t (max -1.0 (min 1.0 t))
        r (if (pos? t) 255 (int (* 255 (+ 1 t))))
        g (if (neg? t) 255 (int (* 255 (- 1 t))))]
    (format "rgb(%d,%d,120)" r g)))

(defn ratio-cell
  "Ratios are small, so a significant change is tinted on a fixed scale
   and set apart by weight and outline; one within noise stays plain and grey."
  [v significant?]
  (if v
    (kind/hiccup
      [:span {:style (str "display:block;text-align:right;padding:2px 6px;"
                          (if significant?
                            (str "background:" (tint (/ (- v 1.0) tint-amplitude))
                                 ";color:black;font-weight:bold;box-shadow:inset 0 0 0 1px #000")
                            "color:#999"))}
       (format "%.2f" (double v))])
    ""))

(defn- significant?
  "Whether benchstat found the latest night's change significant."
  [pkg bench-name measure]
  (boolean (:significant (last (get data/benchlab-series [pkg bench-name measure])))))

(defn clay-output-path
  "Matches Clay's actual output naming for ns `benchmarks.<id>`."
  [id]
  (str "benchmarks." (str/replace id #"-" "_") ".html"))

(defn source-search-url
  "GitHub code search URL for the benchmark function definition."
  [pkg bench-name]
  (let [func-name (-> bench-name
                      (str/split #"/")
                      first
                      (str/replace #"-\d+$" ""))
        path      (str/replace pkg #"^\.\/" "")]
    (str "https://github.com/search?q="
         (java.net.URLEncoder/encode
           (str "\"func " func-name "\" repo:open-policy-agent/opa path:" path)
           "UTF-8")
         "&type=code")))

(def ^:private baseline-time
  "Epoch seconds of the release the percentages are against, if known."
  (some-> (data/commit-dates data/latest-baseline-sha) ^java.util.Date (.getTime) (quot 1000)))

(def ^:private spark-time-range
  "[earliest latest] in epoch seconds over every plotted night and the
   baseline, so all sparklines share the chart's time axis."
  (let [ts (cond-> (mapv :date data/benchlab-nights) baseline-time (conj baseline-time))]
    (when (seq ts) [(apply min ts) (apply max ts)])))

(defn sparkline
  "The chart in miniature: the same y-range rule, nights placed by date from
   the baseline, a zero line, and a marker per night that is filled when
   significant and hollow when within noise."
  [points]
  (when (and (seq points) spark-time-range)
    (let [w 96 h 24 pad 3
          [t0 t1] spark-time-range
          pts   (cond->> (mapv (fn [p] {:t (:date p) :pct (* 100 (- (:ratio p) 1)) :p p}) points)
                  baseline-time (cons {:t baseline-time :pct 0.0}))
          m     (y-half-span (map :pct pts))
          x-of  (fn [t] (if (= t0 t1)
                          (/ w 2.0)
                          (+ pad (* (- w (* 2 pad)) (/ (double (- t t0)) (- t1 t0))))))
          y-of  (fn [pct] (- (/ h 2.0) (* (/ pct m) (- (/ h 2.0) pad))))]
      (when (> (count pts) 1)
        (kind/hiccup
          (into [:svg {:width w :height h :style "vertical-align:middle"}
                 [:line {:x1 0 :x2 w :y1 (/ h 2.0) :y2 (/ h 2.0) :stroke "#aaa" :stroke-width "1"}]
                 [:polyline {:points (str/join " " (for [{:keys [t pct]} pts]
                                                     (str (double (x-of t)) "," (double (y-of pct)))))
                             :fill "none"
                             :stroke "#268bd2"
                             :stroke-width "1.5"}]]
                (for [{:keys [t pct p]} pts
                      :when p]
                  [:circle {:cx (double (x-of t)) :cy (double (y-of pct))
                            :r 2 :stroke "#268bd2" :stroke-width "1"
                            :style (str "fill:" (if (:significant p) "#268bd2" "var(--yellow)"))}
                   [:title (format "%+.2f%% (%s)" (double pct) (verdict p))]])))))))

(defn- rank
  "Hidden sort key: rows with a significant change in any measure come first,
   by the largest such change, then the rest by how far time moved."
  [{:keys [pkg name] :as b}]
  (let [moves (for [m measure-order
                    :let [r (get b m)]
                    :when (and r (significant? pkg name m))]
                (Math/abs (- (double r) 1.0)))]
    (if (seq moves)
      (+ 1000.0 (apply max moves))
      (Math/abs (- (double (get b "NsPerOp" 1.0)) 1.0)))))

(def ^:private rank-column 6)

(defn index-table [benchmarks]
  (kind/fragment
    [(kind/hiccup
       [:p (str "Latest night against " data/latest-tag " (1.00 is unchanged). Bold, tinted cells "
                "are changes benchstat found significant; grey ones are within noise. "
                "Rows start with the significant changes, largest first.")])
     (kind/table
       {:column-names ["Pkg" "Name" "Trend" "NsPerOp" "AllocsPerOp" "BytesPerOp" "Rank"]
        :row-maps (for [{:keys [pkg name id] :as b} benchmarks]
                    {"Pkg"        pkg
                     "Name"       (kind/hiccup [:a {:href (clay-output-path id)} name])
                     "Trend"      (or (sparkline (get data/benchlab-series [pkg name "NsPerOp"])) "")
                     "NsPerOp"    (ratio-cell (get b "NsPerOp") (significant? pkg name "NsPerOp"))
                     "AllocsPerOp" (ratio-cell (get b "AllocsPerOp") (significant? pkg name "AllocsPerOp"))
                     "BytesPerOp" (ratio-cell (get b "BytesPerOp") (significant? pkg name "BytesPerOp"))
                     "Rank"       (rank b)})}
       {:use-datatables true
        :datatables {:pageLength 25
                     :columnDefs [{:targets rank-column :visible false}]
                     :order [[rank-column "desc"]]}})
     (kind/hiccup
       [:script "
document.addEventListener('DOMContentLoaded', function() {
  document.querySelectorAll('table tbody tr').forEach(function(row) {
    var link = row.querySelector('a');
    if (link) {
      row.style.cursor = 'pointer';
      row.addEventListener('click', function(e) {
        if (e.target.tagName !== 'A') window.location = link.href;
      });
    }
  });
});
"])]))
