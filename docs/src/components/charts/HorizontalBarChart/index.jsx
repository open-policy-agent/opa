import React from "react";
import { Bar, BarChart as RechartsBarChart, CartesianGrid, Tooltip, XAxis, YAxis } from "recharts";

export default function HorizontalBarChart({ data, config }) {
  const showXAxis = config.showXAxis !== false;
  const showYAxis = config.showYAxis !== false;
  const showTooltip = config.showTooltip !== false;

  // Calculate height based on number of bars (48px per bar + margins)
  const barHeight = 48;
  const chartHeight = (data.length * barHeight) + 60;

  return (
    <RechartsBarChart
      layout="vertical"
      style={{ width: "100%", height: chartHeight }}
      data={data}
      margin={{
        top: 0,
        right: 0,
        left: 0,
        bottom: 30, // Space for x-axis label
      }}
    >
      <CartesianGrid strokeDasharray="3 3" />
      {showYAxis && (
        <YAxis
          type="category"
          dataKey="name"
          width={250}
          tick={{ fontSize: 12 }}
          {...(config.yAxisLabel && { label: { value: config.yAxisLabel, angle: -90, position: "insideLeft" } })}
        />
      )}
      {showXAxis && (
        <XAxis
          type="number"
          {...(config.xAxisLabel && { label: { value: config.xAxisLabel, position: "insideBottom", offset: -5 } })}
        />
      )}
      {showTooltip && <Tooltip />}
      <Bar dataKey="value" fill="#888888" />
    </RechartsBarChart>
  );
}
