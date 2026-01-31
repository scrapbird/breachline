import React, { useRef, useEffect } from 'react';

export interface AnnotationInfo {
  originalRowIndex: number;
  displayRowIndex: number;
  note: string;
  color: string;
}

export interface AnnotationPanelProps {
  show: boolean;
  height: number;
  onHeightChange: (h: number) => void;
  annotations: AnnotationInfo[];
  onAnnotationClick: (displayIndex: number) => void;
  onEditAnnotation: (displayIndex: number) => void;
}

const colorClasses: Record<string, string> = {
  blue: 'annotation-color-blue',
  green: 'annotation-color-green',
  yellow: 'annotation-color-yellow',
  orange: 'annotation-color-orange',
  red: 'annotation-color-red',
  grey: 'annotation-color-grey',
  gray: 'annotation-color-grey',
  white: 'annotation-color-white',
};

const AnnotationPanel: React.FC<AnnotationPanelProps> = ({
  show,
  height,
  onHeightChange,
  annotations,
  onAnnotationClick,
  onEditAnnotation,
}) => {
  const panelBodyRef = useRef<HTMLDivElement | null>(null);
  const isResizingRef = useRef<boolean>(false);
  const startYRef = useRef<number>(0);
  const startHRef = useRef<number>(0);

  // Filter to only show annotations that are visible in current view (displayRowIndex >= 0)
  const visibleAnnotations = annotations
    .filter(a => a.displayRowIndex >= 0)
    .sort((a, b) => a.displayRowIndex - b.displayRowIndex);

  const onResizeStart = (e: React.MouseEvent<HTMLDivElement>) => {
    isResizingRef.current = true;
    startYRef.current = e.clientY;
    startHRef.current = height;
    window.addEventListener('mousemove', onResizeMove, true);
    window.addEventListener('mouseup', onResizeEnd, true);
  };

  const onResizeMove = (e: MouseEvent) => {
    if (!isResizingRef.current) return;
    const dy = startYRef.current - e.clientY; // dragging up increases panel height
    const next = Math.max(80, Math.min(400, startHRef.current + dy));
    onHeightChange(next);
  };

  const onResizeEnd = () => {
    isResizingRef.current = false;
    window.removeEventListener('mousemove', onResizeMove, true);
    window.removeEventListener('mouseup', onResizeEnd, true);
  };

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      window.removeEventListener('mousemove', onResizeMove, true);
      window.removeEventListener('mouseup', onResizeEnd, true);
    };
  }, []);

  if (!show) return null;

  return (
    <>
      <div 
        className="resizer-h annotation-panel-resizer" 
        onMouseDown={onResizeStart} 
        title="Drag to resize annotation panel" 
      />
      <div className="annotation-panel" style={{ height }}>
        <div className="annotation-panel-header">
          <div className="annotation-panel-title">
            Annotations
            <span className="annotation-count">({visibleAnnotations.length})</span>
          </div>
        </div>
        <div className="annotation-panel-body" ref={panelBodyRef}>
          {visibleAnnotations.length === 0 ? (
            <div className="annotation-empty">No annotations in current view</div>
          ) : (
            <table className="annotation-table">
              <thead>
                <tr>
                  <th className="annotation-col-index">Index</th>
                  <th className="annotation-col-note">Note</th>
                  <th className="annotation-col-actions"></th>
                </tr>
              </thead>
              <tbody>
                {visibleAnnotations.map((annot, idx) => {
                  const colorClass = colorClasses[annot.color.toLowerCase()] || 'annotation-color-grey';
                  return (
                    <tr
                      key={`${annot.originalRowIndex}-${idx}`}
                      className={`annotation-row ${colorClass}`}
                      onClick={() => onAnnotationClick(annot.displayRowIndex)}
                      title={annot.note || '(no note)'}
                    >
                      <td className="annotation-col-index">{annot.displayRowIndex + 1}</td>
                      <td className="annotation-col-note">
                        <span className="annotation-note-text">
                          {annot.note || <em className="annotation-no-note">(no note)</em>}
                        </span>
                      </td>
                      <td className="annotation-col-actions">
                        <button
                          className="annotation-edit-btn"
                          onClick={(e) => {
                            e.stopPropagation();
                            onEditAnnotation(annot.displayRowIndex);
                          }}
                          title="Edit annotation"
                        >
                          <i className="fa-solid fa-pen-to-square" />
                        </button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          )}
        </div>
      </div>
    </>
  );
};

export default AnnotationPanel;
