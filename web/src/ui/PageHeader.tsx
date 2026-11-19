import type { ReactNode } from 'react';

import './PageHeader.css';

type PageHeaderProps = {
  title: string;
  children?: ReactNode;
  className?: string;
};

export function PageHeader({ title, children, className }: PageHeaderProps) {
  const classes = ['page-header', className].filter(Boolean).join(' ');

  return (
    <div className={classes}>
      <h1 className="page-header__title">{title}</h1>
      {children ? <div className="page-header__aside">{children}</div> : null}
    </div>
  );
}
