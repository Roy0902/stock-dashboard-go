import { useState } from 'react';

import type { PriceRange } from '@/api/types';
import { ChartSkeleton } from '@/features/chart/ChartSkeleton';
import { PriceChart } from '@/features/chart/PriceChart';
import { RangeSelector } from '@/features/chart/RangeSelector';
import { useChart } from '@/features/chart/useChart';
import { copy } from '@/locale';
import { Alert } from '@/ui/Alert';
import { PageHeader } from '@/ui/PageHeader';
import { Panel } from '@/ui/Panel';
import './ChartView.css';

type ChartViewProps = {
  ticker: string;
};

export function ChartView({ ticker }: ChartViewProps) {
  const [range, setRange] = useState<PriceRange>('1M');
  const chart = useChart(ticker, range);
  const series = chart.data;

  return (
    <div className="chart-view">
      <PageHeader title={ticker}>
        <RangeSelector value={range} onChange={setRange} />
      </PageHeader>

      <Panel className="chart-panel" aria-busy={chart.isFetching}>
        {chart.isPending ? (
          <ChartSkeleton />
        ) : chart.isError ? (
          <Alert>{chart.error?.message ?? copy.chart.unavailable}</Alert>
        ) : series === undefined ? (
          <Alert>{copy.chart.unavailable}</Alert>
        ) : series.points.length === 0 ? (
          <p className="chart-panel__empty" role="status">
            {copy.chart.noData}
          </p>
        ) : (
          <>
            <PriceChart ticker={ticker} range={range} points={series.points} />
            <p className="chart-panel__source">{copy.chart.source(series.source)}</p>
          </>
        )}
      </Panel>
    </div>
  );
}
