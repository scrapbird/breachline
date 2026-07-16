#!/usr/bin/env python3
"""Print the sync-API base URL for an environment, derived from the repo-root
config.yml.

Used at build time to bake the correct endpoint into the desktop app + CLI
binary via `-ldflags -X breachline/app/sync.BaseURL=<url>` (see
application/build.sh and the release workflow). Only the URL is written to
stdout so the value can be captured via command substitution; all logging goes
to stderr.
"""

import argparse
import logging
import sys
from pathlib import Path

try:
    import yaml
except ModuleNotFoundError:
    sys.exit("pyyaml not installed; run `make venv` or `pip install pyyaml`")

REPO = Path(__file__).resolve().parents[1]
CONFIG_FILE = REPO / "config.yml"

logging.basicConfig(level=logging.INFO, format="%(levelname)s %(message)s",
                    stream=sys.stderr)
logger = logging.getLogger(__name__)


def sync_api_endpoint(env: str) -> str:
    """Return https://<api_domain_name>/v1 for env, read from config.yml."""
    if not CONFIG_FILE.exists():
        sys.exit(f"{CONFIG_FILE} not found - run `make init`")
    data = yaml.safe_load(CONFIG_FILE.read_text()) or {}
    cfg = data.get("config") or {}
    envs = cfg.get("environments") or {}
    if env not in envs:
        sys.exit(f"environment '{env}' not found in {CONFIG_FILE}")
    sync = ((envs[env] or {}).get("sync_api")) or {}
    domain = sync.get("api_domain_name")
    if not domain:
        sys.exit(f"config.environments.{env}.sync_api.api_domain_name is empty")
    return f"https://{domain}/v1"


def main() -> None:
    parser = argparse.ArgumentParser(
        description="Print the sync-API base URL for an environment from config.yml.")
    parser.add_argument("env", choices=("dev", "prod"),
                        help="target environment (dev|prod)")
    args = parser.parse_args()
    endpoint = sync_api_endpoint(args.env)
    logger.debug("resolved %s sync endpoint: %s", args.env, endpoint)
    print(endpoint)


if __name__ == "__main__":
    main()
