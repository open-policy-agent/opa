import React from 'react';
import { BarChart as RechartsBarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip } from 'recharts';

export default function BarChart({ data, config }) {
  const showXAxis = config.showXAxis !== false;
  const showYAxis = config.showYAxis !== false;
  const showTooltip = config.showTooltip !== false;

  return (
    <RechartsBarChart
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
      <Bar dataKey="value" fill="#888888" />
    </RechartsBarChart>
  );
}
