import React from 'react';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, Legend } from 'recharts';

export default function StackedBarChart({ data, config, categories }) {
  const colors = ['#0077BB', '#EE7733', '#009988', '#CCBB44', '#CC3311', '#AA3377'];

  const showXAxis = config.showXAxis !== false;
  const showYAxis = config.showYAxis !== false;
  const showTooltip = config.showTooltip !== false;
  const showLegend = config.showLegend !== false;

  return (
    <BarChart
      style={{ width: '100%', height: '25rem' }}
      data={data}
      margin={{
        top: 20,
        right: 0,
        left: 0,
        bottom: 20,
      }}
    >
      <CartesianGrid strokeDasharray="3 3" />
      {showXAxis && (
        <XAxis
          dataKey="name"
          interval={0}
          angle={-45}
          textAnchor="end"
          height={120}
          tick={{ dy: 40 }}
          {...(config.xAxisLabel && { label: { value: config.xAxisLabel, position: 'insideBottom', offset: -5 } })}
        />
      )}
      {showYAxis && (
        <YAxis label={{ value: config.yAxisLabel, angle: -90, position: 'insideLeft' }} />
      )}
      {showTooltip && <Tooltip />}
      {showLegend && <Legend />}
      {categories.map((category, index) => (
        <Bar
          key={category}
          dataKey={category}
          stackId="a"
          fill={colors[index % colors.length]}
        />
      ))}
    </BarChart>
  );
}
