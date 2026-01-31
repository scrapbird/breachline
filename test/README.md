# Timezone Test Files

Test files for manually verifying timezone handling during file ingestion.

## Files

Each test case is available in CSV, JSON, and XLSX formats:

### `timestamps_no_tz.*`
Timestamps **without** any timezone information. These should be interpreted using:
- The **Ingest Timezone Override** (if set on the file tab)
- The **Default Ingest Timezone** (from settings, if no override)

Use these files to verify that the default/override timezone settings work correctly.

### `timestamps_with_utc.*`
Timestamps **with UTC timezone** indicators (`Z` suffix or `UTC` suffix).
These should always be interpreted as UTC regardless of any timezone settings.

Use these files to verify that explicit UTC timestamps are handled correctly.

### `timestamps_with_offsets.*`
Timestamps **with explicit timezone offsets** (e.g., `+13:00`, `-05:00`).
These should use their embedded timezone offset regardless of any timezone settings.

Use these files to verify that explicit offset timestamps are handled correctly.

### `timestamps_mixed_formats.*`
A **mix of timestamp formats** including:
- ISO8601 with `Z` suffix
- ISO8601 with timezone offsets
- Space-separated without timezone
- US date format (MM/DD/YYYY)
- EU date format (DD/MM/YYYY)
- With milliseconds/microseconds
- Month name formats

Use these files to verify format detection and timezone handling across various formats.

## Test Scenarios

### 1. Default Ingest Timezone
1. Open a `timestamps_no_tz` file without setting any override
2. Verify timestamps are interpreted in the Default Ingest Timezone (from Settings)
3. Change the Default Ingest Timezone in Settings and reload
4. Verify timestamps change accordingly

### 2. Ingest Timezone Override
1. Open a `timestamps_no_tz` file
2. Set an Ingest Timezone Override on the file tab
3. Verify timestamps are interpreted in the override timezone
4. Verify override takes precedence over default

### 3. Timestamps with Embedded Timezone
1. Open `timestamps_with_utc` files
2. Verify timestamps show correctly in UTC
3. Open `timestamps_with_offsets` files
4. Verify each timestamp is converted from its embedded offset to display timezone
5. Check that timezone override has no effect on these files

### 4. Display Timezone
1. Open any file with timezone-aware timestamps
2. Change the Display Timezone in Settings
3. Verify timestamps are displayed in the new display timezone

## Expected Behavior Summary

| Timestamp Type | Ingest Override | Default Ingest | Result |
|---------------|-----------------|----------------|--------|
| No timezone | Set | Any | Uses override |
| No timezone | Not set | Set | Uses default |
| With Z/UTC | Any | Any | Uses UTC |
| With offset | Any | Any | Uses embedded offset |

## Regenerating XLSX Files

If you need to regenerate the XLSX files:

```bash
cd test
source venv/bin/activate
python3 generate_xlsx.py --output-dir .
```
