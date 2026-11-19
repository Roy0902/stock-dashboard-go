/** Shared brand glyph. Callers own the surrounding badge and its size. */
export function BrandMark() {
  return (
    <svg viewBox="0 0 32 32" aria-hidden="true" focusable="false">
      <path
        d="M5 21.5 11.5 14l5 3.5L27 7"
        fill="none"
        stroke="currentColor"
        strokeWidth="2.25"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <circle cx="27" cy="7" r="2" fill="currentColor" />
    </svg>
  );
}
