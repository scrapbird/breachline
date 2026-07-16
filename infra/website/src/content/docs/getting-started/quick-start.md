---
title: "Quick Start"
date: 2025-01-01T00:00:00Z
draft: false
weight: 2
---

This walkthrough takes you from a cold start to your first filtered timeline in about two minutes.

## 1. Open a file

Choose **File → Open** and select a CSV, XLSX, or JSON file. You can also open an entire directory to view many files as a single dataset - handy for CloudTrail backups split across hundreds of JSON files.

## 2. Pick the timestamp field

BreachLine auto-detects the most likely timestamp column. If it guesses wrong, click the column header and choose **Use as timestamp**. Common formats are recognised automatically, including ISO 8601 and epoch values.

## 3. Run a query

Type a query into the search bar and press **Enter**:

```
status_code>=400 AND source_ip="10.0.4.12"
```

Results update instantly and the timeline histogram redraws to match your filter. See the [Query Language](/docs/searching/query-language/) guide for the full syntax.

## 4. Annotate findings

Right-click any row and choose **Add annotation** to attach a note. Annotations travel with the workspace, so you can share them with your team.

## Next steps

- Learn the full [Query Language](/docs/searching/query-language/).
- Understand [Loading Data](/docs/loading-data/loading-data/) and custom JPATH expressions.
- Activate a license to unlock [workspaces and annotations](/docs/workspaces/overview/).
