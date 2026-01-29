import BarChart from './BarChart';
import StackedBarChart from './StackedBarChart';
import TextList from './TextList';
import HorizontalBarChart from './HorizontalBarChart';

export const chartRegistry = {
  'bar': BarChart,
  'stacked-bar': StackedBarChart,
  'text-list': TextList,
  'horizontal-bar': HorizontalBarChart,
};

export function getChartComponent(chartType) {
  return chartRegistry[chartType] || null;
}
