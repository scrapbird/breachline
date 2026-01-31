# BreachLine

![Icon](./application/build/appicon.png)

# About

BreachLine is a flexible tool for visualizing and analyzing time series data such as audit logs and security events. It is built for speed and ease of use during cyber incident response investigations.

It supports reading time series data from CSV, XLSX and JSON files, supporting custom JPATH expressions to locate and ingest the list data.

# Features

- Loads large files (up to ~5-10 GB), each row being a timestamped event
- Sort and query cache, allowing for fast re-sorting and re-querying of the data
- External sort, using a temporary file to sort the data, allowing for sorting of files larger than available memory
- Displays the events in a fast, responsive, virtualized data grid (only rendering visible rows, etc.)
- Provides filtering, sorting, navigation (seek by time)
- Includes a persistent search bar at the top that uses a filter language similar to Splunk SPL to filter rows in real time
- Renders graphs / histograms showing counts of events in time buckets (e.g. 1 hour, 5 minutes, etc.)
- Cross-platform: builds & runs on Windows, macOS, Linux
- Supports workspaces with saved annotations
- Allows annotated data to be exported to a combined timeline file
- Flexible timezone handling, including default ingest timezone and separate display timezone
- Normalizes timestamps to a configurable time format

# Repository Structure

The repository is structured as follows:

- [application](./application): The main application code
- [doc](./doc): Documentation
- [infra](./infra): Infrastructure terraform templates and supporting code
- [scripts](./scripts): Various helper scripts for generating licenses, test files and automating simple tasks go here

