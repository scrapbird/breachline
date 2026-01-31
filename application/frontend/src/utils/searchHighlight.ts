/**
 * Utility functions for extracting and highlighting free-text search terms
 * from filter queries.
 */

/**
 * Extract free-text search terms from a filter query.
 * Excludes field=value, field!=value, field~value, field!~value conditions.
 * Wildcard patterns (ending with *) extract only the literal prefix.
 * 
 * Examples:
 *   "filter foo" → ["foo"]
 *   "filter foo bar" → ["foo", "bar"]
 *   "filter \"hello world\"" → ["hello world"]
 *   "filter foo OR bar" → ["foo", "bar"]
 *   "filter username=admin" → []
 *   "filter foo AND username=admin" → ["foo"]
 *   "filter scr*" → ["scr"] (wildcard prefix only)
 */
export function extractSearchTerms(query: string): string[] {
  if (!query || !query.trim()) {
    return [];
  }

  const terms: string[] = [];
  
  // Split by pipe to handle multiple stages
  const stages = splitByPipe(query);
  
  for (const stage of stages) {
    const trimmedStage = stage.trim();
    
    // Only process filter stages
    if (!trimmedStage.toLowerCase().startsWith('filter ')) {
      continue;
    }
    
    // Remove the "filter " prefix
    const filterContent = trimmedStage.substring(7).trim();
    
    if (!filterContent) {
      continue;
    }
    
    // Extract terms from the filter content
    const extracted = extractTermsFromFilterContent(filterContent);
    terms.push(...extracted);
  }
  
  // Remove duplicates and return
  return [...new Set(terms.map(t => t.toLowerCase()))];
}

/**
 * Split query by pipe character, respecting quoted strings.
 */
function splitByPipe(query: string): string[] {
  const stages: string[] = [];
  let current = '';
  let inQuote = false;
  let quoteChar = '';
  
  for (let i = 0; i < query.length; i++) {
    const char = query[i];
    
    if ((char === '"' || char === "'") && (i === 0 || query[i - 1] !== '\\')) {
      if (!inQuote) {
        inQuote = true;
        quoteChar = char;
      } else if (char === quoteChar) {
        inQuote = false;
        quoteChar = '';
      }
      current += char;
    } else if (char === '|' && !inQuote) {
      stages.push(current.trim());
      current = '';
    } else {
      current += char;
    }
  }
  
  if (current.trim()) {
    stages.push(current.trim());
  }
  
  return stages;
}

/**
 * Extract free-text search terms from filter content.
 * Skips field operators (=, !=, ~, !~) and boolean keywords.
 */
function extractTermsFromFilterContent(content: string): string[] {
  const terms: string[] = [];
  const tokens = tokenizeFilterContent(content);
  
  // Boolean operator keywords to skip
  const booleanKeywords = new Set(['and', 'or', 'not']);
  // Time filter keywords to skip
  const timeKeywords = new Set(['before', 'after']);
  
  for (const token of tokens) {
    const lowerToken = token.toLowerCase();
    
    // Skip boolean operators
    if (booleanKeywords.has(lowerToken)) {
      continue;
    }
    
    // Skip time filter keywords
    if (timeKeywords.has(lowerToken)) {
      continue;
    }
    
    // Skip parentheses
    if (token === '(' || token === ')') {
      continue;
    }
    
    // Skip field=value, field!=value, field~value, field!~value conditions
    if (token.includes('=') || token.includes('~')) {
      continue;
    }
    
    // Skip if it looks like a time filter value (after/before values are typically ISO dates or relative times)
    // These are handled by time filter keywords above, but just in case
    if (/^\d{4}-\d{2}-\d{2}/.test(token)) {
      continue;
    }
    
    // This is a free-text search term
    // Remove surrounding quotes if present
    let term = token;
    if ((term.startsWith('"') && term.endsWith('"')) || 
        (term.startsWith("'") && term.endsWith("'"))) {
      term = term.slice(1, -1);
    }
    
    // Handle wildcard patterns - only highlight the literal prefix
    // e.g., "scr*" should highlight "scr", not the full matched text
    if (term.endsWith('*')) {
      term = term.slice(0, -1);
    }
    
    if (term.trim()) {
      terms.push(term.trim());
    }
  }
  
  return terms;
}

/**
 * Tokenize filter content, respecting quoted strings.
 */
function tokenizeFilterContent(content: string): string[] {
  const tokens: string[] = [];
  let current = '';
  let inQuote = false;
  let quoteChar = '';
  
  for (let i = 0; i < content.length; i++) {
    const char = content[i];
    
    if ((char === '"' || char === "'") && (i === 0 || content[i - 1] !== '\\')) {
      if (!inQuote) {
        inQuote = true;
        quoteChar = char;
        current += char;
      } else if (char === quoteChar) {
        inQuote = false;
        quoteChar = '';
        current += char;
        // End of quoted string, push token
        tokens.push(current);
        current = '';
      } else {
        current += char;
      }
    } else if (!inQuote && (char === ' ' || char === '\t')) {
      // Whitespace outside quotes - end current token
      if (current.trim()) {
        tokens.push(current.trim());
      }
      current = '';
    } else if (!inQuote && (char === '(' || char === ')')) {
      // Parentheses outside quotes
      if (current.trim()) {
        tokens.push(current.trim());
      }
      tokens.push(char);
      current = '';
    } else {
      current += char;
    }
  }
  
  // Push final token
  if (current.trim()) {
    tokens.push(current.trim());
  }
  
  return tokens;
}

/**
 * Match information for highlighting
 */
export interface MatchRange {
  start: number;
  end: number;
}

/**
 * Find all occurrences of search terms in a text string.
 * Returns ranges (start, end) for each match.
 * Case-insensitive matching.
 */
export function findMatches(text: string, terms: string[]): MatchRange[] {
  if (!text || !terms || terms.length === 0) {
    return [];
  }
  
  const matches: MatchRange[] = [];
  const lowerText = text.toLowerCase();
  
  for (const term of terms) {
    const lowerTerm = term.toLowerCase();
    let startIndex = 0;
    
    while (startIndex < lowerText.length) {
      const index = lowerText.indexOf(lowerTerm, startIndex);
      if (index === -1) {
        break;
      }
      
      matches.push({
        start: index,
        end: index + term.length,
      });
      
      startIndex = index + 1;
    }
  }
  
  // Sort by start position and merge overlapping ranges
  return mergeOverlappingRanges(matches);
}

/**
 * Merge overlapping match ranges into non-overlapping ranges.
 */
function mergeOverlappingRanges(ranges: MatchRange[]): MatchRange[] {
  if (ranges.length === 0) {
    return [];
  }
  
  // Sort by start position
  const sorted = [...ranges].sort((a, b) => a.start - b.start);
  
  const merged: MatchRange[] = [sorted[0]];
  
  for (let i = 1; i < sorted.length; i++) {
    const current = sorted[i];
    const last = merged[merged.length - 1];
    
    if (current.start <= last.end) {
      // Overlapping or adjacent - merge
      last.end = Math.max(last.end, current.end);
    } else {
      // No overlap - add new range
      merged.push(current);
    }
  }
  
  return merged;
}
