import { useId, useLayoutEffect, useRef, useState } from 'react';

import type { ChartPoint, PriceRange } from '@/api/types';
import { formatChartDate, formatCurrency, formatPrice } from '@/lib/format';
import { copy } from '@/locale';
import './PriceChart.css';

type PriceChartProps = {
  ticker: string;
  range: PriceRange;
  points: ChartPoint[];
};

type PlotPoint = {
  x: number;
  y: number;
  point: ChartPoint;
};

type Insets = {
  top: number;
  right: number;
  bottom: number;
  left: number;
};

const DEFAULT_WIDTH = 720;
const MIN_WIDTH = 240;
const MAX_WIDTH = 1200;
const DESKTOP_HEIGHT = 380;
const COMPACT_HEIGHT = 300;

function useChartWidth() {
  const frameRef = useRef<HTMLDivElement>(null);
  const [width, setWidth] = useState(DEFAULT_WIDTH);

  useLayoutEffect(() => {
    const frame = frameRef.current;
    if (frame === null) {
      return;
    }

    const measure = () => {
      const measured = Math.round(frame.getBoundingClientRect().width);
      if (measured > 0) {
        setWidth(Math.min(MAX_WIDTH, Math.max(MIN_WIDTH, measured)));
      }
    };

    measure();

    if (typeof ResizeObserver === 'undefined') {
      return;
    }

    const observer = new ResizeObserver(measure);
    observer.observe(frame);
    return () => {
      observer.disconnect();
    };
  }, []);

  return { frameRef, width };
}

function createTicks(min: number, max: number, count: number): number[] {
  const span = max - min;
  if (!(span > 0)) {
    return [min];
  }

  const roughStep = span / Math.max(count - 1, 1);
  const magnitude = 10 ** Math.floor(Math.log10(roughStep));
  const residual = roughStep / magnitude;
  const step = (residual <= 1 ? 1 : residual <= 2 ? 2 : residual <= 5 ? 5 : 10) * magnitude;
  const first = Math.ceil(min / step) * step;

  const ticks: number[] = [];
  for (let value = first; value <= max + step * 0.001; value += step) {
    ticks.push(Number(value.toFixed(10)));
  }

  return ticks.length > 0 ? ticks : [min, max];
}

function smoothLine(plotted: PlotPoint[]): string {
  const first = plotted[0];
  if (first === undefined) {
    return '';
  }
  if (plotted.length === 1) {
    return `M ${first.x.toFixed(2)} ${first.y.toFixed(2)}`;
  }

  let path = `M ${first.x.toFixed(2)} ${first.y.toFixed(2)}`;
  for (let index = 1; index < plotted.length; index += 1) {
    const previous = plotted[index - 1];
    const current = plotted[index];
    if (previous === undefined || current === undefined) {
      continue;
    }
    const midX = ((previous.x + current.x) / 2).toFixed(2);
    path += ` C ${midX} ${previous.y.toFixed(2)}, ${midX} ${current.y.toFixed(2)}, ${current.x.toFixed(2)} ${current.y.toFixed(2)}`;
  }
  return path;
}

function clampAnchor(x: number, left: number, right: number): 'start' | 'middle' | 'end' {
  if (x < left + 48) {
    return 'start';
  }
  if (x > right - 48) {
    return 'end';
  }
  return 'middle';
}

export function PriceChart({ ticker, range, points }: PriceChartProps) {
  const { frameRef, width } = useChartWidth();
  const idPrefix = useId().replace(/[^a-zA-Z0-9_-]/g, '');
  const gradientId = `chart-area-${idPrefix}`;
  const titleId = `chart-title-${idPrefix}`;
  const descriptionId = `chart-description-${idPrefix}`;

  const height = width < 560 ? COMPACT_HEIGHT : DESKTOP_HEIGHT;
  const insets: Insets =
    width < 400
      ? { top: 20, right: 12, bottom: 34, left: 48 }
      : width < 560
        ? { top: 24, right: 14, bottom: 38, left: 64 }
        : { top: 30, right: 30, bottom: 44, left: 76 };
  const plotWidth = Math.max(width - insets.left - insets.right, 1);
  const plotHeight = Math.max(height - insets.top - insets.bottom, 1);
  const baseline = insets.top + plotHeight;

  const firstDatum = points[0];
  if (firstDatum === undefined) {
    return null;
  }

  const closes = points.map((point) => point.close);
  const rawMin = Math.min(...closes);
  const rawMax = Math.max(...closes);
  const rawSpan = rawMax - rawMin;
  const padding = rawSpan === 0 ? Math.max(Math.abs(rawMax) * 0.02, 1) : rawSpan * 0.08;
  const min = rawMin - padding;
  const max = rawMax + padding;
  const valueSpan = max - min || 1;

  const xForIndex = (index: number): number => {
    if (points.length === 1) {
      return insets.left + plotWidth / 2;
    }
    return insets.left + (index / (points.length - 1)) * plotWidth;
  };

  const plotted: PlotPoint[] = points.map((point, index) => ({
    x: xForIndex(index),
    y: insets.top + plotHeight - ((point.close - min) / valueSpan) * plotHeight,
    point,
  }));

  const firstPlot = plotted[0];
  const lastPlot = plotted[plotted.length - 1];
  if (firstPlot === undefined || lastPlot === undefined) {
    return null;
  }

  let minPlot = firstPlot;
  let maxPlot = firstPlot;
  for (const entry of plotted) {
    if (entry.point.close < minPlot.point.close) {
      minPlot = entry;
    }
    if (entry.point.close > maxPlot.point.close) {
      maxPlot = entry;
    }
  }

  const linePath = smoothLine(plotted);
  const areaPath =
    plotted.length > 1
      ? `${linePath} L ${lastPlot.x.toFixed(2)} ${baseline.toFixed(2)} L ${firstPlot.x.toFixed(2)} ${baseline.toFixed(2)} Z`
      : '';
  const ticks = createTicks(min, max, width < 400 ? 3 : 5);
  const labelY = height - insets.bottom + 22;
  const minAnchor = clampAnchor(minPlot.x, insets.left, insets.left + plotWidth);
  const maxAnchor = clampAnchor(maxPlot.x, insets.left, insets.left + plotWidth);
  const firstDate = formatChartDate(firstPlot.point.date);
  const lastDate = formatChartDate(lastPlot.point.date);
  const accessibleLabel = copy.chart.accessibleLabel(ticker, range);
  const description = copy.chart.description({
    firstDate,
    lastDate,
    low: formatCurrency(minPlot.point.close),
    lowDate: formatChartDate(minPlot.point.date),
    high: formatCurrency(maxPlot.point.close),
    highDate: formatChartDate(maxPlot.point.date),
  });

  return (
    <div className="price-chart-frame" ref={frameRef}>
      <svg
        className="price-chart"
        viewBox={`0 0 ${width} ${height}`}
        role="img"
        aria-label={accessibleLabel}
        aria-describedby={descriptionId}
      >
        <title id={titleId}>{accessibleLabel}</title>
        <desc id={descriptionId}>{description}</desc>

        <defs>
          <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="var(--color-accent)" stopOpacity="0.3" />
            <stop offset="70%" stopColor="var(--color-accent)" stopOpacity="0.07" />
            <stop offset="100%" stopColor="var(--color-accent)" stopOpacity="0" />
          </linearGradient>
        </defs>

        <g className="price-chart__grid">
          {ticks.map((tick) => {
            const y = insets.top + plotHeight - ((tick - min) / valueSpan) * plotHeight;
            return (
              <g key={tick}>
                <line
                  x1={insets.left}
                  x2={insets.left + plotWidth}
                  y1={y}
                  y2={y}
                  vectorEffect="non-scaling-stroke"
                />
                <text
                  className="price-chart__axis-label"
                  x={insets.left - 12}
                  y={y}
                  textAnchor="end"
                  dominantBaseline="middle"
                >
                  {formatPrice(tick)}
                </text>
              </g>
            );
          })}
        </g>

        {areaPath === '' ? null : (
          <path className="price-chart__area" d={areaPath} fill={`url(#${gradientId})`} />
        )}
        <path
          className="price-chart__line"
          d={linePath}
          fill="none"
          vectorEffect="non-scaling-stroke"
        />

        <g className="price-chart__extremes">
          <circle className="price-chart__extreme-dot" cx={maxPlot.x} cy={maxPlot.y} r="3.5" />
          <text
            className="price-chart__extreme-label"
            x={maxPlot.x}
            y={maxPlot.y - 11}
            textAnchor={maxAnchor}
          >
            {formatPrice(maxPlot.point.close)}
          </text>
          <circle className="price-chart__extreme-dot" cx={minPlot.x} cy={minPlot.y} r="3.5" />
          <text
            className="price-chart__extreme-label"
            x={minPlot.x}
            y={minPlot.y + 18}
            textAnchor={minAnchor}
          >
            {formatPrice(minPlot.point.close)}
          </text>
        </g>

        <g className="price-chart__dates">
          <text className="price-chart__axis-label" x={firstPlot.x} y={labelY} textAnchor="start">
            {firstDate}
          </text>
          <text className="price-chart__axis-label" x={lastPlot.x} y={labelY} textAnchor="end">
            {lastDate}
          </text>
        </g>
      </svg>
    </div>
  );
}
