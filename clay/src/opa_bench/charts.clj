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
  "Fixed trace order, so a measure keeps its colour between renders."
  ["NsPerOp" "AllocsPerOp" "BytesPerOp"])

(def measure-colors
  {"NsPerOp"     "#268bd2"
   "AllocsPerOp" "#d33682"
   "BytesPerOp"  "#859900"})

(def ^:private plotly-cdn
  "https://cdnjs.cloudflare.com/ajax/libs/plotly.js/2.20.0/plotly.min.js")

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

(defn- commit-index [labelled]
  (into {} (map (fn [{:keys [x commit]}] [x (commit-detail commit)])) labelled))

(def ^:private max-gap-ticks
  "How many intermediate commits get an axis tick for a single gap between two
   samples. One tick per commit in a busy gap would blow up the axis width far
   more than it clarifies; a handful spread evenly through the gap is enough
   to show a gap exists without diluting the density of the real points."
  3)

(defn- thin
  "At most n elements from coll, evenly spaced, in order, always including
   the first and last."
  [n coll]
  (let [v (vec coll) c (count v)]
    (cond
      (<= c n) v
      (<= n 1) (subvec v 0 1)
      :else (->> (range n)
                 (map #(Math/round (double (* % (/ (dec c) (dec n))))))
                 distinct
                 (map v)
                 vec))))

(defn- benchlab-axis-commits
  "Every commit backing an axis tick, in order: each sampled commit, plus up
   to max-gap-ticks evenly spaced commits from the gap right after it --
   enough to show where the gaps are without one tick per intervening commit
   spreading the real points thin across the axis.

   Kept as shas rather than labels: the tick-label click handler needs each
   one's GitHub URL, which is derived from the sha, not the display text.

   `intervals` is interval-commits' raw output (not the by-commit map built
   from it for the hover panel)."
  [labelled intervals]
  (let [gap-after (into {} (map (fn [{:keys [after commits]}] [after commits])) intervals)
        samples   (->> labelled distinct (sort-by :date) (map :commit))]
    (vec (mapcat (fn [sha] (cons sha (thin max-gap-ticks (get gap-after sha)))) samples))))

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
   Interval lists can run to dozens of commits, so only the title is shown --
   the full message belongs to the single-commit hover panel, not a list."
  [message]
  (first (str/split-lines message)))

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
                                          :message (commit-title (:message (data/commit-info sha)))
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
  var tickUrlByLabel = %s;

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

  function linkTickLabels() {
    el.querySelectorAll('.xaxislayer-above .xtick text').forEach(function(t) {
      var url = tickUrlByLabel[t.textContent];
      if (!url) return;
      t.style.cursor = 'pointer';
      t.onclick = function(ev) { ev.stopPropagation(); window.open(url, '_blank'); };
    });
  }
  el.on('plotly_afterplot', linkTickLabels);

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
    if (cd && info) {
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
  "One Plotly chart, optionally paired with a single-commit details panel."
  [{:keys [id heading caption traces layout commit-by-x intervals show-commit-info tick-urls]
    :or   {show-commit-info true}}]
  [:div {:style "margin-bottom:26px"}
   [:h3 {:style "font-size:14px;margin:0 0 2px 0"} heading]
   [:p {:style "font-size:12px;margin:0 0 6px 0;opacity:0.75"} caption]
   [:div {:id id}]
   (when show-commit-info
     [:pre {:id (str id "-commit") :class "commit-panel"
            :style "margin-top:10px;padding:10px;min-height:64px;font-size:13px;white-space:pre-wrap"}
      "Hover over a point to see commit details. Click to open on GitHub."])
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
            (json/write-str layout)
            (json/write-str (or tick-urls {})))]])

(def ^:private base-layout
  {:hoverlabel {:bgcolor "#eaffff" :bordercolor "#888"
                :font {:family "Go Mono, monospace" :size 11 :color "#000"}}
   :hovermode "x unified"
   :font {:family "Go Mono, monospace" :size 11}
   :showlegend true})

(defn- benchlab-panel
  "The nightly experiment, as percent difference from the baseline tag."
  [series]
  (let [points         (apply concat (vals series))
        labelled       (mapv #(select-keys % [:x :date :commit]) points)
        night-shas     (into #{} (map :commit) points)
        raw-intervals  (interval-commits data/commits-ordered night-shas)
        intervals-map  (intervals-by-commit raw-intervals)
        axis-commits   (benchlab-axis-commits labelled raw-intervals)
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
                                               (double (:median_abs_drift_pct c))))
                                     (when-let [n (some-> (get intervals-map (:commit p))
                                                          :commits count)]
                                       (format " | %d commit%s since previous night"
                                               n (if (= n 1) "" "s")))))
                              ps)
                  :customdata (mapv #(commit-detail (:commit %)) ps)
                  :name (measure-labels measure measure)
                  :type "scatter"
                  :mode "lines+markers"
                  :line {:color color}
                  :marker {:color color :symbol "diamond" :size 7}
                  :hovertemplate "%{text}<extra>%{fullData.name}</extra>"})]
    {:id "chart-benchlab"
     :heading "Nightly benchlab run"
     :caption (str "Percent difference from " data/latest-tag
                   ", with both measured side by side on one machine each night. "
                   "Error bars are benchstat's interval for the commit's own samples; "
                   "\"within noise\" in the hover is its significance verdict. Hover a "
                   "point to see which commits landed since the previous night's run.")
     :traces traces
     :commit-by-x (commit-index labelled)
     :intervals intervals-map
     :show-commit-info false
     :tick-urls (into {} (map (fn [sha] [(x-label sha nil) (commit-url sha)])) axis-commits)
     :layout (merge base-layout
                    {:yaxis {:title (str "% vs " data/latest-tag) :zeroline true}
                     :xaxis {:title "" :tickangle -45
                             :categoryorder "array"
                             :categoryarray (mapv #(x-label % nil) axis-commits)}
                     :height 360
                     :margin {:b 110}
                     :shapes [{:type "line" :xref "paper" :x0 0 :x1 1
                               :yref "y" :y0 0 :y1 0}]})}))

(defn benchmark-chart [pkg bench-name]
  (let [series (into {}
                     (keep (fn [measure]
                             (when-let [ps (seq (data/benchlab-series
                                                  [pkg bench-name measure]))]
                               [measure (mapv #(assoc % :x (x-label (:commit %) nil)) ps)])))
                     measure-order)]
    (kind/hiccup
      [:div [:script {:src plotly-cdn}]
       (chart-panel (benchlab-panel series))])))

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

(def ^:private sparkline-amplitude
  "Ratio deviation from 1.0 that maps to the top/bottom of a sparkline, clamped
   beyond that. Fixed rather than fit to each row's own min/max so a benchmark
   with a 1% noise wobble doesn't draw the same full-height swing as one that
   actually moved 80% -- most benchlab NsPerOp ranges are single-digit percent,
   so a wobble should look flat, matching how little it moves on the real
   per-benchmark chart."
  0.05)

(defn sparkline [values]
  (when (and values (> (count values) 1))
    (let [w 80 h 20
          vs (vec values)
          n (count vs)
          y-of (fn [v]
                 (let [d (max -1.0 (min 1.0 (/ (- v 1.0) sparkline-amplitude)))]
                   (- (/ h 2.0) (* d (/ h 2.0)))))
          points (str/join " "
                   (for [i (range n)]
                     (str (double (* (/ i (max 1 (dec n))) w))
                          ","
                          (double (y-of (nth vs i))))))]
      (kind/hiccup
        [:svg {:width w :height h :style "vertical-align:middle"}
         [:polyline {:points points
                     :fill "none"
                     :stroke "#268bd2"
                     :stroke-width "1.5"}]]))))

(defn index-table [benchmarks]
  (kind/fragment
    [(kind/table
       {:column-names ["Pkg" "Name" "Trend" "NsPerOp" "AllocsPerOp" "BytesPerOp"]
        :row-maps (for [{:keys [pkg name id spark] :as b} benchmarks]
                    {"Pkg"        pkg
                     "Name"       (kind/hiccup [:a {:href (clay-output-path id)} name])
                     "Trend"      (or (sparkline spark) "")
                     "NsPerOp"    (ratio-cell (get b "NsPerOp"))
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
