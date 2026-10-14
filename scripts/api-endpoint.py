#!/usr/bin/env python3
"""Print the sync-API base URL for an environment, derived from the repo-root
config.yml.

If the environment sets sync_api.api_domain_name, the URL is
https://<api_domain_name>/v1. If it is empty (dev, deliberately: no public DNS
name for people to find and probe), the raw execute-api URL of the deployed
breachline-sync-api-<env> REST API is looked up from AWS instead, which needs
credentials for the breachline account.

Used at build time to bake the correct endpoint into the desktop app + CLI
binary via `-ldflags -X breachline/app/sync.BaseURL=<url>` (see
application/build.sh and the release workflow). Only the URL is written to
stdout so the value can be captured via command substitution; all logging goes
to stderr.
"""

import argparse
import json
import logging
import subprocess
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


def raw_api_endpoint(env: str, region: str, profile: str) -> str:
    """Return the execute-api URL of the deployed sync API's v1 stage."""
    name = f"breachline-sync-api-{env}"
    logger.info("no api_domain_name for %s; looking up REST API %s (profile %s)",
                env, name, profile)
    proc = subprocess.run(
        ["aws", "apigateway", "get-rest-apis", "--region", region,
         "--profile", profile, "--query", f"items[?name=='{name}'].id",
         "--output", "json"],
        capture_output=True, text=True, check=False,
    )
    if proc.returncode != 0:
        sys.exit(f"aws apigateway get-rest-apis failed:\n{proc.stderr.strip()}")
    ids = json.loads(proc.stdout or "[]")
    if len(ids) != 1:
        sys.exit(f"expected exactly one REST API named {name} in {region}, found {len(ids)}"
                 " - is the sync-api deployed?")
    return f"https://{ids[0]}.execute-api.{region}.amazonaws.com/v1"


def sync_api_endpoint(env: str, profile: str) -> str:
    """Return the sync-API base URL for env, from config.yml (or AWS if the
    environment has no api_domain_name)."""
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
        region = (cfg.get("aws") or {}).get("region", "us-east-2")
        return raw_api_endpoint(env, region, profile)
    return f"https://{domain}/v1"


def main() -> None:
    parser = argparse.ArgumentParser(
        description="Print the sync-API base URL for an environment from config.yml.")
    parser.add_argument("env", choices=("dev", "prod"),
                        help="target environment (dev|prod)")
    parser.add_argument("--profile", default="breachline",
                        help="AWS profile for the raw-URL lookup (default: breachline)")
    args = parser.parse_args()
    endpoint = sync_api_endpoint(args.env, args.profile)
    logger.debug("resolved %s sync endpoint: %s", args.env, endpoint)
    print(endpoint)


if __name__ == "__main__":
    main()
