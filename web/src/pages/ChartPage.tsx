import { Link, useParams } from 'react-router-dom';

import { ChartView } from '@/features/chart/ChartView';
import { copy } from '@/locale';
import { ROUTE_PATHS } from '@/routes';
import './ChartPage.css';

export function ChartPage() {
  const params = useParams<{ ticker: string }>();
  const ticker = (params.ticker ?? '').toUpperCase();

  return (
    <div className="chart-page">
      <Link className="chart-page__back" to={ROUTE_PATHS.dashboard}>
        <svg viewBox="0 0 16 16" aria-hidden="true" focusable="false">
          <path
            d="M13 8H4M8.5 3.5 4 8l4.5 4.5"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
        {copy.chart.back}
      </Link>
      <ChartView key={ticker} ticker={ticker} />
    </div>
  );
}
