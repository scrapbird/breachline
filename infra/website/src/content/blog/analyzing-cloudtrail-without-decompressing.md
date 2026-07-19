---
title: "Reading a week of CloudTrail without unzipping a single file"
date: 2026-07-19T00:00:00Z
draft: false
summary: "CloudTrail lands in S3 as hundreds of tiny gzip files split across regions and days. Point BreachLine at the folder and it becomes one searchable timeline, no gunzip and no jq required."
---

If you have ever pulled CloudTrail down from S3 to chase something, you know the shape of it. The trail writes one object every few minutes, per region, gzipped, into a date partition. A week of a quiet account is already more than a thousand files:

```
AWSLogs/164723697561/CloudTrail/ap-southeast-2/2026/07/16/164723697561_CloudTrail_ap-southeast-2_20260716T1405Z_X2vAPzYyNcAL3lbX.json.gz
AWSLogs/164723697561/CloudTrail/us-east-1/2026/07/16/...
AWSLogs/164723697561/CloudTrail/us-west-2/2026/07/16/...
```

Each file is a JSON object with a single `Records` array inside it. Nothing you can grep, nothing sorted, and the events you care about are spread across four regions and eight days of folders. The usual next move is a small pile of shell: `aws s3 sync`, then `gunzip` everything, then `jq '.Records[]'` to unwrap each file, then concatenate, then sort by `eventTime`. Every step writes a temp file, and you throw all of it away when the investigation closes.

BreachLine skips that entirely. It reads the gzip files where they sit, pulls the records out, and merges every file into one time-ordered grid. This walks through doing exactly that with the last seven days from a real account.

## Get the logs onto disk

The only prep is copying the objects down. Keep the folder structure, it does not matter how deep it is:

```
aws s3 cp \
  s3://cloudtrail-logs-94403pzrvsqdgdi1a3m3/AWSLogs/164723697561/CloudTrail/ \
  ~/investigations/scrappylabs-cloudtrail/AWSLogs/164723697561/CloudTrail/ \
  --recursive --exclude "*" --include "*/2026/07/1[2-9]/*"
```

That leaves 1,384 gzip files on disk, spread across `ap-southeast-2`, `us-east-1`, `us-west-2` and `eu-central-1`. They stay gzipped. You never extract them.

## Point BreachLine at the folder

Open BreachLine and choose **File > Open File with Options**, then pick **Directory** and select the top `scrappylabs-cloudtrail` folder. BreachLine asks how to read what it finds:

- **JSONPath Expression**: `$.Records`, because that is where each file keeps its events.
- **File Pattern**: `**/*.json.gz`. The `**` walks every region and date folder underneath, and the `.gz` is read straight through. BreachLine decompresses each file in memory as it opens it.
- **Include source file column**: tick this. It adds a `__source_file__` column so every row remembers which object it came from.

Apply that and the whole week lands in one grid, already sorted by time.

![The full CloudTrail directory loaded as a single 8,227 row timeline](/blog/images/cloudtrail-directory-loaded.png)

That is 1,384 separate gzip files, four regions, read and merged into 8,227 events on one timeline. The histogram across the top is every management event in the account bucketed over time, so a spike is obvious before you have typed anything. No files were extracted and nothing was written back to disk.

## Filter it like a log, not a pile of JSON

The search bar runs a small pipeline language. A stage starts with a verb, and stages chain with `|`. The first thing worth doing on any CloudTrail set is looking at what failed, so start with a `filter` on the presence of an error code and trim the grid down to the columns that matter:

```
filter errorCode=* | columns eventTime, eventName, eventSource, errorCode, sourceIPAddress, awsRegion
```

![175 failed API calls filtered from the week](/blog/images/cloudtrail-failed-calls.png)

175 rows out of 8,227, across every region, in about the time it takes to press Enter. The `AuthorizationPendingException` bursts on `sso.amazonaws.com` are a device login being polled, the `ResourceNotFoundException` runs against `lambda` are something reading functions that are not there. The histogram redraws to show only the failures, so you can see when they clustered.

## See who signed in

Console logins are their own event, so filter to them and keep the source file column visible:

```
filter eventName=ConsoleLogin | columns eventTime, eventName, sourceIPAddress, awsRegion, __source_file__
```

![Four console logins with the source file each came from](/blog/images/cloudtrail-console-logins.png)

Four logins for the week, all from the same address. The `__source_file__` column is the point here: each of these four rows was pulled from a different gzip object, and BreachLine interleaved them into one timeline by their event time. You are reading across files without ever having joined them yourself.

## Find what actually changed

Read-only calls are noise when you are asking what happened to an account. CloudTrail tags every event with `readOnly`, so drop the reads:

```
filter readOnly=false | columns eventTime, eventName, eventSource, sourceIPAddress, awsRegion
```

![576 mutating events isolated from the read-only noise](/blog/images/cloudtrail-write-events.png)

576 mutating events out of the 8,227. `RunInstances` and `TerminateInstances`, `CreateLogGroup`, `RegisterManagedInstance`, the sign-in sequence around each console session. This is the list you actually walk during a review, and it came out of the same open folder with one more filter.

## That is the whole workflow

No `gunzip`, no `jq`, no concatenation step, no merged file sitting in `/tmp` that you have to remember to delete. You copied the objects down, opened the folder, and told BreachLine the records live at `$.Records`. Everything after that was a filter.

The same approach works for anything CloudTrail-shaped: partitioned, compressed, split across many small files. VPC flow logs, ALB access logs, any exporter that drops gzipped JSON into date folders. Point BreachLine at the top of the tree and let it do the decompressing and the merging.
