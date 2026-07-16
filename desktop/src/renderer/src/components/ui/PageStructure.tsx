import { Card, Drawer, Empty, Result, Skeleton, Space, Tabs, Typography, type CardProps, type DrawerProps, type TabsProps } from "antd";
import type { ReactNode } from "react";

export function PageShell({ children, className = "" }: { children: ReactNode; className?: string }) {
  return <main className={`page-shell page-flow ${className}`.trim()}>{children}</main>;
}

export function PageHeader({ title, description, actions, className = "" }: { title: string; description?: ReactNode; actions?: ReactNode; className?: string }) {
  return (
    <header className={`page-header ${className}`.trim()}>
      <div className="page-header-copy">
        <Typography.Title level={2}>{title}</Typography.Title>
        {description && <Typography.Paragraph type="secondary">{description}</Typography.Paragraph>}
      </div>
      {actions && <div className="page-header-actions">{actions}</div>}
    </header>
  );
}

export function PageTabs(props: TabsProps) {
  return <Tabs {...props} className={`page-tabs ${props.className ?? ""}`.trim()} />;
}

export function ContentCard({ children, className = "", ...props }: CardProps) {
  return <Card {...props} bordered={false} className={`content-card ${className}`.trim()}>{children}</Card>;
}

export function SummaryStat({ label, value, description, tone = "neutral" }: { label: string; value: string; description?: string; tone?: "primary" | "success" | "neutral" }) {
  return <div className={`summary-stat summary-stat-${tone}`}><span>{label}</span><strong>{value}</strong>{description && <small>{description}</small>}</div>;
}

export function FilterToolbar({ children, resultCount, actions }: { children: ReactNode; resultCount?: ReactNode; actions?: ReactNode }) {
  return (
    <div className="filter-toolbar">
      <div className="filter-toolbar-fields">{children}</div>
      {(actions || resultCount) && <div className="filter-toolbar-meta">{resultCount && <Typography.Text type="secondary">{resultCount}</Typography.Text>}{actions}</div>}
    </div>
  );
}

export function DetailDrawer({ children, className = "", ...props }: DrawerProps & { children?: ReactNode }) {
  return <Drawer {...props} className={`detail-drawer ${className}`.trim()}>{children}</Drawer>;
}

export function EmptyState({ description, search = false }: { description: string; search?: boolean }) {
  return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={search ? `没有匹配“${description}”的结果` : description} />;
}

export function ErrorState({ title = "加载失败", description, onRetry }: { title?: string; description?: string; onRetry?: () => void }) {
  return <Result status="error" title={title} subTitle={description} extra={onRetry ? <button className="ui-link-button" type="button" onClick={onRetry}>重试</button> : undefined} />;
}

export function LoadingSkeleton({ rows = 4 }: { rows?: number }) {
  return <div className="loading-skeleton"><Skeleton active paragraph={{ rows }} /></div>;
}

export function Stack({ children, className = "" }: { children: ReactNode; className?: string }) {
  return <Space direction="vertical" size={16} className={`ui-stack ${className}`.trim()}>{children}</Space>;
}
