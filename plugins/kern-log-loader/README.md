# Kern.log Loader Plugin

A BreachLine plugin for loading and parsing Linux kernel log files.

## Supported Formats

This plugin parses standard syslog-format kernel logs found in:
- `/var/log/kern.log`
- `/var/log/syslog`
- `/var/log/messages`

### Expected Format

```
Jan 15 10:30:00 hostname kernel: [12345.678901] USB device connected
Jan 15 10:30:01 myhost systemd: Started Session 42 of user root
```

## Output Columns

| Column | Description |
|--------|-------------|
| `timestamp` | Parsed timestamp in ISO 8601 format (YYYY-MM-DD HH:MM:SS) |
| `hostname` | The host that generated the log entry |
| `facility` | Log facility (e.g., kernel, systemd, NetworkManager) |
| `uptime` | Kernel uptime in seconds (from `[12345.678]` format, if present) |
| `message` | The log message content |

## Installation

1. Copy the `kern-log-loader` directory to your BreachLine plugins folder
2. In BreachLine, go to Settings → Plugins → Add Plugin
3. Select the `kern-log-loader` directory

## Usage

Once installed, BreachLine will automatically use this plugin for `.log` files.

> **Note**: This plugin registers the `.log` extension. If you have other plugins that also handle `.log` files, the last loaded plugin will take precedence.

## Testing

Test the plugin from command line:

```bash
# Test header output
./kern-log-loader --mode=header --file=/path/to/kern.log

# Test row count
./kern-log-loader --mode=count --file=/path/to/kern.log

# Test full stream output
./kern-log-loader --mode=stream --file=/path/to/kern.log | head -20
```

## Examples

### Example Input

```
Jan 15 10:30:00 myhost kernel: [12345.678901] USB device connected
Jan 15 10:30:01 myhost kernel: [12345.789012] usb 1-1: New USB device found
Jan 15 10:30:02 myhost NetworkManager[1234]: <info> device (eth0): state change
```

### Example Output

```csv
timestamp,hostname,facility,uptime,message
2024-01-15 10:30:00,myhost,kernel,12345.678901,USB device connected
2024-01-15 10:30:01,myhost,kernel,12345.789012,usb 1-1: New USB device found
2024-01-15 10:30:02,myhost,NetworkManager[1234],,"<info> device (eth0): state change"
```

## Limitations

- **Year**: Kernel logs don't include the year, so the current year is assumed
- **Timezone**: Timestamps are parsed as local time
- **Non-standard formats**: Some custom syslog formats may not parse correctly

## Requirements

- Python 3.6+
- No external dependencies (uses only standard library)

## License

Part of the BreachLine project.
