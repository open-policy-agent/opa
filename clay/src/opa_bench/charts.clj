(ns opa-bench.charts
  (:require [scicloj.kindly.v4.kind :as kind]
            [clojure.data.json :as json]
            [clojure.string :as str]
            [opa-bench.data :as data]))

(def measure-labels
  {"NsPerOp"    "time"
   "AllocsPerOp" "allocations"
   "BytesPerOp" "memory"})

(def measure-order
  "Fixed trace order, so a measure keeps its colour across both charts on a page
   and between renders. Iterating a group-by would leave that to chance."
  ["NsPerOp" "AllocsPerOp" "BytesPerOp"])

(def measure-colors
  {"NsPerOp"     "#268bd2"
   "AllocsPerOp" "#d33682"
   "BytesPerOp"  "#859900"})

(def push-window
  "How many of the most recent per-push runs a benchmark's push-panel shows.
   benchmarks.json keeps up to 250; plotting all of them is illegible."
  60)

(def ^:private plotly-cdn
  "https://cdnjs.cloudflare.com/ajax/libs/plotly.js/2.20.0/plotly.min.js")

(defn- format-value [measure v]
  (let [v (double v)]
    (case measure
      "NsPerOp"    (cond
                     (>= v 1e9) (format "%.2fs" (/ v 1e9))
                     (>= v 1e6) (format "%.2fms" (/ v 1e6))
                     (>= v 1e3) (format "%.2fµs" (/ v 1e3))
                     :else       (format "%.2fns" v))
      "BytesPerOp" (cond
                     (>= v 1e9) (format "%.2fGB" (/ v 1e9))
                     (>= v 1e6) (format "%.2fMB" (/ v 1e6))
                     (>= v 1e3) (format "%.2fKB" (/ v 1e3))
                     :else       (format "%.0fB" v))
      "AllocsPerOp" (cond
                      (>= v 1e6) (format "%.2fM allocs" (/ v 1e6))
                      (>= v 1e3) (format "%.2fK allocs" (/ v 1e3))
                      :else       (format "%.0f allocs" v)))))

(defn- commit-url [sha]
  (str "https://github.com/open-policy-agent/opa/commit/" sha))

(defn- commit-detail
  "Commit details for the hover panel and click-through."
  [commit]
  (let [c (data/commit-info commit)]
    {:sha     commit
     :author  (:author c)
     :date    (:date c)
     :message (:message c)
     :url     (commit-url commit)}))

(defn- x-label
  "Categorical x value for a commit: its tag if it has one, else a short sha."
  [commit tag]
  (or tag (data/tag-map commit) (subs commit 0 7)))

(defn- ordered-categories
  "Explicit x-axis order, oldest first.

   Plotly orders a categorical axis by first appearance across traces, which is
   only chronological if every trace covers the same commits. Measures can differ
   in coverage, so the order is stated rather than inferred."
  [labelled]
  (->> labelled
       (sort-by :date)
       (map :x)
       distinct
       vec))

(defn- commit-index [labelled]
  (into {} (map (fn [{:keys [x commit]}] [x (commit-detail commit)])) labelled))

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

(defn- intervals-by-commit
  "Maps the sha of the sample ending an interval to
   {:after sha :commits [{:sha :message :url} ...]}, so the UI can look up
   'what landed between the previous sample and this hovered one'."
  [intervals]
  (into {}
        (map (fn [{:keys [after before commits]}]
               [before {:after after
                        :commits (mapv (fn [sha]
                                         {:sha     sha
                                          :message (:message (data/commit-info sha))
                                          :url     (commit-url sha)})
                                       commits)}]))
        intervals))

(def ^:private chart-js "
(function() {
  var el = document.getElementById('%s');
  var info = document.getElementById('%s');
  var intervalInfo = document.getElementById('%s');
  var commitByX = %s;
  var intervalsByCommit = %s;
  var traces = %s;
  var baseLayout = %s;

  var s = getComputedStyle(document.documentElement);
  var cv = function(v) { return s.getPropertyValue(v).trim(); };
  var layout = Object.assign({}, baseLayout, {
    paper_bgcolor: cv('--chart-bg'),
    plot_bgcolor: cv('--chart-bg'),
    font: Object.assign({}, baseLayout.font, {color: cv('--fg')}),
    yaxis: Object.assign({}, baseLayout.yaxis, {gridcolor: cv('--chart-grid'), color: cv('--fg')}),
    xaxis: Object.assign({}, baseLayout.xaxis, {gridcolor: cv('--chart-grid'), color: cv('--fg')}),
  });
  if (layout.shapes && layout.shapes.length) {
    layout.shapes[0].line = {color: cv('--chart-baseline'), width: 1, dash: 'dash'};
    for (var i = 1; i < layout.shapes.length; i++) {
      layout.shapes[i].line.color = cv('--tag-line');
    }
  }

  Plotly.newPlot(el, traces, layout, {responsive: true});

  function renderInterval(interval) {
    if (!intervalInfo) return;
    if (!interval) {
      intervalInfo.style.display = 'none';
      intervalInfo.innerHTML = '';
      return;
    }
    intervalInfo.style.display = '';
    intervalInfo.innerHTML = '';
    var header = document.createElement('div');
    header.className = 'interval-box-header';
    header.textContent = interval.commits.length + ' commit' + (interval.commits.length === 1 ? '' : 's') +
      ' landed since the previous sample at ' + interval.after.slice(0, 7);
    intervalInfo.appendChild(header);
    var ul = document.createElement('ul');
    interval.commits.forEach(function(c) {
      var li = document.createElement('li');
      var a = document.createElement('a');
      a.href = c.url;
      a.target = '_blank';
      a.textContent = c.sha.slice(0, 7);
      li.appendChild(a);
      li.appendChild(document.createTextNode(' ' + c.message));
      ul.appendChild(li);
    });
    intervalInfo.appendChild(ul);
  }

  el.on('plotly_hover', function(d) {
    var x = d.points[0].x;
    var cd = commitByX[x];
    if (cd) {
      info.textContent = 'Commit: ' + cd.sha + '\\n' +
                         'Author: ' + cd.author + '\\n' +
                         'Date:   ' + cd.date + '\\n\\n' +
                         cd.message;
    }
    renderInterval(cd && intervalsByCommit[cd.sha]);
  });

  el.on('plotly_click', function(d) {
    var x = d.points[0].x;
    var cd = commitByX[x];
    if (cd && cd.url) window.open(cd.url, '_blank');
  });
})();
")

(defn- chart-panel
  "One Plotly chart with its own commit-details panel.

   `id` must be unique within the page: a benchmark page carries two of these,
   and Plotly needs a distinct element per plot."
  [{:keys [id heading caption traces layout commit-by-x intervals]}]
  [:div {:style "margin-bottom:26px"}
   [:h3 {:style "font-size:14px;margin:0 0 2px 0"} heading]
   [:p {:style "font-size:12px;margin:0 0 6px 0;opacity:0.75"} caption]
   [:div {:id id}]
   [:pre {:id (str id "-commit") :class "commit-panel"
          :style "margin-top:10px;padding:10px;min-height:64px;font-size:13px;white-space:pre-wrap"}
    "Hover over a point to see commit details. Click to open on GitHub."]
   (when intervals
     [:div {:id (str id "-interval") :class "interval-box" :style "display:none"}])
   [:script {:type "text/javascript"}
    (format chart-js
            id
            (str id "-commit")
            (str id "-interval")
            (json/write-str commit-by-x)
            (json/write-str (or intervals {}))
            (json/write-str (vec traces))
            (json/write-str layout))]])

(def ^:private base-layout
  {:hoverlabel {:bgcolor "#eaffff" :bordercolor "#888"
                :font {:family "Go Mono, monospace" :size 11 :color "#000"}}
   :hovermode "x unified"
   :font {:family "Go Mono, monospace" :size 11}
   :showlegend true})

(defn- recent-runs
  "The last push-window commits' worth of rows, oldest first. bench-rows
   carries up to one row per measure per commit, so windowing by commit rather
   than by row count keeps every measure's trace covering the same commits."
  [bench-rows]
  (let [commits (into #{} (take-last push-window (distinct (map :commit bench-rows))))]
    (filterv #(contains? commits (:commit %)) bench-rows)))

(defn- push-panel
  "The per-push series: absolute measurements divided by the value at the
   anchoring tag. Every point comes from a different runner, so most of the
   spread between neighbours is between-machine variance rather than change."
  [pkg bench-name bench-rows]
  (let [bench-rows (recent-runs bench-rows)
        by-measure (group-by :measure bench-rows)
        tag-xs     (into #{} (keep :tag) bench-rows)
        labelled   (mapv (fn [r] {:x (x-label (:commit r) (:tag r))
                                  :date (:date r) :commit (:commit r)})
                         bench-rows)
        tick-vals  (filterv some? (mapv :tag bench-rows))
        traces (for [measure measure-order
                     :let [rows  (get by-measure measure)
                           color (measure-colors measure)]
                     :when (and (seq rows) (some #(pos? (:value %)) rows))]
                 (let [basis-val (get data/basis [pkg bench-name measure] 1)
                       basis-val (if (zero? basis-val) 1 basis-val)]
                   {:x    (mapv #(x-label (:commit %) (:tag %)) rows)
                    :y    (mapv #(/ (double (:value %)) basis-val) rows)
                    :text (mapv #(format-value measure (:value %)) rows)
                    :customdata (mapv #(commit-detail (:commit %)) rows)
                    :name (measure-labels measure measure)
                    :type "scatter"
                    :mode "lines+markers"
                    :line {:shape "hvh" :color color}
                    :marker {:color color}
                    :hovertemplate "%{text}<extra>%{fullData.name}</extra>"}))]
    {:id "chart-push"
     :heading "Per-push runs"
     :caption (str "Measurements relative to " data/latest-tag
                   (when-not data/basis-measured?
                     " (which has no run of its own, so the nearest run stands in)")
                   ". Each point is a separate CI run on a different machine, so much of the "
                   "spread between neighbouring points is measurement noise. Showing the most "
                   "recent " push-window " runs.")
     :traces traces
     :commit-by-x (commit-index labelled)
     :intervals (-> data/commits-ordered
                    (interval-commits (into #{} (map :commit) bench-rows))
                    intervals-by-commit)
     :layout (merge base-layout
                    {:yaxis {:type "log" :title (str "Relative to " data/latest-tag)}
                     :xaxis {:title "" :tickangle -45
                             :tickvals tick-vals :ticktext tick-vals
                             :categoryorder "array"
                             :categoryarray (ordered-categories labelled)}
                     :height 480
                     :margin {:b 120}
                     :shapes (into [{:type "line" :xref "paper" :x0 0 :x1 1
                                     :yref "y" :y0 1 :y1 1}]
                                   (for [tag tag-xs]
                                     {:type "line" :x0 tag :x1 tag
                                      :yref "paper" :y0 0 :y1 1
                                      :line {:color "grey" :width 1 :dash "dash"}}))})}))

(defn- benchlab-panel
  "The nightly experiment, on its own axes.

   Kept off the per-push chart deliberately. Its points span a couple of percent
   where the per-push series spans two- to three-fold, and they cluster at the
   recent end of a much longer axis, so a shared plot renders the more precise
   measurement both flat and cramped. Percent difference on a linear scale is
   also easier to read here than ratios hugging 1 on a log scale."
  [series]
  (let [points   (apply concat (vals series))
        labelled (mapv #(select-keys % [:x :date :commit]) points)
        traces (for [measure measure-order
                     :let [ps    (get series measure)
                           color (measure-colors measure)]
                     :when (seq ps)]
                 {:x (mapv :x ps)
                  :y (mapv #(* 100 (- (:ratio %) 1)) ps)
                  ;; benchstat reports an interval for the commit's own samples,
                  ;; not one for the difference, so this shows spread rather than
                  ;; testing significance. In percentage-point space the
                  ;; half-width is ratio * ci-pct.
                  :error_y {:type "data"
                            :array (mapv #(* (:ratio %) (:ci-pct %)) ps)
                            :visible true :thickness 1 :width 3 :color color}
                  :text (mapv (fn [p]
                                (str (format "%+.2f%% vs %s"
                                             (* 100 (- (:ratio p) 1)) data/latest-tag)
                                     (when-not (:significant p) " (within noise)")
                                     (when-let [c (:calibration p)]
                                       (format " | night drift %.2f%%"
                                               (double (:median_abs_drift_pct c))))))
                              ps)
                  :customdata (mapv #(commit-detail (:commit %)) ps)
                  :name (measure-labels measure measure)
                  :type "scatter"
                  :mode "lines+markers"
                  :line {:color color}
                  :marker {:color color :symbol "diamond" :size 7}
                  :hovertemplate "%{text}<extra>%{fullData.name}</extra>"})]
    {:id "chart-benchlab"
     :heading "Nightly benchlab experiment"
     :caption (str "Percent difference from " data/latest-tag
                   ", with both measured side by side on one machine each night. "
                   "Error bars are benchstat's interval for the commit's own samples; "
                   "\"within noise\" in the hover is its significance verdict.")
     :traces traces
     :commit-by-x (commit-index labelled)
     :intervals nil
     :layout (merge base-layout
                    {:yaxis {:title (str "% vs " data/latest-tag) :zeroline true}
                     :xaxis {:title "" :tickangle -45
                             :categoryorder "array"
                             :categoryarray (ordered-categories labelled)}
                     :height 360
                     :margin {:b 110}
                     :shapes [{:type "line" :xref "paper" :x0 0 :x1 1
                               :yref "y" :y0 0 :y1 0}]})}))

(defn benchmark-chart [pkg bench-name]
  (let [bench-rows (->> data/rows
                        (filter #(and (= (:pkg %) pkg)
                                      (= (:name %) bench-name)))
                        (sort-by :date))
        series (into {}
                     (keep (fn [measure]
                             (when-let [ps (seq (data/benchlab-series
                                                  [pkg bench-name measure]))]
                               [measure (mapv #(assoc % :x (x-label (:commit %) nil)) ps)])))
                     measure-order)]
    (kind/hiccup
      (into [:div [:script {:src plotly-cdn}]]
            ;; The nightly chart leads when there is one: it is the trustworthy
            ;; measurement, and most benchmarks are outside the curated set and so
            ;; show only the per-push chart, exactly as before.
            (cond-> []
              (seq series) (conj (chart-panel (benchlab-panel series)))
              :always      (conj (chart-panel (push-panel pkg bench-name bench-rows))))))))

(defn color-for-ratio [ratio]
  (let [t (max -1.0 (min 1.0 (Math/log ratio)))
        r (if (pos? t) 255 (int (* 255 (+ 1 t))))
        g (if (neg? t) 255 (int (* 255 (- 1 t))))]
    (format "rgb(%d,%d,120)" r g)))

(defn ratio-cell [v]
  (if v
    (kind/hiccup
      [:span {:style (str "background:" (color-for-ratio v)
                          ";color:black;padding:2px 6px;display:block;text-align:right")}
       (format "%.2f" (double v))])
    ""))

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

(defn sparkline [values]
  (when (and values (> (count values) 1))
    (let [w 80 h 20
          vs (vec values)
          mn (apply min vs)
          mx (apply max vs)
          rng (- mx mn)
          rng (if (zero? rng) 1.0 rng)
          n (count vs)
          points (str/join " "
                   (for [i (range n)]
                     (str (double (* (/ i (max 1 (dec n))) w))
                          ","
                          (double (- h (* (/ (- (nth vs i) mn) rng) h))))))]
      (kind/hiccup
        [:svg {:width w :height h :style "vertical-align:middle"}
         [:polyline {:points points
                     :fill "none"
                     :stroke "#268bd2"
                     :stroke-width "1.5"}]]))))

(defn index-table [benchmarks]
  (kind/fragment
    [(kind/table
       ;; "NsPerOp (benchlab)" is the same ratio as the NsPerOp column, measured
       ;; against the baseline on one machine instead of across two. Where the
       ;; two disagree, this one is the trustworthy number; it is blank for
       ;; benchmarks outside the curated nightly set.
       {:column-names ["Pkg" "Name" "Trend" "NsPerOp" "NsPerOp (benchlab)"
                       "AllocsPerOp" "BytesPerOp"]
        :row-maps (for [{:keys [pkg name id spark benchlab] :as b} benchmarks]
                    {"Pkg"        pkg
                     "Name"       (kind/hiccup [:a {:href (clay-output-path id)} name])
                     "Trend"      (or (sparkline spark) "")
                     "NsPerOp"    (ratio-cell (get b "NsPerOp"))
                     "NsPerOp (benchlab)" (ratio-cell benchlab)
                     "AllocsPerOp" (ratio-cell (get b "AllocsPerOp"))
                     "BytesPerOp" (ratio-cell (get b "BytesPerOp"))})}
       {:use-datatables true
        :datatables {:pageLength 25
                     :order [[3 "desc"]]}})
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
