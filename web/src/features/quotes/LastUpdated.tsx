import { formatClock } from '@/lib/format';
import { copy } from '@/locale';
import './LastUpdated.css';

type LastUpdatedProps = {
  at: number;
};

export function LastUpdated({ at }: LastUpdatedProps) {
  if (at <= 0) {
    return null;
  }

  return (
    <p className="last-updated" role="status">
      <span className="last-updated__dot" aria-hidden="true" />
      {copy.quotes.lastUpdated(formatClock(new Date(at)))}
    </p>
  );
}
