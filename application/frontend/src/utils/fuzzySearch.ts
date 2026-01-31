/**
 * Simple fuzzy match - checks if all characters of the query appear in order in the target
 * Reused from TimezoneSelector for consistency across the application
 */
export function fuzzyMatch(query: string, target: string): { matches: boolean; score: number } {
    const queryLower = query.toLowerCase();
    const targetLower = target.toLowerCase();
    
    // Empty query matches everything
    if (!queryLower) {
        return { matches: true, score: 0 };
    }
    
    // Exact match gets highest score
    if (targetLower === queryLower) {
        return { matches: true, score: 1000 };
    }
    
    // Starts with query gets high score
    if (targetLower.startsWith(queryLower)) {
        return { matches: true, score: 500 + (queryLower.length / targetLower.length) * 100 };
    }
    
    // Contains query as substring gets medium score
    if (targetLower.includes(queryLower)) {
        const index = targetLower.indexOf(queryLower);
        return { matches: true, score: 200 - index };
    }
    
    // Fuzzy character-by-character match
    let queryIdx = 0;
    let score = 0;
    let consecutiveMatches = 0;
    
    for (let i = 0; i < targetLower.length && queryIdx < queryLower.length; i++) {
        if (targetLower[i] === queryLower[queryIdx]) {
            queryIdx++;
            consecutiveMatches++;
            score += consecutiveMatches * 10; // Reward consecutive matches
        } else {
            consecutiveMatches = 0;
        }
    }
    
    if (queryIdx === queryLower.length) {
        return { matches: true, score };
    }
    
    return { matches: false, score: 0 };
}

/**
 * Filter and sort an array of strings using fuzzy search
 */
export function fuzzyFilter(query: string, items: string[]): string[] {
    if (!query) {
        return items;
    }
    
    const results: Array<{ item: string; score: number }> = [];
    
    for (const item of items) {
        const { matches, score } = fuzzyMatch(query, item);
        if (matches) {
            results.push({ item, score });
        }
    }
    
    // Sort by score descending, then alphabetically for ties
    results.sort((a, b) => {
        if (b.score !== a.score) {
            return b.score - a.score;
        }
        return a.item.localeCompare(b.item);
    });
    
    return results.map(r => r.item);
}
