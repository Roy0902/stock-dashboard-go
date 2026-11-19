import { copy } from '@/locale';
import { BrandMark } from '@/ui/BrandMark';
import './AuthSplash.css';

export function AuthSplash() {
  return (
    <div className="auth-splash" role="status" aria-busy="true">
      <span className="auth-splash__mark" aria-hidden="true">
        <BrandMark />
      </span>
      <span className="auth-splash__name">{copy.brand.name}</span>
    </div>
  );
}
