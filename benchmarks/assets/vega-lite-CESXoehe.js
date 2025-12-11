import * as vega from "https://cdn.jsdelivr.net/npm/vega/+esm";
import * as vegaLite from "https://cdn.jsdelivr.net/npm/vega-lite/+esm";
import * as vegaLiteApi from "https://cdn.jsdelivr.net/npm/vega-lite-api/+esm";
const vl = vegaLiteApi.register(vega, vegaLite);
export {
  vl
};
