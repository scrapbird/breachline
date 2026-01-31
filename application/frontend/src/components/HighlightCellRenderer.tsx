import React from 'react';
import { findMatches, MatchRange } from '../utils/searchHighlight';

/**
 * AG Grid cell renderer params - extends standard params with our custom searchTerms
 */
interface HighlightCellRendererProps {
  value: any;
  searchTerms?: string[];
  valueFormatted?: string | null; // AG Grid provides this if valueFormatter is used
  data?: any; // Row data from AG Grid
  colDef?: any; // Column definition from AG Grid
  column?: any; // Column from AG Grid
}

/**
 * Custom AG Grid cell renderer that highlights search terms in cell content.
 * Uses the existing dark theme color scheme for highlights.
 */
const HighlightCellRenderer: React.FC<HighlightCellRendererProps> = (props) => {
  const { value, searchTerms, valueFormatted, data, colDef } = props;
  
  // Determine the text to display:
  // 1. Use valueFormatted if it's a string (including empty - JPath may resolve to empty)
  // 2. Otherwise use the raw value
  // 3. As a fallback, try to get value from row data using field
  // 4. As final fallback, try valueGetter if defined
  let textValue: string;
  if (typeof valueFormatted === 'string') {
    // valueFormatted is set by valueFormatter (e.g., JPath) - respect it even if empty
    textValue = valueFormatted;
  } else if (value !== null && value !== undefined) {
    textValue = String(value);
  } else if (data && colDef) {
    // Fallback: try to get value from row data using field
    let fallbackValue: any;
    if (colDef.field) {
      fallbackValue = data[colDef.field];
    } else if (colDef.valueGetter && typeof colDef.valueGetter === 'function') {
      // If there's a valueGetter, call it to get the value
      try {
        fallbackValue = colDef.valueGetter({ data, colDef, column: props.column });
      } catch {
        fallbackValue = undefined;
      }
    }
    textValue = fallbackValue !== null && fallbackValue !== undefined ? String(fallbackValue) : '';
  } else {
    textValue = '';
  }
  
  // If no search terms or empty value, just render the text
  // Use inline style to ensure text is visible (inherit from parent)
  const baseStyle: React.CSSProperties = { display: 'inline', color: 'inherit' };
  
  if (!searchTerms || searchTerms.length === 0 || !textValue) {
    return <span style={baseStyle}>{textValue}</span>;
  }
  
  // Find matches in the text
  const matches = findMatches(textValue, searchTerms);
  
  // If no matches, just render the text
  if (matches.length === 0) {
    return <span style={baseStyle}>{textValue}</span>;
  }
  
  // Split text into segments and render with highlighting
  return <span style={baseStyle}>{renderHighlightedText(textValue, matches)}</span>;
};

/**
 * Render text with highlighted segments.
 * Splits text into alternating normal and highlighted spans.
 */
function renderHighlightedText(text: string, matches: MatchRange[]): React.ReactNode[] {
  const segments: React.ReactNode[] = [];
  let lastEnd = 0;
  
  for (let i = 0; i < matches.length; i++) {
    const match = matches[i];
    
    // Add non-highlighted text before this match
    if (match.start > lastEnd) {
      segments.push(
        <span key={`text-${i}`}>
          {text.slice(lastEnd, match.start)}
        </span>
      );
    }
    
    // Add highlighted match
    segments.push(
      <span 
        key={`match-${i}`} 
        className="search-highlight"
      >
        {text.slice(match.start, match.end)}
      </span>
    );
    
    lastEnd = match.end;
  }
  
  // Add remaining text after last match
  if (lastEnd < text.length) {
    segments.push(
      <span key="text-final">
        {text.slice(lastEnd)}
      </span>
    );
  }
  
  return segments;
}

export default HighlightCellRenderer;
