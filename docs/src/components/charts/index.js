import BarChart from "./BarChart";
import HorizontalBarChart from "./HorizontalBarChart";
import StackedBarChart from "./StackedBarChart";
import TextList from "./TextList";

export const chartRegistry = {
  "bar": BarChart,
  "stacked-bar": StackedBarChart,
  "text-list": TextList,
  "horizontal-bar": HorizontalBarChart,
};

export function getChartComponent(chartType) {
  return chartRegistry[chartType] || null;
}
