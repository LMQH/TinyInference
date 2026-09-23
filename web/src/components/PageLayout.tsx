import type { ReactNode } from 'react';

interface PageHeaderProps {
  title: string;
  description?: string;
  eyebrow?: string;
  meta?: ReactNode;
}

export function PageHeader({ title, description, eyebrow, meta }: PageHeaderProps) {
  return (
    <header className="page-header">
      <div>
        {eyebrow && <span className="page-kicker">{eyebrow}</span>}
        <h1>{title}</h1>
        {description && <p>{description}</p>}
      </div>
      {meta && <div className="page-header-meta">{meta}</div>}
    </header>
  );
}

interface SectionHeaderProps {
  id?: string;
  title: string;
  description?: ReactNode;
  action?: ReactNode;
}

export function SectionHeader({ id, title, description, action }: SectionHeaderProps) {
  return (
    <div className="section-heading">
      <div>
        <h2 id={id}>{title}</h2>
        {description && <p>{description}</p>}
      </div>
      {action}
    </div>
  );
}

interface ContentCardProps {
  children: ReactNode;
  labelledBy?: string;
  className?: string;
}

export function ContentCard({ children, labelledBy, className = '' }: ContentCardProps) {
  return <section className={`section content-card ${className}`.trim()} aria-labelledby={labelledBy}>{children}</section>;
}

export function EmptyState({ children }: { children: ReactNode }) {
  return <div className="empty-state"><span aria-hidden="true" className="empty-state-mark">—</span><p>{children}</p></div>;
}

interface DataRegionProps {
  children: ReactNode;
  labelledBy?: string;
  label?: string;
  description: string;
}

export function DataRegion({ children, labelledBy, label, description }: DataRegionProps) {
  return (
    <div className="table-scroll" role="region" aria-labelledby={labelledBy} aria-label={label} tabIndex={0}>
      <p className="table-description">{description}</p>
      {children}
    </div>
  );
}
