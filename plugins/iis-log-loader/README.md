# IIS Log Loader Plugin

A BreachLine plugin for loading Microsoft IIS (Internet Information Services) web server log files.

## Supported Formats

This plugin automatically detects and parses three IIS log formats:

### 1. W3C Extended Log Format (Most Common)

The default format for IIS logging with customizable fields. The plugin reads the `#Fields:` directive to determine column names.

**Example:**
```
#Software: Microsoft Internet Information Services 10.0
#Version: 1.0
#Date: 2024-01-15 10:30:00
#Fields: date time s-ip cs-method cs-uri-stem cs-uri-query s-port cs-username c-ip cs(User-Agent) sc-status sc-substatus sc-win32-status time-taken
2024-01-15 10:30:45 192.168.1.100 GET /api/users - 443 - 10.0.0.50 Mozilla/5.0 200 0 0 125
```

**Output Columns:** Dynamic based on `#Fields:` directive. The `date` and `time` fields are combined into a single `timestamp` column.

### 2. IIS Native Log Format

A fixed comma-separated format with predefined fields.

**Example:**
```
192.168.114.201, -, 03/20/24, 7:55:20, W3SVC2, SALES1, 172.21.13.45, 4502, 163, 3223, 200, 0, GET, /DeptLogo.gif, -
```

**Output Columns:**
- `timestamp` - Combined date and time
- `client_ip` - Client IP address
- `username` - Authenticated username
- `service` - IIS service name (e.g., W3SVC2)
- `server` - Server name
- `server_ip` - Server IP address
- `time_taken` - Request processing time (ms)
- `bytes_received` - Bytes received from client
- `bytes_sent` - Bytes sent to client
- `status_code` - HTTP status code
- `win32_status` - Windows status code
- `method` - HTTP method (GET, POST, etc.)
- `uri` - Request URI
- `parameters` - Request parameters

### 3. NCSA Common Log Format

A fixed space-separated format used by many web servers.

**Example:**
```
172.21.13.45 - john [08/Apr/2024:17:39:04 -0800] "GET /scripts/iisadmin/ism.dll HTTP/1.0" 200 3401
```

**Output Columns:**
- `timestamp` - Date and time with timezone
- `client_ip` - Client IP address
- `ident` - Remote logname (usually empty)
- `username` - Authenticated username
- `method` - HTTP method
- `uri` - Request URI
- `protocol` - HTTP protocol version
- `status_code` - HTTP status code
- `bytes_sent` - Bytes sent to client

## Timezone Handling

Each format handles timezones differently:

| Format | Timezone in File | BreachLine Behavior |
|--------|------------------|---------------------|
| **W3C Extended** | Always UTC | Timestamps output with `Z` suffix (UTC) |
| **IIS Native** | Local server time | No timezone in output; set BreachLine's `Default Ingest Timezone` to match your server |
| **NCSA Common** | Local with offset (e.g., `-0800`) | Timezone offset preserved in ISO 8601 format |

### Configuration Tips

- **W3C Extended logs**: No configuration needed. Timestamps are UTC.
- **IIS Native logs**: Set BreachLine's `Default Ingest Timezone` in Settings to match your server's timezone for accurate time display.
- **NCSA Common logs**: No configuration needed. The timezone offset is preserved and parsed correctly.

## Installation

1. Copy the `iis-log-loader` directory to your BreachLine plugins folder
2. Open BreachLine and go to Settings
3. Enable plugins if not already enabled
4. Click "Add Plugin" and select the `iis-log-loader` directory
5. The plugin will be enabled and ready to use

## File Extensions

The plugin registers for the following extensions:
- `.log` - Generic log files
- `.iis` - IIS-specific log files
- `.w3c` - W3C Extended format files

**Note:** If you have multiple plugins that handle `.log` files (e.g., `kern-log-loader`), BreachLine will let you choose which plugin to use when opening a file.

## Auto-Detection

The plugin automatically detects the log format by examining the file content:

1. **W3C Extended**: Files starting with `#Software:`, `#Version:`, or `#Fields:` directives
2. **IIS Native**: Comma-separated lines with 14+ fields
3. **NCSA Common**: Lines matching `IP - user [timestamp] "request" status bytes` pattern

If the format cannot be determined, the plugin defaults to W3C Extended with standard fields.

## Testing

Test the plugin from the command line:

```bash
# Test header mode
./iis-log-loader --mode=header --file=samples/w3c_extended.log

# Test count mode
./iis-log-loader --mode=count --file=samples/w3c_extended.log

# Test stream mode (first 10 rows)
./iis-log-loader --mode=stream --file=samples/w3c_extended.log | head -10
```

## Troubleshooting

### Wrong columns displayed
- Check if your log file has a `#Fields:` directive
- If missing, the plugin uses default W3C fields which may not match your data

### Timestamps show wrong time
- For IIS Native format, ensure your `Default Ingest Timezone` setting matches your server's timezone
- W3C Extended logs are always UTC

### Plugin not loading
- Ensure the `iis-log-loader` file has execute permissions: `chmod +x iis-log-loader`
- Check that Python 3 is installed and accessible

## Dependencies

- Python 3.6 or later
- No external packages required (uses only standard library)
