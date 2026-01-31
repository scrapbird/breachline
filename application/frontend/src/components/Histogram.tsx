import React, { useEffect, useMemo, useRef, useState } from 'react';
import ReactECharts from 'echarts-for-react';
import type { EChartsOption } from 'echarts';
import { SavePNGFromDataURL, CopyPNGFromDataURL } from '../../wailsjs/go/app/App';
import { ClipboardSetText } from '../../wailsjs/runtime/runtime';

export type HistogramBucket = { start: number; count: number };

interface HistogramProps {
  buckets: HistogramBucket[];
  height?: number;
  loading?: boolean;
  // Optional hint for bucket duration in milliseconds (e.g., bucketSeconds*1000 from parent)
  bucketMsHint?: number;
  // Fired when the user drag-selects a range on the histogram. Times are epoch ms.
  onRangeSelected?: (startMs: number, endMs: number) => void;
  // Display timezone for labels/tooltips. Examples: 'Local', 'UTC', 'America/Los_Angeles'
  displayTimeZone?: string;
}

// utils: format epoch ms -> 'yyyy-mm-dd hh:mm' in local time
function pad2(n: number) {
  return n < 10 ? `0${n}` : String(n);
}

function makeFormatters(tz?: string) {
  // Use Intl.DateTimeFormat with optional timeZone; fallback to local if invalid or 'Local'
  const resolvedTZ = (tz && tz.trim() && tz.trim().toUpperCase() !== 'LOCAL') ? tz.trim() : undefined;
  const fmtHm = new Intl.DateTimeFormat(undefined, {
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', hour12: false,
    timeZone: resolvedTZ,
  });
  const fmtHms = new Intl.DateTimeFormat(undefined, {
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false,
    timeZone: resolvedTZ,
  });
  const formatYmdHm = (epochMs: number) => {
    // Intl may include separators per locale; normalize to YYYY-MM-DD HH:MM
    const d = new Date(epochMs);
    const parts = fmtHm.formatToParts(d);
    const m = Object.fromEntries(parts.map(p => [p.type, p.value]));
    const yyyy = m.year || String(d.getFullYear());
    const mm = m.month || pad2(d.getMonth() + 1);
    const dd = m.day || pad2(d.getDate());
    const hh = m.hour || pad2(d.getHours());
    const mi = m.minute || pad2(d.getMinutes());
    return `${yyyy}-${mm}-${dd} ${hh}:${mi}`;
  };
  const formatYmdHms = (epochMs: number) => {
    const d = new Date(epochMs);
    const parts = fmtHms.formatToParts(d);
    const m = Object.fromEntries(parts.map(p => [p.type, p.value]));
    const yyyy = m.year || String(d.getFullYear());
    const mm = m.month || pad2(d.getMonth() + 1);
    const dd = m.day || pad2(d.getDate());
    const hh = m.hour || pad2(d.getHours());
    const mi = m.minute || pad2(d.getMinutes());
    const ss = m.second || pad2(d.getSeconds());
    return `${yyyy}-${mm}-${dd} ${hh}:${mi}:${ss}`;
  };
  return { formatYmdHm, formatYmdHms };
}

// Histogram rendered with ECharts
const Histogram: React.FC<HistogramProps> = ({ buckets, height = 200, loading = false, bucketMsHint, onRangeSelected, displayTimeZone }) => {
  // Track container width to decide label density responsively
  const containerRef = useRef<HTMLDivElement | null>(null);
  const chartRef = useRef<ReactECharts | null>(null);
  const chartDomRef = useRef<HTMLElement | null>(null);
  const detachCtxMenuRef = useRef<(() => void) | null>(null);
  const menuRef = useRef<HTMLDivElement | null>(null);
  const [containerWidth, setContainerWidth] = useState(0);
  const [menuState, setMenuState] = useState<{ visible: boolean; x: number; y: number }>({ visible: false, x: 0, y: 0 });
  // Drag-to-select state
  const isDraggingRef = useRef(false);
  const dragStartXRef = useRef<number | null>(null);
  // Build time formatters for the provided display timezone
  const { formatYmdHm, formatYmdHms } = useMemo(() => makeFormatters(displayTimeZone), [displayTimeZone]);
  const [selectionRect, setSelectionRect] = useState<{ left: number; width: number } | null>(null);

  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    // Use ResizeObserver to react to container size changes
    const ro = new ResizeObserver((entries) => {
      for (const entry of entries) {
        const cr = entry.contentRect;
        setContainerWidth(Math.max(0, Math.floor(cr.width)));
      }
    });
    ro.observe(el);
    // Initialize immediately
    setContainerWidth(Math.max(0, Math.floor(el.getBoundingClientRect().width)));
    return () => ro.disconnect();
  }, []);

  // Ensure hooks are always called in the same order every render
  const categories = useMemo(() => buckets.map((b) => String(b.start)), [buckets]);
  const data = useMemo(() => buckets.map((b) => b.count), [buckets]);
  const maxCount = useMemo(
    () => buckets.reduce((m, b) => Math.max(m, b.count), 0),
    [buckets]
  );

  const hasData = !(buckets.length === 0 || maxCount === 0);

  // Infer bucket duration in milliseconds (0 if indeterminable)
  const bucketMs = useMemo(() => {
    if (buckets.length >= 2) {
      const a = buckets[0].start;
      const b = buckets[1].start;
      const d = Math.abs(Number(b) - Number(a));
      return Number.isFinite(d) ? d : 0;
    }
    return 0;
  }, [buckets]);
  // Derive a fallback bucket size by computing the minimum positive delta across starts
  const effectiveBucketMs = useMemo(() => {
    if (bucketMs > 0) return bucketMs;
    if (!buckets || buckets.length < 2) {
      // Fallback to provided hint (e.g., from parent) when we can't infer from data
      return Math.max(0, Number(bucketMsHint) || 0);
    }
    let minDelta = Number.POSITIVE_INFINITY;
    for (let i = 1; i < buckets.length; i++) {
      const prev = Number(buckets[i - 1].start);
      const curr = Number(buckets[i].start);
      const d = curr - prev;
      if (Number.isFinite(d) && d > 0 && d < minDelta) minDelta = d;
    }
    if (Number.isFinite(minDelta) && minDelta !== Number.POSITIVE_INFINITY) return minDelta;
    // Last resort: use hint
    return Math.max(0, Number(bucketMsHint) || 0);
  }, [bucketMs, buckets, bucketMsHint]);

  // Decide label density based on container width to avoid overlap
  // Heuristic: assume ~110px per label (including spacing). Compute max labels that fit.
  // ECharts axisLabel.interval: 0 shows all labels; n shows one label per (n+1) ticks
  const axisLabelInterval = useMemo(() => {
    if (containerWidth <= 0) {
      // Fallback to simple rule when width is not known yet
      return buckets.length > 10 ? 4 : 0;
    }
    const approxPxPerLabel = 110; // includes text width + padding
    const maxLabels = Math.max(1, Math.floor(containerWidth / approxPxPerLabel));
    if (buckets.length <= maxLabels) return 0;
    // interval n => shows every (n+1)-th label
    return Math.max(0, Math.ceil(buckets.length / maxLabels) - 1);
  }, [buckets.length, containerWidth]);

  const option: EChartsOption = {
    backgroundColor: '#131313',
    grid: { left: 16, right: 16, top: 16, bottom: 28, containLabel: true },
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
      formatter: (params: any) => {
        const p = Array.isArray(params) ? params[0] : params;
        const ts = Number(p.axisValue);
        const fmt = bucketMs > 0 && bucketMs < 60_000 ? formatYmdHms : formatYmdHm;
        return `${fmt(ts)}<br/>Count: ${p.data}`;
      },
    },
    xAxis: {
      type: 'category',
      data: categories,
      boundaryGap: true,
      axisLine: { lineStyle: { color: '#666' } },
      axisTick: { alignWithLabel: true },
      axisLabel: {
        interval: axisLabelInterval,
        color: '#bbb',
        formatter: (value: string | number) => {
          const ts = Number(value);
          const fmt = bucketMs > 0 && bucketMs < 60_000 ? formatYmdHms : formatYmdHm;
          return fmt(ts);
        },
        hideOverlap: true,
      },
      splitLine: { show: false },
    },
    yAxis: {
      type: 'value',
      min: 0,
      axisLine: { show: false },
      axisLabel: { color: '#bbb' },
      splitLine: { lineStyle: { color: '#333' } },
    },
    series: [
      {
        type: 'bar',
        data,
        barWidth: '80%',
        itemStyle: { color: '#5b9bd5', borderRadius: [2, 2, 0, 0] },
        emphasis: { focus: 'series' },
      },
    ],
  };

  // Helpers to capture chart image
  const getChartDataURL = () => {
    try {
      const inst = chartRef.current?.getEchartsInstance?.();
      if (!inst) return '';
      // Use dark background to match chart bg
      return inst.getDataURL({ type: 'png', pixelRatio: 2, backgroundColor: '#131313' });
    } catch (e) {
      console.error('Failed to capture chart image', e);
      return '';
    }
  };

  const handleSave = async () => {
    const dataURL = getChartDataURL();
    if (!dataURL) return;
    try {
      await SavePNGFromDataURL(dataURL, 'histogram.png');
    } catch (e) {
      console.error('SavePNGFromDataURL failed', e);
    } finally {
      setMenuState((s) => ({ ...s, visible: false }));
    }
  };

  const handleCopy = async () => {
    const dataURL = getChartDataURL();
    if (!dataURL) return;
    try {
      // Prefer backend clipboard image copy for reliability across platforms
      try {
        const ok = await CopyPNGFromDataURL(dataURL);
        if (ok) {
          console.log('Copied histogram screenshot to clipboard (backend image).');
          return;
        }
      } catch (be) {
        // fall through to frontend methods
      }
      // Try writing an image to the clipboard (Chromium-based webviews generally support this)
      if (navigator.clipboard && 'ClipboardItem' in window) {
        const blob = await (await fetch(dataURL)).blob();
        const ClipboardItem = (window as any).ClipboardItem;
        const item = new ClipboardItem({ [blob.type]: blob });
        await navigator.clipboard.write([item]);
        console.log('Copied histogram screenshot to clipboard (image).');
      } else {
        // Fallback: copy data URL as text
        await navigator.clipboard.writeText(dataURL);
        console.log('Copied histogram screenshot to clipboard (as data URL text).');
      }
    } catch (e) {
      // Use Wails runtime clipboard API as a robust fallback for text
      try {
        const ok = await ClipboardSetText(dataURL);
        if (ok) {
          console.log('Copied histogram screenshot to clipboard via Wails (as data URL text).');
        } else {
          // Last resort fallback using execCommand
          const ta = document.createElement('textarea');
          ta.value = dataURL;
          document.body.appendChild(ta);
          ta.select();
          document.execCommand('copy');
          document.body.removeChild(ta);
          console.log('Copied histogram screenshot to clipboard using execCommand (as data URL text).');
        }
      } catch (err) {
        console.error('Failed to copy screenshot', err);
      }
    } finally {
      setMenuState((s) => ({ ...s, visible: false }));
    }
  };

  const onContextMenu = (e: React.MouseEvent) => {
    e.preventDefault();
    setMenuState({ visible: true, x: e.clientX, y: e.clientY });
  };

  useEffect(() => {
    const onClick = () => setMenuState((s) => ({ ...s, visible: false }));
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setMenuState((s) => ({ ...s, visible: false }));
    };
    window.addEventListener('click', onClick);
    window.addEventListener('keydown', onKeyDown);
    return () => {
      window.removeEventListener('click', onClick);
      window.removeEventListener('keydown', onKeyDown);
    };
  }, []);

  // Cleanup any attached contextmenu listener on unmount
  useEffect(() => {
    return () => {
      if (detachCtxMenuRef.current) {
        detachCtxMenuRef.current();
        detachCtxMenuRef.current = null;
      }
    };
  }, []);

  // Utilities to convert pixel -> epoch ms using the ECharts instance.
  // Determine the bucket index under a given x position (relative to chart DOM left)
  const bucketIndexFromPixel = (px: number): number | null => {
    const el = chartDomRef.current || containerRef.current;
    if (!el || buckets.length === 0) return null;
    const r = el.getBoundingClientRect();
    // Clamp within chart DOM width; ECharts expects pixel in container coordinate space
    const x = Math.max(0, Math.min(px, r.width));
    try {
      const inst: any = chartRef.current?.getEchartsInstance?.();
      if (inst && inst.convertFromPixel) {
        // Prefer xAxisIndex mapping so grid/label margins are accounted for
        const raw = inst.convertFromPixel({ xAxisIndex: 0 }, x);
        // raw can be number (possibly fractional) or category value
        let idx: number | null = null;
        if (typeof raw === 'number' && isFinite(raw)) {
          idx = Math.floor(raw);
        } else if (typeof raw === 'string') {
          const i = categories.indexOf(raw);
          idx = i >= 0 ? i : null;
        } else if (Array.isArray(raw) && typeof raw[0] === 'number') {
          idx = Math.floor(raw[0]);
        }
        if (idx == null || !isFinite(idx as number)) {
          // Fallback to proportional division if convertFromPixel was inconclusive
          const frac = r.width > 0 ? x / r.width : 0;
          idx = Math.floor(frac * buckets.length);
        }
        if (idx < 0) idx = 0;
        if (idx >= buckets.length) idx = buckets.length - 1;
        return idx;
      }
    } catch {
      // fall through to proportional fallback
    }
    const frac = r.width > 0 ? x / r.width : 0;
    let idx = Math.floor(frac * buckets.length);
    if (idx >= buckets.length) idx = buckets.length - 1;
    if (idx < 0) idx = 0;
    return idx;
  };

  const pixelToEpochMs = (px: number): number | null => {
    const idx = bucketIndexFromPixel(px);
    if (idx == null) return null;
    return Number(buckets[idx].start);
  };

  // Mouse handlers for drag selection over the chart area
  const onMouseDown = (e: React.MouseEvent) => {
    // Don't start drag selection if the context menu is visible or if clicking within the menu
    if (menuState.visible) return;
    if (menuRef.current && menuRef.current.contains(e.target as Node)) return;
    // Prefer the ECharts DOM for mouse coordinate space so convertFromPixel aligns correctly
    const coordEl = chartDomRef.current || containerRef.current;
    if (!coordEl) return;
    // Only respond to left button
    if (e.button !== 0) return;
    isDraggingRef.current = true;
    const rect = coordEl.getBoundingClientRect();
    const x = e.clientX - rect.left;
    dragStartXRef.current = x;
    setSelectionRect({ left: x, width: 0 });
    // Attach global listeners so drag continues outside the chart bounds
    const onMove = (ev: MouseEvent) => {
      const el = chartDomRef.current || containerRef.current;
      if (!isDraggingRef.current || !el) return;
      const r = el.getBoundingClientRect();
      let mx = ev.clientX - r.left;
      mx = Math.max(0, Math.min(mx, r.width));
      const start = dragStartXRef.current ?? mx;
      const left = Math.min(start, mx);
      const width = Math.abs(mx - start);
      setSelectionRect({ left, width });
    };
    const onUp = (ev: MouseEvent) => {
      const el = chartDomRef.current || containerRef.current;
      if (!isDraggingRef.current || !el) return;
      isDraggingRef.current = false;
      const r = el.getBoundingClientRect();
      let xEnd = ev.clientX - r.left;
      xEnd = Math.max(0, Math.min(xEnd, r.width));
      const xStart = dragStartXRef.current ?? xEnd;
      setSelectionRect(null);
      dragStartXRef.current = null;
      // Convert to bucket indices and snap to bucket boundaries
      const iA = bucketIndexFromPixel(xStart);
      const iB = bucketIndexFromPixel(xEnd);
      if (iA != null && iB != null) {
        const i0 = Math.min(iA, iB);
        const i1 = Math.max(iA, iB);
        const startMs = Number(buckets[i0].start);
        const bucketWidth = (effectiveBucketMs && effectiveBucketMs > 0) ? effectiveBucketMs : Math.max(0, Number(bucketMsHint) || 0);
        const endMs = Number(buckets[i1].start) + (bucketWidth || 0);
        if (onRangeSelected && Number.isFinite(startMs) && Number.isFinite(endMs) && endMs > startMs) {
          console.debug('[Histogram] Range selected', { startMs, endMs, i0, i1 });
          onRangeSelected(startMs, endMs);
        }
      }
      window.removeEventListener('mousemove', onMove, true);
      window.removeEventListener('mouseup', onUp, true);
    };
    window.addEventListener('mousemove', onMove, true);
    window.addEventListener('mouseup', onUp, true);
  };

  return (
    <div ref={containerRef} style={{ width: '100%', position: 'relative' }} onContextMenu={onContextMenu}>
      {hasData ? (
        <div style={{ position: 'relative' }} onMouseDownCapture={onMouseDown}>
          <div style={{ filter: loading ? 'grayscale(0.7)' : 'none', opacity: loading ? 0.5 : 1, transition: 'opacity 120ms ease' }}>
            <ReactECharts
              option={option}
              style={{ height, width: '100%', border: '1px solid #333', borderRadius: 8 }}
              notMerge
              lazyUpdate
              ref={chartRef}
              onChartReady={(inst: any) => {
                // Detach previous if any
                if (detachCtxMenuRef.current) {
                  detachCtxMenuRef.current();
                  detachCtxMenuRef.current = null;
                }
                try {
                  const dom = inst?.getDom?.() as HTMLElement | undefined;
                  if (!dom) return;
                  chartDomRef.current = dom;
                  const handler = (e: MouseEvent) => {
                    e.preventDefault();
                    setMenuState({ visible: true, x: e.clientX, y: e.clientY });
                  };
                  dom.addEventListener('contextmenu', handler);
                  detachCtxMenuRef.current = () => dom.removeEventListener('contextmenu', handler);
                } catch (e) {
                  // no-op
                }
              }}
            />
          </div>
          {/* Drag selection handled via onMouseDownCapture; keep overlay-free for proper tooltip hover */}
          {/* Visual selection rectangle */}
          {selectionRect && selectionRect.width > 2 && (
            <div
              style={{
                position: 'absolute',
                top: 2,
                bottom: 2,
                left: selectionRect.left,
                width: selectionRect.width,
                background: 'rgba(91,155,213,0.25)',
                border: '1px solid rgba(91,155,213,0.8)',
                pointerEvents: 'none', // Do not block ECharts hover/tooltip
                borderRadius: 4,
                zIndex: 3,
              }}
            />
          )}
          {loading && (
            <div
              style={{
                position: 'absolute',
                inset: 0,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                background: 'rgba(0,0,0,0.25)',
                borderRadius: 8,
                pointerEvents: 'none',
              }}
            >
              <i className="fas fa-spinner fa-spin" style={{ color: '#ddd', fontSize: 24 }} aria-label="Loading histogram" />
            </div>
          )}
          {menuState.visible && (
            <div
              ref={menuRef}
              style={{
                position: 'fixed',
                top: menuState.y,
                left: menuState.x,
                background: '#1b1b1b',
                border: '1px solid #333',
                borderRadius: 6,
                boxShadow: '0 4px 12px rgba(0,0,0,0.5)',
                zIndex: 1000,
                minWidth: 180,
                color: '#eee',
                overflow: 'hidden',
              }}
              onClick={(e) => e.stopPropagation()}
              onMouseDownCapture={(e) => { e.stopPropagation(); e.preventDefault(); }}
              onMouseUpCapture={(e) => { e.stopPropagation(); e.preventDefault(); }}
            >
              <button
                onClick={handleSave}
                style={{
                  display: 'block',
                  width: '100%',
                  textAlign: 'left',
                  padding: '8px 12px',
                  background: 'transparent',
                  border: 'none',
                  color: 'inherit',
                  cursor: 'pointer',
                }}
              >
                Save histogram
              </button>
              <button
                onClick={handleCopy}
                style={{
                  display: 'block',
                  width: '100%',
                  textAlign: 'left',
                  padding: '8px 12px',
                  background: 'transparent',
                  border: 'none',
                  color: 'inherit',
                  cursor: 'pointer',
                }}
              >
                Copy histogram
              </button>
            </div>
          )}
        </div>
      ) : (
        <div
          style={{
            height,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            color: '#888',
            border: '1px solid #333',
            borderRadius: 8,
            background: '#131313',
            position: 'relative',
          }}
        >
          No data
          {loading && (
            <div
              style={{
                position: 'absolute',
                inset: 0,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                background: 'rgba(0,0,0,0.25)',
                borderRadius: 8,
                pointerEvents: 'none',
              }}
            >
              <i className="fas fa-spinner fa-spin" style={{ color: '#ddd', fontSize: 24 }} aria-label="Loading histogram" />
            </div>
          )}
        </div>
      )}
    </div>
  );
};

export default Histogram;
