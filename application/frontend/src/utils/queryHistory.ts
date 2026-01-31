const STORAGE_KEY = 'breachline.queryHistory';
const MAX_HISTORY = 50;

export function loadQueryHistory(): string[] {
    try {
        const raw = localStorage.getItem(STORAGE_KEY);
        if (raw) {
            const arr = JSON.parse(raw);
            if (Array.isArray(arr)) {
                return arr.filter((x: any) => typeof x === 'string');
            }
        }
    } catch {}
    return [];
}

export function saveQueryHistory(arr: string[]): void {
    try {
        localStorage.setItem(STORAGE_KEY, JSON.stringify(arr));
    } catch {}
}

export function addToQueryHistory(currentHistory: string[], query: string): string[] {
    const trimmed = (query || '').trim();
    if (!trimmed) return currentHistory;

    let next = currentHistory.filter((x) => (x || '').trim() !== trimmed);
    next = [trimmed, ...next];
    if (next.length > MAX_HISTORY) next = next.slice(0, MAX_HISTORY);

    saveQueryHistory(next);
    return next;
}
