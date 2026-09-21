import { useState, type ReactNode } from 'react';
import {
  Header,
  HeaderGlobalBar,
  HeaderMenuButton,
  HeaderName,
  InlineNotification,
  SideNav,
  SideNavItems,
  SideNavLink,
  SkeletonText,
} from '@carbon/react';
import { Alarm, ChartLine, DataBase, Dashboard, RequestQuote } from '@carbon/icons-react';
import { useLocation } from 'react-router-dom';
import { useSnapshot } from '../api/client/hooks';
import { ServiceStatusTag } from '../components/StatusTag';
import { formatTime } from '../components/format';
import { useRealtime } from '../realtime/RealtimeProvider';

const NAVIGATION = [
  { href: '/', label: '运行总览', icon: Dashboard },
  { href: '/requests', label: '请求与队列', icon: RequestQuote },
  { href: '/metrics', label: '指标与趋势', icon: ChartLine },
  { href: '/alerts', label: '告警与日志', icon: Alarm },
  { href: '/data', label: '数据与备份', icon: DataBase },
] as const;

export interface AppShellProps { children: ReactNode }

export function AppShell({ children }: AppShellProps) {
  const [navigationOpen, setNavigationOpen] = useState(false);
  const location = useLocation();
  const snapshot = useSnapshot();
  const realtime = useRealtime();
  const connectionLabel = realtime.state === 'connected' ? '实时更新已连接' : realtime.state === 'stale' ? '更新已断开' : realtime.state === 'compatibility_error' ? '更新契约不兼容' : '正在连接更新';
  const isStale = realtime.state === 'stale' || realtime.state === 'compatibility_error' || (snapshot.isError && snapshot.data !== undefined);

  return (
    <>
      <a className="skip-link" href="#main-content">跳到主要内容</a>
      <Header aria-label="Mini-Inference 运维控制台">
        <HeaderMenuButton aria-label={navigationOpen ? '关闭导航' : '打开导航'} isActive={navigationOpen} onClick={() => setNavigationOpen((open) => !open)} />
        <HeaderName href="/" prefix="Mini-Inference">运维控制台</HeaderName>
        <HeaderGlobalBar>
          <div className="header-private-boundary">无需登录，仅限私有网络</div>
          <div className="header-service-state">
            {snapshot.data ? <ServiceStatusTag state={snapshot.data.service.state} /> : <SkeletonText width="6rem" />}
          </div>
        </HeaderGlobalBar>
        <SideNav aria-label="主要导航" expanded={navigationOpen} isFixedNav onOverlayClick={() => setNavigationOpen(false)}>
          <SideNavItems>
            {NAVIGATION.map((item) => (
              <SideNavLink
                key={item.href}
                href={item.href}
                renderIcon={item.icon}
                aria-current={location.pathname === item.href ? 'page' : undefined}
                isActive={location.pathname === item.href}
                onClick={() => setNavigationOpen(false)}
              >
                {item.label}
              </SideNavLink>
            ))}
          </SideNavItems>
        </SideNav>
      </Header>
      {isStale && (
        <div className="stale-banner">
          <InlineNotification
            hideCloseButton
            kind={realtime.state === 'compatibility_error' ? 'error' : 'warning'}
            title={realtime.state === 'compatibility_error' ? '更新契约不兼容' : '数据更新已中断'}
            subtitle={realtime.state === 'compatibility_error' ? '收到无法识别的事件，控制台正在重新同步权威快照；当前数据不能视为最新事实。' : `以下为 ${formatTime(snapshot.data?.generated_at ?? realtime.lastSuccessfulUpdate)} 的最近结果，可能已过期。轮询与事件流正在恢复。`}
          />
        </div>
      )}
      <main id="main-content" className="app-main" tabIndex={-1}>
        {children}
      </main>
      <div className="sr-only" aria-live="polite" aria-atomic="true">{realtime.announcement || connectionLabel}</div>
    </>
  );
}
