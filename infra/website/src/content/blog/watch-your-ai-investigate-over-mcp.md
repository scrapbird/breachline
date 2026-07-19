---
title: "Watch your AI investigate: driving BreachLine over MCP"
date: 2026-07-19T00:00:00Z
draft: false
summary: "BreachLine can host an MCP server so an AI agent drives the same window you are looking at. Instead of handing a log file to a model and trusting the summary, you watch every query it runs land in the grid in front of you."
---

The usual way to point an AI at logs is to hand it a file and read what it says back. It tells you there were 291 delete calls and one of them looks bad. Now you have a choice: trust it, or go and redo the work yourself to check. Trusting it is how a hallucinated line number or an invented event ends up in an incident report. Redoing it defeats the point of asking. Either way the model did its work somewhere you could not see, and you are left holding a paragraph you cannot fully verify.

BreachLine takes a different path. It can host an [MCP](https://modelcontextprotocol.io) server that lets an AI client drive the running app: open files, run queries, read results, annotate rows. The important part is where those actions happen. They happen in the visible window, through the exact handlers a person uses. There is one set of tabs, one active query, one grid, and the agent and you are both looking at it. When the agent filters to the delete calls, the delete calls appear on your screen. You are not reading a claim about the data. You are watching the data.

## Turn it on

The server is off by default. Open **Settings > MCP**, tick **Enable MCP server**, and press Save. It binds to `127.0.0.1:8765` on loopback so it is never reachable off your machine, and saving generates a bearer token that every client request must carry.

![The MCP settings panel: enable the server, loopback listen address, generated token, and a ready to paste client command](/blog/images/mcp-settings.png)

The panel hands you a ready to paste command for connecting a client:

```
claude mcp add --transport http breachline http://127.0.0.1:8765/ --header "Authorization: Bearer <token>"
```

That is the whole setup. From here the agent has the same surface you do: it can open data, run the query language, and page through results, and each action it takes is performed by the window in front of you.

## Give it a question, watch it work

Here is a real run against a week of CloudTrail: 1,384 gzipped log objects pulled from S3, four regions, still zipped on disk. The prompt to the agent was plain: something may have been torn down in this account this week, take a look.

Its first move is to open the folder. BreachLine reads the gzip files where they sit, pulls the records out of each `$.Records` array, and merges everything into one time sorted grid. This is not a summary the agent typed up. This is your window, filled in by the agent, showing all 8,227 events with the histogram across the top:

![The full week of CloudTrail loaded into the live window as 8,227 time sorted events](/blog/images/mcp-timeline.png)

There is already a spike near 2026-07-15 that neither of you had to look for. The agent goes after the destructive actions the way you would, with a filter on delete calls:

```
filter eventName=Delete* | columns eventTime, eventName, eventSource, sourceIPAddress, awsRegion
```

The search bar shows the exact query it wrote, the grid narrows to the matches, and the histogram redraws to just the burst:

![291 delete calls, every one from a single source IP, clustered into a few minutes](/blog/images/mcp-delete-burst.png)

291 delete calls, and the shape of it is obvious on screen: `DeleteSecret`, `DeleteFunction`, `DeleteLogGroup`, `DeleteTopic`, `DeleteRestApi`, `DeleteStage`, `DeleteAlarms`, `DeleteDashboards`, all from the same `sourceIPAddress`, all inside a few minutes. Secrets gone, log groups gone, alarms and dashboards gone. That is exactly the sequence you dread reading in an agent's summary, because a wall of deletes plus torn down logging is what an intruder covering their tracks looks like.

But you are not reading a summary. You can pivot in the same grid the agent is driving. Filter to that one IP and put the user agent on screen:

```
filter eventName=Delete* AND sourceIPAddress=203.211.105.190 | columns eventTime, eventName, userAgent
```

![Every delete carries a Terraform user agent, resolving the burst as a terraform destroy](/blog/images/mcp-terraform-useragent.png)

Every one of the 291 deletes carries `Terraform/1.14.2` and the AWS provider. This was not an intruder. It was a `terraform destroy` tearing down an environment, and expanding any row shows it ran under the account's own `AdministratorAccess` SSO role. The scary looking teardown resolves to a routine piece of infrastructure work in about the time it takes to type one more filter.

That is the difference. An agent handed the log file might have reported "291 destructive calls including deletion of secrets and logging" and left you to decide whether to page someone at midnight. Here the agent surfaced the same finding, but the resolution happened in your grid, on rows you can see, with a query you can read and rerun. You did not trust it and you did not redo it. You watched it, and you closed it out.

## Why driving the real UI matters

The design choice underneath this is that the server never owns UI state. Reads are answered straight from the backend, but anything that changes the window is dispatched to the frontend and performed through the same handlers a human click would trigger. So the tab the agent opens and the tab you see are the same tab. There is no separate "agent view" that can drift from what is on your screen, and nothing to reconcile afterward.

That has a few consequences that add up during an investigation:

- Every step is visible while it happens. You see the query, the row count, the histogram move. If the agent goes down a wrong path you catch it on the spot, not three paragraphs later.
- The work is yours to keep. When the agent lands on something, the grid is already filtered to it. You take over by hand, annotate the rows, keep pivoting. There is no export and reimport.
- Verification is free. The thing you would do to check the agent, run the query yourself, already happened on your screen. Reproducing a finding is scrolling back up, not starting over.

It cuts the slow part of an investigation, the part where you are pulling files apart and merging them, while removing the part you would normally have to spend re-earning trust in the output. The agent moves at machine speed across a week of compressed logs, and you keep the human judgement, because you can see everything it touched.

## A note on trust and access

Enabling the server hands a local AI client real capability: it can read the files you open and act in the app. That is worth being deliberate about.

- It binds to loopback by default, so it is not reachable from other machines.
- Every request needs the bearer token, checked in constant time. No token, 401.
- It is off until you explicitly enable it, and the setting is visible.
- The workspace and annotation tools additionally require a valid license.

Turn it on when you intend to use it, point your agent at the logs, and watch it work. The output is not a paragraph you have to believe. It is a window you were looking at the whole time.
