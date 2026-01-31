const FALLBACK_TIMEZONES = [
    'Pacific/Auckland', 'Australia/Sydney', 'Asia/Tokyo', 'Asia/Shanghai', 'Asia/Kolkata', 'Asia/Singapore',
    'Europe/London', 'Europe/Berlin', 'Europe/Paris', 'Europe/Moscow',
    'America/New_York', 'America/Chicago', 'America/Denver', 'America/Los_Angeles', 'America/Sao_Paulo',
    'Africa/Johannesburg', 'Pacific/Honolulu'
];

export function getTimezoneOptions(): string[] {
    let zones: string[] = [];

    try {
        const anyIntl = Intl as any;
        if (anyIntl && typeof anyIntl.supportedValuesOf === 'function') {
            const vals = anyIntl.supportedValuesOf('timeZone');
            if (Array.isArray(vals) && vals.length) {
                zones = vals as string[];
            }
        }
    } catch {}

    if (!zones || zones.length === 0) {
        zones = FALLBACK_TIMEZONES;
    }

    zones = Array.from(new Set(zones)).sort((a, b) => a.localeCompare(b));
    zones = zones.filter((z) => z !== 'UTC');

    return ['Local', 'UTC', ...zones];
}
