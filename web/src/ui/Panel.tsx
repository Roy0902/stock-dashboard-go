import type { HTMLAttributes } from 'react';

import './Panel.css';

type PanelProps = HTMLAttributes<HTMLDivElement>;

export function Panel({ className, ...rest }: PanelProps) {
  const classes = ['panel', className].filter(Boolean).join(' ');

  return <div className={classes} {...rest} />;
}
