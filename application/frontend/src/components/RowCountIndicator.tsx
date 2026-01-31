import React from 'react';

export type RowCountIndicatorProps = {
  visible: boolean;
  totalRows: number | null;
};

const RowCountIndicator: React.FC<RowCountIndicatorProps> = ({ visible, totalRows }) => {
  const [hidden, setHidden] = React.useState<boolean>(false);
  const indicatorRef = React.useRef<HTMLDivElement | null>(null);
  const hideTimerRef = React.useRef<number | null>(null);
  const timeoutElapsedRef = React.useRef<boolean>(false);
  const trackingMouseRef = React.useRef<boolean>(false);
  const savedRectRef = React.useRef<DOMRect | null>(null);
  const moveHandlerRef = React.useRef<((ev: MouseEvent) => void) | null>(null);
  const insideRef = React.useRef<boolean>(false);
  const hiddenRef = React.useRef<boolean>(false);
  React.useEffect(() => { hiddenRef.current = hidden; }, [hidden]);

  React.useEffect(() => {
    // Cleanup on unmount
    return () => {
      if (hideTimerRef.current) {
        window.clearTimeout(hideTimerRef.current);
        hideTimerRef.current = null;
      }
      if (trackingMouseRef.current && moveHandlerRef.current) {
        window.removeEventListener('mousemove', moveHandlerRef.current, true);
        trackingMouseRef.current = false;
        moveHandlerRef.current = null;
      }
    };
  }, []);

  if (!visible) return null;

  return (
    <div
      ref={indicatorRef}
      className={`row-count-indicator ${hidden ? 'hidden' : ''}`}
      title="Number of rows in current result set"
      onMouseEnter={() => {
        const el = indicatorRef.current;
        if (!el) return;
        savedRectRef.current = el.getBoundingClientRect();
        insideRef.current = true;
        timeoutElapsedRef.current = false;
        setHidden(true);

        const onMove = (ev: MouseEvent) => {
          const r = savedRectRef.current;
          if (!r) return;
          const inside = ev.clientX >= r.left && ev.clientX <= r.right && ev.clientY >= r.top && ev.clientY <= r.bottom;
          insideRef.current = inside;
          if (timeoutElapsedRef.current && !inside && hiddenRef.current) {
            setHidden(false);
            window.removeEventListener('mousemove', onMove, true);
            trackingMouseRef.current = false;
            moveHandlerRef.current = null;
          }
        };

        if (!trackingMouseRef.current) {
          window.addEventListener('mousemove', onMove, true);
          trackingMouseRef.current = true;
          moveHandlerRef.current = onMove;
        }

        if (hideTimerRef.current) window.clearTimeout(hideTimerRef.current);
        hideTimerRef.current = window.setTimeout(() => {
          timeoutElapsedRef.current = true;
          if (!insideRef.current && hiddenRef.current) {
            setHidden(false);
            if (trackingMouseRef.current) {
              const mh = moveHandlerRef.current;
              if (mh) window.removeEventListener('mousemove', mh, true);
              trackingMouseRef.current = false;
              moveHandlerRef.current = null;
            }
          }
        }, 5000);
      }}
    >
      <span style={{ opacity: 0.8, marginRight: 6 }}>Rows:</span>
      {totalRows == null ? (
        <i className="fas fa-spinner fa-spin" aria-label="Counting rows" />
      ) : (
        <span>{Number(totalRows).toLocaleString()}</span>
      )}
    </div>
  );
};

export default RowCountIndicator;
