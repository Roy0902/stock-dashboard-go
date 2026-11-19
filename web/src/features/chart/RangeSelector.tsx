import { PRICE_RANGES } from '@/api/types';
import type { PriceRange } from '@/api/types';
import { copy } from '@/locale';
import './RangeSelector.css';

type RangeSelectorProps = {
  value: PriceRange;
  onChange: (range: PriceRange) => void;
};

export function RangeSelector({ value, onChange }: RangeSelectorProps) {
  return (
    <div className="range-selector" role="group" aria-label={copy.chart.rangeLabel}>
      {PRICE_RANGES.map((range) => (
        <button
          key={range}
          className="range-selector__button"
          type="button"
          aria-pressed={range === value}
          onClick={() => {
            onChange(range);
          }}
        >
          {range}
        </button>
      ))}
    </div>
  );
}
