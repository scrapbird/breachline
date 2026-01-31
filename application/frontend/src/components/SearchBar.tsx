import React from 'react';
import { fuzzyFilter } from '../utils/fuzzySearch';

export type SearchBarProps = {
  appliedQuery: string;
  onApply: (text: string) => void;
  inputRef: React.RefObject<HTMLInputElement>;
  history: string[];
  queryError?: string | null;
};

const SearchBar: React.FC<SearchBarProps> = React.memo(({ appliedQuery, onApply, inputRef, history, queryError }) => {
  const [text, setText] = React.useState<string>(appliedQuery || "");
  // Keep local text in sync when applied query changes externally
  React.useEffect(() => {
    setText(appliedQuery || "");
  }, [appliedQuery]);
  const disabled = (text === appliedQuery);
  const [showHistory, setShowHistory] = React.useState<boolean>(false);
  const [selectedIdx, setSelectedIdx] = React.useState<number>(-1);
  const dropdownRef = React.useRef<HTMLDivElement | null>(null);
  const filteredHistory = React.useMemo(() => {
    return fuzzyFilter(text, history);
  }, [history, text]);
  // Clamp or reset selection when filtered list changes or dropdown is hidden
  React.useEffect(() => {
    if (!showHistory) { setSelectedIdx(-1); return; }
    if (!filteredHistory || filteredHistory.length === 0) { setSelectedIdx(-1); return; }
    if (selectedIdx < 0 || selectedIdx >= filteredHistory.length) {
      setSelectedIdx(-1);
    }
  }, [filteredHistory, showHistory]);
  // Ensure the selected item scrolls into view
  React.useEffect(() => {
    if (!showHistory) return;
    if (selectedIdx < 0) return;
    const cont = dropdownRef.current;
    if (!cont) return;
    const el = cont.querySelector(`[data-idx="${selectedIdx}"]`) as HTMLElement | null;
    if (el && typeof el.scrollIntoView === 'function') {
      el.scrollIntoView({ block: 'nearest' });
    }
  }, [selectedIdx, showHistory]);

  return (
    <div className="topbar" style={{ display: 'flex', gap: 8, alignItems: 'center', marginBottom: 0, paddingTop: 2, paddingBottom: 2, flexWrap: 'nowrap' }}>
      <div style={{ position: 'relative', flex: 1, minWidth: 0 }}>
        <input
          aria-label="SPL Search"
          placeholder={'Search (SPL): status>=500 OR error="timeout" | host=api* env=prod'}
          value={text}
          ref={inputRef}
          onFocus={() => setShowHistory(true)}
          onBlur={() => { setTimeout(() => setShowHistory(false), 120); }}
          onChange={(e) => setText(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Escape') {
              e.preventDefault();
              e.stopPropagation();
              setShowHistory(false);
              setSelectedIdx(-1);
              return;
            }
            if (e.key === 'ArrowDown' || e.key === 'Down') {
              e.preventDefault();
              // If dropdown hidden, show and select first item
              if (!showHistory) {
                setShowHistory(true);
                if (filteredHistory && filteredHistory.length > 0) setSelectedIdx(0);
                return;
              }
              if (filteredHistory && filteredHistory.length > 0) {
                setSelectedIdx((prev) => {
                  const next = (prev < 0) ? 0 : Math.min(prev + 1, filteredHistory.length - 1);
                  return next;
                });
              }
              return;
            }
            if (e.key === 'ArrowUp' || e.key === 'Up') {
              e.preventDefault();
              // If dropdown hidden, show and select last item
              if (!showHistory) {
                setShowHistory(true);
                if (filteredHistory && filteredHistory.length > 0) setSelectedIdx(filteredHistory.length - 1);
                return;
              }
              if (filteredHistory && filteredHistory.length > 0) {
                setSelectedIdx((prev) => {
                  const next = (prev < 0) ? (filteredHistory.length - 1) : Math.max(prev - 1, 0);
                  return next;
                });
              }
              return;
            }
            if (e.key === 'Enter') {
              // If a dropdown item is selected, apply that; else apply current text
              if (showHistory && selectedIdx >= 0 && filteredHistory && filteredHistory[selectedIdx]) {
                const chosen = filteredHistory[selectedIdx];
                onApply(chosen);
                setText(chosen);
                setShowHistory(false);
                setSelectedIdx(-1);
              } else {
                onApply(text);
                setShowHistory(false);
              }
              return;
            }
          }}
          style={{ 
            width: '100%', 
            padding: '8px 12px', 
            borderRadius: 8, 
            border: queryError ? '2px solid #ff4444' : '1px solid #444', 
            background: '#1e1e1e', 
            color: '#eee', 
            boxSizing: 'border-box',
            outline: queryError ? '0 0 0 1px rgba(255, 68, 68, 0.3)' : 'none'
          }}
        />
        {showHistory && (
          <div ref={dropdownRef} style={{ position: 'absolute', top: 'calc(100% + 4px)', left: 0, right: 0, maxHeight: 240, overflowY: 'auto', background: '#1e1e1e', border: '1px solid #444', borderRadius: 8, zIndex: 3000, boxShadow: '0 6px 18px rgba(0,0,0,0.45)', textAlign: 'left' }}>
            {(filteredHistory && filteredHistory.length > 0) ? (
              filteredHistory.map((h, idx) => {
                const active = idx === selectedIdx;
                return (
                  <div
                    key={idx}
                    data-idx={idx}
                    title={h}
                    onMouseDown={(e) => { e.preventDefault(); setText(h); onApply(h); setShowHistory(false); setSelectedIdx(-1); }}
                    style={{ padding: '8px 12px', cursor: 'pointer', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', borderBottom: '1px solid #333', textAlign: 'left', background: active ? '#2a2a2a' : 'transparent' }}
                  >
                    {h}
                  </div>
                );
              })
            ) : (
              <div style={{ padding: '8px 12px', color: '#aaa', userSelect: 'none' }}>
                {history && history.length > 0 ? `No queries match "${text}"` : 'No previous queries yet'}
              </div>
            )}
          </div>
        )}
      </div>
      <button
        onClick={() => onApply(text)}
        disabled={disabled}
        style={{ padding: '8px 12px', borderRadius: 8, border: '1px solid #444', background: '#3a3f41', color: '#eee', cursor: (disabled ? 'default' : 'pointer'), opacity: (disabled ? 0.6 : 1), flex: '0 0 auto', position: 'relative', zIndex: 1101 }}
        title="Apply search"
      >
        Apply
      </button>
    </div>
  );
});

export default SearchBar;
