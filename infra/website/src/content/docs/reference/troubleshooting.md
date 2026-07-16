---
title: "Troubleshooting"
date: 2025-01-01T00:00:00Z
draft: false
weight: 2
---

Fixes for the issues that come up most often. If your problem is not here, the console panel (**View → Toggle Console**) often shows a message explaining what went wrong.

## Timestamps look wrong or are out of order

- BreachLine may have picked the wrong timestamp column. Click the correct column header and choose **Use as timestamp**.
- If zone-less timestamps are shifted, check your **ingest timezone**; if the displayed times are shifted, check your **display timezone**. Both are covered in [Timestamps & Timezones](/docs/loading-data/timestamps-timezones/).
- Rows with timestamps BreachLine cannot parse are grouped and sorted after the parseable rows rather than dropped.

## My license will not import

- Confirm you selected the license file itself, exactly as it was emailed to you.
- Check the expiry date under **Help → About**; an expired license does not unlock the licensed features. [Renew](/docs/getting-started/licensing/) and import the new file.
- Make sure you imported the file through **File → Import License**.

## A JSON file opens empty or with the wrong columns

Nested JSON needs a JPATH expression pointing at the array of records, for example `$.Records`. See [Loading Data](/docs/loading-data/loading-data/).

## A plugin does not run

- The `plugin.yml` manifest must sit in the same directory as the executable and be valid YAML.
- On macOS and Linux the executable needs execute permission (`chmod +x`).
- If two plugins claim the same extension, the most recently loaded one wins; disable one to resolve it.
- See the [Plugin Developer Guide](/docs/extending/plugin-developer-guide/) for the full contract.

## The app is slow or runs out of memory on a huge file

BreachLine loads files fully into memory and caches sorted and filtered results. On a machine that is tight on RAM, turn off the query cache under **File → Settings**, and see [System Requirements](/docs/getting-started/system-requirements/) and the [Cache System](/docs/configuration/cache-system/) for sizing guidance.

## Sync is not updating across machines

- Make sure you are [signed in](/docs/workspaces/sync-account-auth/) on both machines with a valid license.
- Remember that only workspace metadata and annotations sync, not your data files. Each machine needs its own local access to the underlying files.
