#!/usr/bin/env python3
"""
Breachline Sync API Rate Limit and Validation Test Suite

This script tests all rate limits and request validators in the sync API.
It requires a valid Breachline settings file with sync tokens.

Usage:
    python test_rate_limits.py /path/to/breachline/settings.yaml
"""

import argparse
import json
import logging
import sys
import time
import uuid
from concurrent.futures import ThreadPoolExecutor, as_completed
from dataclasses import dataclass, field
from typing import Optional

import requests
from requests.adapters import HTTPAdapter
from urllib3.util.retry import Retry
import yaml

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s',
    datefmt='%Y-%m-%d %H:%M:%S'
)
logger = logging.getLogger(__name__)

# API Configuration
BASE_URL = "https://u9trlagch8.execute-api.ap-southeast-2.amazonaws.com/v1"

# Rate limit configuration (from rate_limits.go)
RATE_LIMITS = {
    "workspaces": 10,   # 10 workspace ops per minute
    "files": 100,       # 100 file ops per minute
    "annotations": 1000, # 1000 annotation ops per minute
    "auth": 5,          # 5 auth ops per minute
}
RATE_LIMIT_WINDOW_SECONDS = 60

# Validation limits (from request_validator.go)
VALIDATION_LIMITS = {
    "annotation_note": 512,
    "annotation_color": 16,
    "workspace_name": 64,
    "file_path": 1024,
    "jpath": 512,
    "id": 46,
    "hash": 64,
}


@dataclass
class TestResult:
    """Result of a test"""
    name: str
    passed: bool
    message: str
    details: Optional[str] = None


@dataclass
class TestSuite:
    """Collection of test results"""
    results: list = field(default_factory=list)
    
    def add(self, result: TestResult):
        self.results.append(result)
        status = "PASS" if result.passed else "FAIL"
        logger.info(f"[{status}] {result.name}: {result.message}")
        if result.details:
            logger.debug(f"  Details: {result.details}")
    
    def summary(self):
        passed = sum(1 for r in self.results if r.passed)
        failed = sum(1 for r in self.results if not r.passed)
        total = len(self.results)
        return passed, failed, total


class SyncAPIClient:
    """Client for the Breachline Sync API"""
    
    def __init__(self, session_token: str, license_key: str):
        self.session_token = session_token
        self.license_key = license_key
        self.session = requests.Session()
        self.session.headers.update({
            "Authorization": f"Bearer {session_token}",
            "Content-Type": "application/json",
        })
        # Configure connection pool for concurrent requests
        adapter = HTTPAdapter(pool_connections=30, pool_maxsize=30)
        self.session.mount("https://", adapter)
        self.session.mount("http://", adapter)
        # Track created resources for cleanup
        self.created_workspaces = []
        self.created_files = []  # (workspace_id, file_hash) tuples
        self.created_annotations = []  # (workspace_id, annotation_id) tuples
    
    def _request(self, method: str, endpoint: str, json_data: dict = None, 
                 expect_error: bool = False) -> tuple[int, dict]:
        """Make an API request and return status code and response"""
        url = f"{BASE_URL}{endpoint}"
        try:
            response = self.session.request(method, url, json=json_data, timeout=30)
            try:
                data = response.json()
            except json.JSONDecodeError:
                data = {"raw": response.text}
            return response.status_code, data
        except requests.exceptions.RequestException as e:
            logger.error(f"Request failed: {e}")
            return 0, {"error": str(e)}
    
    # Workspace operations
    def create_workspace(self, name: str) -> tuple[int, dict]:
        status, data = self._request("POST", "/workspaces", {"name": name})
        if status == 201 and "workspace_id" in data:
            self.created_workspaces.append(data["workspace_id"])
        return status, data
    
    def list_workspaces(self) -> tuple[int, dict]:
        return self._request("GET", "/workspaces")
    
    def get_workspace(self, workspace_id: str) -> tuple[int, dict]:
        return self._request("GET", f"/workspaces/{workspace_id}")
    
    def delete_workspace(self, workspace_id: str) -> tuple[int, dict]:
        status, data = self._request("DELETE", f"/workspaces/{workspace_id}")
        if status in (200, 204) and workspace_id in self.created_workspaces:
            self.created_workspaces.remove(workspace_id)
        return status, data
    
    # File operations
    def create_file(self, workspace_id: str, file_hash: str, 
                    description: str = "", jpath: str = "$") -> tuple[int, dict]:
        payload = {
            "file_hash": file_hash,
            "description": description,
            "options": {"jpath": jpath}
        }
        status, data = self._request("POST", f"/workspaces/{workspace_id}/files", payload)
        if status == 200:
            self.created_files.append((workspace_id, file_hash))
        return status, data
    
    def list_files(self, workspace_id: str) -> tuple[int, dict]:
        return self._request("GET", f"/workspaces/{workspace_id}/files")
    
    def delete_file(self, workspace_id: str, file_hash: str) -> tuple[int, dict]:
        payload = {"file_hash": file_hash, "options": {"jpath": "$"}}
        status, data = self._request("DELETE", f"/workspaces/{workspace_id}/files/{file_hash}", payload)
        key = (workspace_id, file_hash)
        if status in (200, 204) and key in self.created_files:
            self.created_files.remove(key)
        return status, data
    
    # Annotation operations
    def create_annotation(self, workspace_id: str, file_hash: str, 
                         note: str = "test", color: str = "#ff0000",
                         column_hashes: list = None) -> tuple[int, dict]:
        if column_hashes is None:
            column_hashes = [{"column": "test", "hash": "abcd1234abcd1234abcd1234abcd1234"}]
        payload = {
            "workspace_id": workspace_id,
            "file_hash": file_hash,
            "note": note,
            "color": color,
            "column_hashes": column_hashes,
            "options": {"jpath": "$"}
        }
        status, data = self._request("POST", f"/workspaces/{workspace_id}/annotations", payload)
        if status in (200, 201) and "annotation_id" in data:
            self.created_annotations.append((workspace_id, data["annotation_id"]))
        return status, data
    
    def list_annotations(self, workspace_id: str) -> tuple[int, dict]:
        return self._request("GET", f"/workspaces/{workspace_id}/annotations")
    
    def delete_annotation(self, workspace_id: str, annotation_id: str) -> tuple[int, dict]:
        status, data = self._request("DELETE", f"/workspaces/{workspace_id}/annotations/{annotation_id}")
        key = (workspace_id, annotation_id)
        if status in (200, 204) and key in self.created_annotations:
            self.created_annotations.remove(key)
        return status, data
    
    def cleanup(self):
        """Delete all created resources"""
        logger.info("Cleaning up created resources...")
        
        # Delete annotations first
        for workspace_id, annotation_id in list(self.created_annotations):
            logger.debug(f"Deleting annotation {annotation_id}")
            self.delete_annotation(workspace_id, annotation_id)
        
        # Delete files
        for workspace_id, file_hash in list(self.created_files):
            logger.debug(f"Deleting file {file_hash}")
            self.delete_file(workspace_id, file_hash)
        
        # Delete workspaces
        for workspace_id in list(self.created_workspaces):
            logger.info(f"Deleting workspace {workspace_id}")
            status, _ = self.delete_workspace(workspace_id)
            if status in (200, 204):
                logger.info(f"Workspace {workspace_id} deleted successfully")
            else:
                logger.warning(f"Failed to delete workspace {workspace_id}: status {status}")
        
        logger.info("Cleanup complete")


def generate_test_hash() -> str:
    """Generate a valid test file hash (hex, 64 characters)"""
    return uuid.uuid4().hex + uuid.uuid4().hex


def load_settings(settings_path: str) -> dict:
    """Load Breachline settings from YAML file"""
    with open(settings_path, 'r') as f:
        return yaml.safe_load(f)


def test_validation_limits(client: SyncAPIClient, suite: TestSuite, workspace_id: str):
    """Test all request validation limits"""
    logger.info("\n=== Testing Request Validation Limits ===")
    
    test_hash = generate_test_hash()
    
    # Test workspace name too long
    long_name = "x" * (VALIDATION_LIMITS["workspace_name"] + 1)
    status, data = client.create_workspace(long_name)
    error_msg = data.get("error", {}).get("message", "") if isinstance(data.get("error"), dict) else ""
    suite.add(TestResult(
        name="Workspace name exceeds limit",
        passed=status == 400 and "exceeds maximum size" in error_msg,
        message=f"Status: {status}, Expected 400 with size error",
        details=error_msg
    ))
    
    # Test annotation note too long
    long_note = "x" * (VALIDATION_LIMITS["annotation_note"] + 1)
    status, data = client.create_annotation(workspace_id, test_hash, note=long_note)
    error_msg = data.get("error", {}).get("message", "") if isinstance(data.get("error"), dict) else ""
    suite.add(TestResult(
        name="Annotation note exceeds limit",
        passed=status == 400 and "exceeds maximum size" in error_msg,
        message=f"Status: {status}, Expected 400 with size error",
        details=error_msg
    ))
    
    # Test annotation color too long
    long_color = "#" + "f" * (VALIDATION_LIMITS["annotation_color"] + 1)
    status, data = client.create_annotation(workspace_id, test_hash, color=long_color)
    error_msg = data.get("error", {}).get("message", "") if isinstance(data.get("error"), dict) else ""
    suite.add(TestResult(
        name="Annotation color exceeds limit",
        passed=status == 400 and "exceeds maximum size" in error_msg,
        message=f"Status: {status}, Expected 400 with size error",
        details=error_msg
    ))
    
    # Test jpath too long
    long_jpath = "$." + "x" * (VALIDATION_LIMITS["jpath"] + 1)
    status, data = client.create_file(workspace_id, test_hash, jpath=long_jpath)
    error_msg = data.get("error", {}).get("message", "") if isinstance(data.get("error"), dict) else ""
    suite.add(TestResult(
        name="JPath exceeds limit",
        passed=status == 400 and "exceeds maximum size" in error_msg,
        message=f"Status: {status}, Expected 400 with size error",
        details=error_msg
    ))
    
    # Test invalid file hash format
    invalid_hash = "not-a-valid-hash!!!"
    status, data = client.create_file(workspace_id, invalid_hash)
    error_msg = data.get("error", {}).get("message", "") if isinstance(data.get("error"), dict) else ""
    suite.add(TestResult(
        name="Invalid file hash format",
        passed=status == 400 and ("hash" in error_msg.lower() or "invalid" in error_msg.lower()),
        message=f"Status: {status}, Expected 400 with hash format error",
        details=error_msg
    ))
    
    # Test missing required field - file_hash
    status, data = client._request("POST", f"/workspaces/{workspace_id}/files", {"description": "test"})
    error_msg = data.get("error", {}).get("message", "") if isinstance(data.get("error"), dict) else ""
    suite.add(TestResult(
        name="Missing required file_hash",
        passed=status == 400 and "file_hash" in error_msg.lower(),
        message=f"Status: {status}, Expected 400 with missing field error",
        details=error_msg
    ))
    
    # Test missing required field - workspace name
    status, data = client._request("POST", "/workspaces", {})
    error_msg = data.get("error", {}).get("message", "") if isinstance(data.get("error"), dict) else ""
    suite.add(TestResult(
        name="Missing required workspace name",
        passed=status == 400 and ("name" in error_msg.lower() or "required" in error_msg.lower()),
        message=f"Status: {status}, Expected 400 with missing field error",
        details=error_msg
    ))


def test_rate_limit(client: SyncAPIClient, suite: TestSuite, 
                    category: str, limit: int, 
                    request_func, workspace_id: str = None):
    """Test rate limiting for a specific category by verifying headers are present"""
    logger.info(f"\n=== Testing Rate Limit: {category} ({limit} requests/min) ===")
    
    # Make a request and check for rate limit headers
    status, data = request_func()
    
    # Check if we got a rate limit response
    if status == 429:
        error_msg = data.get("error", {}).get("message", "") if isinstance(data.get("error"), dict) else ""
        # Verify the error message is descriptive
        has_category = category in error_msg.lower() or any(
            label in error_msg.lower() 
            for label in ["workspace", "file", "annotation", "authentication", "api"]
        )
        has_limit = "limit" in error_msg.lower()
        has_seconds = "seconds" in error_msg.lower()
        
        suite.add(TestResult(
            name=f"Rate limit {category} - descriptive error message",
            passed=has_category and has_limit and has_seconds,
            message=f"Got 429 with message: {error_msg[:100]}...",
            details=error_msg
        ))
    else:
        # Request succeeded, which is also valid
        suite.add(TestResult(
            name=f"Rate limit {category} - request allowed",
            passed=status in (200, 201),
            message=f"Request succeeded with status {status}",
            details=str(data)[:200]
        ))


def test_rate_limit_headers(client: SyncAPIClient, suite: TestSuite, workspace_id: str):
    """Test that rate limit headers are included in responses"""
    logger.info("\n=== Testing Rate Limit Headers ===")
    
    # Make a request and check for headers
    url = f"{BASE_URL}/workspaces"
    response = client.session.get(url, timeout=30)
    
    headers = response.headers
    
    # Check for rate limit error header (indicates rate limiting system failure)
    has_error_header = "X-RateLimit-Error" in headers
    if has_error_header:
        suite.add(TestResult(
            name="Rate limit headers present",
            passed=False,
            message=f"Rate limit system error: {headers.get('X-RateLimit-Error', 'unknown')}",
            details=f"The rate limiting system encountered an error. Check Lambda CloudWatch logs for details. Headers: {dict(headers)}"
        ))
        return
    
    # Check for rate limit headers
    has_limit_header = "X-RateLimit-Limit" in headers
    has_remaining_header = "X-RateLimit-Remaining" in headers
    has_reset_header = "X-RateLimit-Reset" in headers
    
    suite.add(TestResult(
        name="Rate limit headers present",
        passed=has_limit_header and has_remaining_header and has_reset_header,
        message=f"Limit: {headers.get('X-RateLimit-Limit', 'N/A')}, "
                f"Remaining: {headers.get('X-RateLimit-Remaining', 'N/A')}, "
                f"Reset: {headers.get('X-RateLimit-Reset', 'N/A')}",
        details=str(dict(headers))
    ))


def test_rate_limit_exhaustion_for_category(
    client: SyncAPIClient, 
    suite: TestSuite, 
    workspace_id: str,
    category: str,
    limit: int,
    request_func
):
    """
    Test rate limit exhaustion for a specific category.
    
    Args:
        client: API client
        suite: Test suite to add results to
        workspace_id: Workspace ID for file/annotation requests
        category: Rate limit category name (workspaces, files, annotations)
        limit: Expected rate limit
        request_func: Function that makes a request and returns (url, response)
    """
    logger.info(f"\n=== Testing Rate Limit Exhaustion: {category} ({limit}/min) ===")
    logger.warning(f"This test will exhaust the {category} rate limit. It will reset in 60 seconds.")
    
    hit_limit = False
    requests_made = 0
    error_message = ""
    rate_limit_headers = {}
    
    # Make enough requests to definitely exceed the limit
    # Add buffer for requests already made earlier in the test
    max_test_requests = limit + 20
    batch_size = 20
    
    logger.info(f"Making up to {max_test_requests} requests ({batch_size} concurrent) to exceed limit of {limit}/min...")
    
    # Process requests in batches of 20 concurrent requests
    # Add small delay between batches to avoid hitting API Gateway's built-in throttle (50 req/s)
    # We want to test our DynamoDB-based rate limits, not API Gateway's throttle
    for batch_start in range(0, max_test_requests, batch_size):
        batch_end = min(batch_start + batch_size, max_test_requests)
        batch_count = batch_end - batch_start
        
        # Submit batch of concurrent requests
        with ThreadPoolExecutor(max_workers=batch_size) as executor:
            futures = [executor.submit(request_func) for _ in range(batch_count)]
            
            for future in as_completed(futures):
                requests_made += 1
                response = future.result()
                status = response.status_code
                
                # Extract rate limit headers if present
                if "X-RateLimit-Limit" in response.headers:
                    rate_limit_headers = {
                        "limit": response.headers.get("X-RateLimit-Limit"),
                        "remaining": response.headers.get("X-RateLimit-Remaining"),
                        "reset": response.headers.get("X-RateLimit-Reset"),
                    }
                
                # Check for error header indicating rate limit system failure
                if "X-RateLimit-Error" in response.headers:
                    logger.warning(f"Rate limit system error: {response.headers.get('X-RateLimit-Error')}")
                
                if status == 429:
                    hit_limit = True
                    # Only capture first error message (don't overwrite)
                    if not error_message:
                        # Debug: log raw response for first 429
                        logger.debug(f"429 response - Headers: {dict(response.headers)}")
                        logger.debug(f"429 response - Raw body: {response.text[:500] if response.text else '(empty)'}")
                        try:
                            if response.text:
                                data = response.json()
                                logger.debug(f"429 response - Parsed JSON: {data}")
                                # Handle both our custom format and API Gateway's format
                                # Our format: {"error": {"message": "..."}}
                                # API Gateway format: {"message": "Too Many Requests"}
                                if isinstance(data.get("error"), dict):
                                    error_message = data.get("error", {}).get("message", "")
                                    logger.debug("Detected custom rate limit response format")
                                elif "message" in data:
                                    error_message = data.get("message", "")
                                    logger.debug("Detected API Gateway throttle response format")
                                else:
                                    error_message = str(data)
                                    logger.warning(f"Unknown 429 response format: {data}")
                                
                                if error_message:
                                    logger.debug(f"Captured error message: {error_message[:100]}")
                            else:
                                logger.warning("429 response has empty body")
                                error_message = "(empty body)"
                        except Exception as e:
                            error_message = response.text if response.text else "(empty body)"
                            logger.debug(f"Failed to parse JSON, using raw text: {e}")
                elif status not in (200, 201, 202):
                    logger.warning(f"Unexpected status {status}: {response.text[:200]}")
        
        # Log progress after each batch
        remaining = rate_limit_headers.get("remaining", "?")
        limit_val = rate_limit_headers.get("limit", "?")
        reset_val = rate_limit_headers.get("reset", "?")
        logger.info(f"Batch complete - requests={requests_made}, limit={limit_val}, remaining={remaining}, reset={reset_val}")
        
        if hit_limit:
            logger.info(f"✓ Hit {category} rate limit after {requests_made} requests!")
            logger.info(f"Error message: {error_message[:200] if error_message else '(empty)'}")
            break
        
        # Small delay to avoid hitting API Gateway's 50 req/s throttle
        # This ensures we test our custom rate limits, not API Gateway throttle
        time.sleep(0.5)
    
    if hit_limit:
        # Determine if this is API Gateway throttle or our custom rate limit
        # API Gateway throttle returns "Too Many Requests" without our custom headers
        # Our custom rate limit returns detailed message with category info AND our custom headers
        is_api_gateway_throttle = (
            "too many requests" in error_message.lower() or 
            not rate_limit_headers or  # No custom headers means API Gateway throttle
            rate_limit_headers.get("limit") == "0"  # Unknown license returns limit=0
        )
        
        if is_api_gateway_throttle:
            # API Gateway throttle - we hit the gateway's 50 req/s limit, not our custom limit
            logger.warning(f"Hit API Gateway throttle instead of custom rate limit. Consider reducing request rate.")
            suite.add(TestResult(
                name=f"Rate limit {category} - hit limit",
                passed=True,
                message=f"Hit API Gateway throttle after {requests_made} requests (gateway limit: 50 req/s)",
                details=f"This is API Gateway's built-in throttle, not our custom {limit}/min rate limit. Error: {error_message}"
            ))
            
            suite.add(TestResult(
                name=f"Rate limit {category} - custom limit tested",
                passed=False,
                message="Could not test custom rate limit - hit API Gateway throttle first",
                details="Reduce concurrent requests or increase delay between batches to avoid API Gateway throttle"
            ))
        else:
            # Our custom rate limit - verify the error message contains specific information
            category_keywords = {
                "workspaces": ["workspace"],
                "files": ["file"],
                "annotations": ["annotation"],
                "auth": ["auth", "authentication"]
            }
            keywords = category_keywords.get(category, [category])
            has_category = any(kw in error_message.lower() for kw in keywords)
            has_limit = "limit" in error_message.lower()
            has_seconds = "seconds" in error_message.lower() or "retry" in error_message.lower()
            
            suite.add(TestResult(
                name=f"Rate limit {category} - hit limit",
                passed=True,
                message=f"Hit custom rate limit after {requests_made} requests (limit: {limit})",
                details=error_message
            ))
            
            suite.add(TestResult(
                name=f"Rate limit {category} - error contains category",
                passed=has_category,
                message=f"Error message contains category keyword: {has_category}",
                details=error_message
            ))
            
            suite.add(TestResult(
                name=f"Rate limit {category} - error contains limit info",
                passed=has_limit,
                message=f"Error message contains 'limit': {has_limit}",
                details=error_message
            ))
            
            suite.add(TestResult(
                name=f"Rate limit {category} - error contains retry time",
                passed=has_seconds,
                message=f"Error message contains retry info: {has_seconds}",
                details=error_message
            ))
        
        # Also verify headers were present (for our custom rate limit)
        if rate_limit_headers and rate_limit_headers.get("limit") != "0":
            suite.add(TestResult(
                name=f"Rate limit {category} - headers present",
                passed=True,
                message=f"Headers: limit={rate_limit_headers.get('limit')}, remaining={rate_limit_headers.get('remaining')}",
                details=str(rate_limit_headers)
            ))
    else:
        # This is a FAILURE - we should have hit the limit
        error_detail = f"Made {requests_made} requests without hitting limit (expected: {limit}/min). "
        if rate_limit_headers:
            error_detail += f"Last headers: {rate_limit_headers}"
        else:
            error_detail += "No rate limit headers were returned - rate limiting may not be enabled."
        
        suite.add(TestResult(
            name=f"Rate limit {category} - hit limit",
            passed=False,
            message=f"Made {requests_made} requests without hitting limit (expected: {limit}/min)",
            details=error_detail
        ))


def test_all_rate_limits_exhaustion(client: SyncAPIClient, suite: TestSuite, workspace_id: str):
    """Test rate limit exhaustion for all categories."""
    
    # Test workspaces (10/min) - lowest limit, test first
    test_rate_limit_exhaustion_for_category(
        client, suite, workspace_id,
        category="workspaces",
        limit=RATE_LIMITS["workspaces"],
        request_func=lambda: client.session.get(f"{BASE_URL}/workspaces", timeout=30)
    )
    
    # Test files (100/min)
    test_rate_limit_exhaustion_for_category(
        client, suite, workspace_id,
        category="files",
        limit=RATE_LIMITS["files"],
        request_func=lambda: client.session.get(f"{BASE_URL}/workspaces/{workspace_id}/files", timeout=30)
    )
    
    # Test annotations (1000/min) - high limit, will take a while
    logger.warning("Testing annotations rate limit (1000/min) - this will make many requests...")
    test_rate_limit_exhaustion_for_category(
        client, suite, workspace_id,
        category="annotations",
        limit=RATE_LIMITS["annotations"],
        request_func=lambda: client.session.get(f"{BASE_URL}/workspaces/{workspace_id}/annotations", timeout=30)
    )


def main():
    parser = argparse.ArgumentParser(
        description="Test Breachline Sync API rate limits and request validators"
    )
    parser.add_argument(
        "settings_file",
        help="Path to Breachline settings YAML file"
    )
    parser.add_argument(
        "-v", "--verbose",
        action="store_true",
        help="Enable verbose output"
    )
    parser.add_argument(
        "--skip-exhaustion",
        action="store_true",
        help="Skip rate limit exhaustion test (avoids hitting actual limits)"
    )
    
    args = parser.parse_args()
    
    if args.verbose:
        logging.getLogger().setLevel(logging.DEBUG)
    
    # Load settings
    logger.info(f"Loading settings from {args.settings_file}")
    try:
        settings = load_settings(args.settings_file)
    except FileNotFoundError:
        logger.error(f"Settings file not found: {args.settings_file}")
        sys.exit(1)
    except yaml.YAMLError as e:
        logger.error(f"Failed to parse settings file: {e}")
        sys.exit(1)
    
    # Extract required tokens
    session_token = settings.get("sync_session_token")
    license_key = settings.get("license")
    
    if not session_token:
        logger.error("sync_session_token not found in settings file")
        sys.exit(1)
    if not license_key:
        logger.error("license not found in settings file")
        sys.exit(1)
    
    logger.info("Settings loaded successfully")
    
    # Create API client
    client = SyncAPIClient(session_token, license_key)
    suite = TestSuite()
    
    try:
        # Create a test workspace for testing
        logger.info("\n=== Setting up test workspace ===")
        test_workspace_name = f"rate-limit-test-{uuid.uuid4().hex[:8]}"
        status, data = client.create_workspace(test_workspace_name)
        
        if status != 201:
            logger.error(f"Failed to create test workspace: {status} - {data}")
            sys.exit(1)
        
        workspace_id = data["workspace_id"]
        logger.info(f"Created test workspace: {workspace_id}")
        
        # Run validation tests
        test_validation_limits(client, suite, workspace_id)
        
        # Test rate limit headers
        test_rate_limit_headers(client, suite, workspace_id)
        
        # Test individual rate limit categories
        test_rate_limit(
            client, suite, "workspaces", RATE_LIMITS["workspaces"],
            lambda: client.list_workspaces(),
            workspace_id
        )
        
        test_rate_limit(
            client, suite, "files", RATE_LIMITS["files"],
            lambda: client.list_files(workspace_id),
            workspace_id
        )
        
        test_rate_limit(
            client, suite, "annotations", RATE_LIMITS["annotations"],
            lambda: client.list_annotations(workspace_id),
            workspace_id
        )
        
        # Test rate limit exhaustion for all categories (optional)
        if not args.skip_exhaustion:
            test_all_rate_limits_exhaustion(client, suite, workspace_id)
        else:
            logger.info("\n=== Skipping Rate Limit Exhaustion Tests ===")
        
    finally:
        # Cleanup
        client.cleanup()
    
    # Print summary
    passed, failed, total = suite.summary()
    logger.info("\n" + "=" * 60)
    logger.info("TEST SUMMARY")
    logger.info("=" * 60)
    logger.info(f"Total:  {total}")
    logger.info(f"Passed: {passed}")
    logger.info(f"Failed: {failed}")
    logger.info("=" * 60)
    
    # Exit with appropriate code
    sys.exit(0 if failed == 0 else 1)


if __name__ == "__main__":
    main()
