#!/usr/bin/env python3
"""Generate XLSX test files for timezone testing."""

import argparse
import logging
from openpyxl import Workbook

logging.basicConfig(level=logging.INFO, format='%(levelname)s: %(message)s')
logger = logging.getLogger(__name__)


def create_xlsx_file(filename, data):
    """Create an XLSX file with the given data."""
    wb = Workbook()
    ws = wb.active
    ws.title = "Test Data"
    
    # Write header
    ws.append(["timestamp", "description"])
    
    # Write data rows
    for row in data:
        ws.append(row)
    
    wb.save(filename)
    logger.info(f"Created {filename}")


def main():
    parser = argparse.ArgumentParser(description='Generate XLSX test files for timezone testing')
    parser.add_argument('--output-dir', default='.', help='Output directory for XLSX files')
    args = parser.parse_args()
    
    output_dir = args.output_dir
    
    # Data for timestamps without timezone
    no_tz_data = [
        ("2024-01-15 09:30:00", "No timezone - should use default/override ingest timezone"),
        ("2024-01-15 14:45:30", "No timezone - afternoon time"),
        ("2024-01-15 00:00:00", "No timezone - midnight"),
        ("2024-01-15 23:59:59", "No timezone - end of day"),
        ("2024-06-15 12:00:00", "No timezone - summer date (DST consideration)"),
        ("2024-12-15 12:00:00", "No timezone - winter date (no DST)"),
    ]
    create_xlsx_file(f"{output_dir}/timestamps_no_tz.xlsx", no_tz_data)
    
    # Data for timestamps with UTC
    utc_data = [
        ("2024-01-15T09:30:00Z", "UTC timezone (Z suffix) - 09:30 UTC"),
        ("2024-01-15T14:45:30Z", "UTC timezone (Z suffix) - 14:45 UTC"),
        ("2024-01-15T00:00:00Z", "UTC timezone (Z suffix) - midnight UTC"),
        ("2024-01-15T23:59:59Z", "UTC timezone (Z suffix) - end of day UTC"),
        ("2024-06-15T12:00:00Z", "UTC timezone (Z suffix) - summer noon UTC"),
        ("2024-01-15 09:30:00 UTC", "UTC timezone (UTC suffix) - 09:30 UTC"),
        ("2024-01-15 14:45:30 UTC", "UTC timezone (UTC suffix) - 14:45 UTC"),
    ]
    create_xlsx_file(f"{output_dir}/timestamps_with_utc.xlsx", utc_data)
    
    # Data for timestamps with offsets
    offset_data = [
        ("2024-01-15T09:30:00+00:00", "Offset +00:00 (UTC) - 09:30 UTC"),
        ("2024-01-15T09:30:00+13:00", "Offset +13:00 (NZDT) - 09:30 in NZ = 20:30 previous day UTC"),
        ("2024-01-15T09:30:00+12:00", "Offset +12:00 (NZST) - 09:30 in NZ = 21:30 previous day UTC"),
        ("2024-01-15T09:30:00-05:00", "Offset -05:00 (EST) - 09:30 in EST = 14:30 UTC"),
        ("2024-01-15T09:30:00-08:00", "Offset -08:00 (PST) - 09:30 in PST = 17:30 UTC"),
        ("2024-01-15T09:30:00+05:30", "Offset +05:30 (IST India) - 09:30 in IST = 04:00 UTC"),
        ("2024-01-15T09:30:00+09:00", "Offset +09:00 (JST) - 09:30 in Japan = 00:30 UTC"),
        ("2024-01-15T09:30:00+01:00", "Offset +01:00 (CET) - 09:30 in CET = 08:30 UTC"),
        ("2024-01-15T09:30:00-03:30", "Offset -03:30 (NST) - 09:30 in Newfoundland = 13:00 UTC"),
    ]
    create_xlsx_file(f"{output_dir}/timestamps_with_offsets.xlsx", offset_data)
    
    # Mixed format data
    mixed_data = [
        ("2024-01-15T09:30:00Z", "ISO8601 with Z - 09:30 UTC"),
        ("2024-01-15 09:30:00", "Space separated no TZ - needs default/override"),
        ("2024-01-15T09:30:00+13:00", "ISO8601 with NZ offset - 09:30 NZDT"),
        ("01/15/2024 09:30:00", "US date format no TZ - needs default/override"),
        ("15/01/2024 09:30:00", "EU date format no TZ - needs default/override"),
        ("2024-01-15T09:30:00.123Z", "ISO8601 with milliseconds and Z - 09:30 UTC"),
        ("2024-01-15T09:30:00.123456Z", "ISO8601 with microseconds and Z - 09:30 UTC"),
        ("2024-01-15 09:30:00.123", "With milliseconds no TZ - needs default/override"),
        ("Jan 15 2024 09:30:00", "Month name format no TZ - needs default/override"),
        ("15 Jan 2024 09:30:00", "Day first month name no TZ - needs default/override"),
    ]
    create_xlsx_file(f"{output_dir}/timestamps_mixed_formats.xlsx", mixed_data)
    
    logger.info("All XLSX files generated successfully")


if __name__ == "__main__":
    main()
