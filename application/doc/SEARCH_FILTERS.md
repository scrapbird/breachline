# Supported search filters (current implementation)

The current query engine is a simplified, SPL-like syntax implemented in the backend ([search.go](../app/search.go), [util.go](../app/util.go)). It is case-insensitive for both field names and values. The features below are supported today:

- **Basic terms**
  - Bare term: substring match across any field. Example: `error` matches any row containing "error" in any column.
  - Quoted phrase: spaces are respected. Example: `"connection reset"`.
  - Wildcard suffix for prefix search: a term ending with `*` is treated as a prefix. Example: `api*`.

- **Field equality/inequality**
  - `field=value` for case-insensitive equality.
  - `field!=value` for case-insensitive inequality.
  - `field=*` for existence check - matches rows where the field has a non-empty value (after trimming whitespace).
  - Values may end with `*` to mean prefix comparison. Examples: `host=api*`, `service!=web*`.
  - Spaced operators are allowed and merged: `field = value`, `field != value`.

- **Boolean logic**
  - Space between terms implies AND.
  - `OR` at the top level joins groups. Example: `status=500 OR error`.
  - `NOT` negates the next term (repeating `NOT` toggles). Example: `NOT error`.

- **Time filters**
  - `after <value>` and/or `before <value>` filter by a detected timestamp column.
  - Supported time values ([util.go](../app/util.go): `parseFlexibleTime`):
    - Relative: `15m`, `2h`, `3d`, `4w`, `6mo`, `1y`, optionally with `ago` (e.g., `15m ago`).
    - Absolute: RFC3339/RFC3339Nano (e.g., `2023-01-02T15:04:05Z`), `YYYY-MM-DD HH:MM:SS` (with optional `Z`/offset), or epoch seconds/milliseconds.
    - `now`.
  - Timestamp column detection ([util.go](../app/util.go): detectTimestampIndex): prefers `@timestamp`, `timestamp`, `time`, then fields containing `@timestamp`, `timestamp`, `datetime`, `date`, `time`, `ts`.

- **Columns projection stage**
  - Pipe stage to project columns: `| columns colA, colB, "col with spaces"`.
  - Only affects which columns are returned/rendered; it does not change matching.

- **Dedup stage**
  - Pipe stage to deduplicate rows based on one or more columns: `| dedup colA, colB`.
  - Keeps only the first row encountered for each unique value (or combination of values) of the specified columns.
  - Examples:
    - `dedup userName`
    - `status=500 | dedup userName, environment`
  - Notes:
    - Dedup operates on the raw field values (case-sensitive for dedup keys; matching remains case-insensitive).
    - Dedup runs after `columns` projection is parsed but uses the original row values of the columns you specify.
    - `limit N` applies after dedup, capping the number of unique rows considered.

- **Result limiting**
  - `limit N` caps the total number of matched rows considered. Affects both row paging and filtered counts.

- **Quoting rules**
  - Fields and values can be quoted to preserve spaces. Example: `"user name"="Jane Doe"`.

Limitations versus full SPL (subject to future work): numeric comparisons (`>`, `>=`, ranges), and `IN (...)` are not yet implemented.
