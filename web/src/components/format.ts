const dateFormatter = new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'medium' });
const numberFormatter = new Intl.NumberFormat('zh-CN', { maximumFractionDigits: 2 });
const byteFormatter = new Intl.NumberFormat('zh-CN', { style: 'unit', unit: 'megabyte', unitDisplay: 'short', maximumFractionDigits: 1 });

export function formatTime(value: string | null): string {
  if (value === null) return '尚无可确认的时间';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? '无法识别的时间' : dateFormatter.format(date);
}

export function absoluteTime(value: string | null): string {
  if (value === null) return '无';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : `${dateFormatter.format(date)}（${Intl.DateTimeFormat().resolvedOptions().timeZone}）`;
}

export function formatNumber(value: number | null, suffix = ''): string {
  return value === null ? '无法获取' : `${numberFormatter.format(value)}${suffix}`;
}

export function formatBytes(value: number | null): string {
  return value === null ? '无法获取' : byteFormatter.format(value / 1_000_000);
}

export function formatDuration(value: number | null): string {
  return value === null ? '不适用' : `${numberFormatter.format(value)} 毫秒`;
}

export function shortId(value: string): string {
  return value.length > 12 ? `${value.slice(0, 8)}…${value.slice(-4)}` : value;
}

export function yesNo(value: boolean): string {
  return value ? '是' : '否';
}
