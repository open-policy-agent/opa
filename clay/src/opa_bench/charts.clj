(ns opa-bench.charts
  (:require [scicloj.kindly.v4.kind :as kind]
            [clojure.data.json :as json]
            [clojure.string :as str]
            [opa-bench.data :as data]))

(defn benchmark-chart [pkg bench-name]
  (let [bench-rows (->> data/rows
                        (filter #(and (= (:pkg %) pkg)
                                      (= (:name %) bench-name)))
                        (sort-by :date))
        by-measure (group-by :measure bench-rows)
        tag-xs     (into #{} (keep :tag) bench-rows)
        traces     (for [[measure rows] by-measure]
                     (let [basis-val (get data/basis [pkg bench-name measure] 1)
                           basis-val (if (zero? basis-val) 1 basis-val)]
                       {:x    (mapv #(or (:tag %) (subs (:commit %) 0 7)) rows)
                        :y    (mapv #(/ (double (:value %)) basis-val) rows)
                        :text (mapv #(str (long (:value %))) rows)
                        :customdata
                        (mapv (fn [r]
                                (let [c (data/commits (:commit r))]
                                  {:sha     (:commit r)
                                   :author  (:author c)
                                   :date    (:date c)
                                   :message (or (:message c) "")
                                   :url     (str "https://github.com/open-policy-agent/opa/commit/"
                                                 (:commit r))}))
                              rows)
                        :name measure
                        :type "scatter"
                        :mode "lines+markers"
                        :line {:shape "hvh"}
                        :hovertemplate "%{text}<extra>%{fullData.name}</extra>"}))
        tag-shapes (for [tag tag-xs]
                     {:type "line"
                      :x0 tag :x1 tag
                      :yref "paper" :y0 0 :y1 1
                      :line {:color "grey" :width 1 :dash "dash"}})
        commit-by-x (into {}
                          (map (fn [r]
                                 (let [x (or (:tag r) (subs (:commit r) 0 7))
                                       c (data/commits (:commit r))]
                                   [x {:sha     (:commit r)
                                       :author  (:author c)
                                       :date    (:date c)
                                       :message (or (:message c) "")
                                       :url     (str "https://github.com/open-policy-agent/opa/commit/"
                                                     (:commit r))}])))
                          bench-rows)
        layout     {:yaxis    {:type "log" :title (str "Relative to " data/latest-tag)}
                    :xaxis    {:title "" :tickangle -45}
                    :hovermode "x unified"
                    :height   500
                    :margin   {:b 120}
                    :colorway ["#268bd2" "#d33682" "#859900"]
                    :font {:family "Go Mono, monospace" :size 11}
                    :shapes   (into [{:type "line"
                                      :xref "paper" :x0 0 :x1 1
                                      :yref "y" :y0 1 :y1 1}]
                                    tag-shapes)}]
    (kind/hiccup
      [:div
       [:script {:src "https://cdnjs.cloudflare.com/ajax/libs/plotly.js/2.20.0/plotly.min.js"}]
       [:div {:id "chart"}]
       [:pre {:id "commit-info"
              :style "margin-top:12px;padding:10px;min-height:80px;font-size:13px;white-space:pre-wrap"}
        "Hover over a point to see commit details. Click to open on GitHub."]
       [:script {:type "text/javascript"}
        (format "
(function() {
  var el = document.getElementById('chart');
  var info = document.getElementById('commit-info');
  var commitByX = %s;
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
  layout.shapes[0].line = {color: cv('--chart-baseline'), width: 1, dash: 'dash'};
  for (var i = 1; i < layout.shapes.length; i++) {
    layout.shapes[i].line.color = cv('--tag-line');
  }

  Plotly.newPlot(el, traces, layout, {responsive: true});

  el.on('plotly_hover', function(d) {
    var x = d.points[0].x;
    var cd = commitByX[x];
    if (cd) {
      info.textContent = 'Commit: ' + cd.sha + '\\n' +
                         'Author: ' + cd.author + '\\n' +
                         'Date:   ' + cd.date + '\\n\\n' +
                         cd.message;
    }
  });

  el.on('plotly_click', function(d) {
    var x = d.points[0].x;
    var cd = commitByX[x];
    if (cd && cd.url) window.open(cd.url, '_blank');
  });
})();
"
                (json/write-str commit-by-x)
                (json/write-str (vec traces))
                (json/write-str layout))]])))

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

(defn index-table [benchmarks]
  (kind/table
    {:column-names ["Pkg" "Name" "NsPerOp" "AllocsPerOp" "BytesPerOp"]
     :row-maps (for [{:keys [pkg name id] :as b} benchmarks]
                 {"Pkg"        pkg
                  "Name"       (kind/hiccup [:a {:href (clay-output-path id)} name])
                  "NsPerOp"    (ratio-cell (get b "NsPerOp"))
                  "AllocsPerOp" (ratio-cell (get b "AllocsPerOp"))
                  "BytesPerOp" (ratio-cell (get b "BytesPerOp"))})}
    {:use-datatables true
     :datatables {:pageLength 25
                  :order [[2 "desc"]]}}))
