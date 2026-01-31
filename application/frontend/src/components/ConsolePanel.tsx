import React from 'react';

export type LogEntry = { ts: number; level: 'info' | 'warn' | 'error'; message: string };

export type ConsolePanelProps = {
  show: boolean;
  height: number;
  onHeightChange: (h: number) => void;
  logs: LogEntry[];
  onClear: () => void;
};

const ConsolePanel: React.FC<ConsolePanelProps> = ({ show, height, onHeightChange, logs, onClear }) => {
  const consoleBodyRef = React.useRef<HTMLDivElement | null>(null);
  const isResizingRef = React.useRef<boolean>(false);
  const startYRef = React.useRef<number>(0);
  const startHRef = React.useRef<number>(0);

  // Auto-scroll console when new logs come in
  React.useEffect(() => {
    if (!show) return;
    const el = consoleBodyRef.current;
    if (!el) return;
    el.scrollTop = el.scrollHeight;
  }, [logs, show]);

  const onConsoleResizeStart = (e: React.MouseEvent<HTMLDivElement>) => {
    isResizingRef.current = true;
    startYRef.current = e.clientY;
    startHRef.current = height;
    window.addEventListener('mousemove', onConsoleResizeMove, true);
    window.addEventListener('mouseup', onConsoleResizeEnd, true);
  };
  const onConsoleResizeMove = (e: MouseEvent) => {
    if (!isResizingRef.current) return;
    const dy = startYRef.current - e.clientY; // dragging up increases console height
    const next = Math.max(80, startHRef.current + dy);
    onHeightChange(next);
  };
  const onConsoleResizeEnd = () => {
    isResizingRef.current = false;
    window.removeEventListener('mousemove', onConsoleResizeMove, true);
    window.removeEventListener('mouseup', onConsoleResizeEnd, true);
  };

  // Cleanup on unmount for resize listeners
  React.useEffect(() => {
    return () => {
      window.removeEventListener('mousemove', onConsoleResizeMove, true);
      window.removeEventListener('mouseup', onConsoleResizeEnd, true);
    };
  }, []);

  if (!show) return null;

  return (
    <>
      <div className="resizer-h" onMouseDown={onConsoleResizeStart} title="Drag to resize console" />
      <div className="console" style={{ height }}>
        <div className="console-header">
          <div>Console</div>
          <div style={{ display: 'flex', gap: 8 }}>
            <button onClick={onClear} className="console-btn">Clear</button>
          </div>
        </div>
        <div className="console-body" ref={consoleBodyRef}>
          {logs.length === 0 ? (
            <div style={{ opacity: 0.7 }}>No messages</div>
          ) : (
            logs.map((l, i) => (
              <div key={i} className={`log-row ${l.level}`}>
                <span className="ts">{new Date(l.ts).toLocaleTimeString()}</span>
                <span className="lvl">[{l.level.toUpperCase()}]</span>
                <span className="msg">{l.message}</span>
              </div>
            ))
          )}
        </div>
      </div>
    </>
  );
};

export default ConsolePanel;
