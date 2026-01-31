# Sync API Architecture

## Overview

The Sync API is implemented as a collection of serverless AWS Lambda functions written in Go, with DynamoDB for data persistence. Lambda functions apply changes directly to DynamoDB tables using optimistic locking for consistency. This architecture ensures high scalability and cost-effectiveness.

## Pricing Model

### Premium License
- **Monthly**: $10 USD per month
- **Yearly**: $100 USD per year
- Required for all users (including invited team members)

### Workspace Sync Service

**Base Plan**: $5 USD per month
- Includes 5 online workspaces
- For workspace owners only

**Additional Workspaces**: $5 USD per pack (5 workspaces)
- Purchased in increments of 5
- Examples: 10 workspaces = $10/month, 15 workspaces = $15/month

**Team Collaboration Seats**: $1 USD per seat per month
- Each seat allows adding 1 team member to 1 workspace
- Same user in multiple workspaces = multiple seats
- Purchased individually as needed

**Key Points**:
- Invited members need Premium License but not Workspace Sync subscription
- All sync costs paid by workspace owner
- Members don't consume workspace quota

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────┐
│               API Gateway (REST API)                         │
│           https://api.breachline.com/v1                     │
└───────────────────────┬─────────────────────────────────────┘
                        │
                        ▼
┌────────────────────────────────────────────────────────────┐
│           Lambda Authorizer (JWT Validation)                │
└────────────────────────────────────────────────────────────┘
                        │
        ┌───────────────┴───────────────┐
        │                               │
        ▼                               ▼
┌──────────────┐              ┌──────────────────┐
│  Auth Lambda │              │ Workspace Lambda │
│  Functions   │──────────┐   │   Functions      │
└──────┬───────┘          │   └────────┬─────────┘
       │                  │            │
       │                  ▼            ▼
       │          ┌──────────────┐  ┌──────────────────┐
       │          │  AWS SES     │  │ Annotation Lambda│
       │          │ (Send PINs)  │  │   Functions      │
       │          └──────────────┘  └────────┬─────────┘
       │                                     │
       └───────────────┬─────────────────────┘
                       │
                       ▼
        ┌──────────────────────────────┐
        │     DynamoDB Tables          │
        │  - PINs (TTL: 12h)           │
        │  - Workspaces                │
        │  - WorkspaceMembers          │
        │  - Annotations               │
        │  - WorkspaceFiles            │
        │  - WorkspaceFileLocations    │
        └──────────────────────────────┘
```

## Components

### Lambda Functions

All functions are written in Go

### Key Lambda Functions

1. **Authentication Functions**
   - `auth-request-pin`: Validate license and send 6-digit PIN via email
   - `auth-verify-pin`: Verify PIN and issue JWT tokens
   - `auth-refresh`: Refresh expired tokens
   - `auth-logout`: Invalidate sessions

2. **Workspace Functions**
   - `workspace-create`: Create new workspaces (enforces per-user workspace limit)
   - `workspace-list`: List accessible workspaces
   - `workspace-get`: Get workspace details
   - `workspace-update`: Update workspace name
   - `workspace-delete`: Delete workspace and all data
   - `workspace-convert-to-shared`: Convert to shared workspace

3. **Annotation Functions**
   - `annotation-list`: List annotations with filters
   - `annotation-get`: Get annotation details
   - `annotation-create/update/delete`: Modify annotations directly in DynamoDB

4. **Team Management Functions**
   - `team-list-members`: List workspace members
   - `team-add-member`: Add member (consumes owner's seats)
   - `team-update-member`: Update member role
   - `team-remove-member`: Remove member

## DynamoDB Schema

### PINs Table
- **PK**: `email`
- **Attributes**: pin_hash, license_key_hash, expires_at, created_at
- **TTL**: expires_at (12 hours)
- **Note**: Stores temporary PINs for email-based authentication. No persistent user accounts or sessions.

### Workspaces Table
- **PK**: `workspace_id`
- **GSI**: `owner-index` (owner_email, created_at)
- **Attributes**: name, owner_email, is_shared, member_count, version, created_at, updated_at

### WorkspaceMembers Table
- **PK**: `workspace_id`
- **SK**: `email`
- **GSI**: `user-workspaces-index` (email, workspace_id)
- **Attributes**: role, added_at, last_active

### Annotations Table
- **PK**: `workspace_id`
- **SK**: `annotation_id`
- **GSI**: `file-path-index`, `type-index`, `user-index`
- **Attributes**: file_path, content, annotation_type, position, tags, created_by, created_at, updated_at, version

### WorkspaceFiles Table
- **PK**: `workspace_id`
- **SK**: `file_hash`
- **Attributes**: jpath, description, created_at, updated_at
- **Purpose**: Stores files added to workspaces with their JSONPath expressions and descriptions

### WorkspaceFileLocations Table
- **PK**: `instance_id`
- **SK**: `file_hash`
- **GSI**: `file-hash-index` (file_hash, instance_id)
- **Attributes**: workspace_id, file_path, created_at, updated_at
- **Purpose**: Maps file hashes to local file paths on specific Breachline instances

### UserSubscriptions Table
- **PK**: `email`
- **Attributes**: workspace_limit, seat_count, created_at, updated_at
- **Note**: Stores user subscription data (workspace limits and seat counts). Updated independently of JWT tokens to allow immediate effect of plan upgrades.

## Data Consistency

The architecture ensures data consistency through the following mechanisms:

### 1. Optimistic Locking

- Workspace records include monotonically increasing `version` field
- All updates use conditional expressions:
  ```
  ConditionExpression: "version = :expected_version"
  ```
- DynamoDB transactions ensure atomicity across multiple tables

### 2. Direct Lambda Processing

- Lambda functions apply changes directly to DynamoDB tables
- No intermediate queuing or message processing
- Immediate consistency for all operations
- Simplified error handling and debugging

### 3. Atomic Operations

- Each API call performs complete operations within DynamoDB transactions
- Multiple table updates are atomic (all succeed or all fail)
- No partial state or orphaned records

### Processing Flow Example

```
Client Request (workspace ws_123):
  Create Annotation A

annotation-create Lambda:
  ├─ Validate workspace access and permissions
  ├─ Check current workspace version = 10
  ├─ Begin DynamoDB transaction:
  │   ├─ Create annotation record in Annotations table
  │   ├─ Update Workspaces.version = 11 (conditional on version = 10)
  │   └─ Commit transaction
  └─ Return success with annotation details

Client Request (workspace ws_123):
  Create Annotation B

annotation-create Lambda:
  ├─ Validate workspace access and permissions
  ├─ Check current workspace version = 11
  ├─ Begin DynamoDB transaction:
  │   ├─ Create annotation record in Annotations table
  │   ├─ Update Workspaces.version = 12 (conditional on version = 11)
  │   └─ Commit transaction
  └─ Return success with annotation details

Result: All changes applied immediately with strong consistency
```


## Authentication & Authorization

### PIN-Based Authentication Flow

1. **Request PIN** (`/auth/request-pin`):
   - Client sends license file only
   - Lambda validates license signature
   - Lambda extracts email from license
   - Generates 6-digit PIN (cryptographically random)
   - Stores hashed PIN in PINs table with email as key (TTL: 12 hours)
   - Sends PIN via SES to the extracted email address
   - Returns success response with masked email

2. **Verify PIN** (`/auth/verify-pin`):
   - Client sends license and PIN
   - Lambda validates license signature
   - Lambda extracts email from license
   - Looks up and validates PIN for that email
   - Checks PIN hasn't expired (12 hours)
   - Creates or updates user subscription record in database with default values from license
   - Generates signed JWT tokens (access and refresh)
   - Deletes used PIN from PINs table
   - Returns access and refresh tokens
   - **No session storage** - tokens are stateless and cryptographically verified

### JWT Tokens

**Access Token** (24h expiry):
```json
{
  "sub": "user@example.com",
  "email": "user@example.com",
  "license_expires_at": "2026-10-15T07:01:24Z",
  "license_key_hash": "sha256:abc123...",
  "exp": 1697328084,
  "iat": 1697241684,
  "type": "access"
}
```

**Refresh Token** (30d expiry):
```json
{
  "sub": "user@example.com",
  "email": "user@example.com",
  "license_key_hash": "sha256:abc123...",
  "exp": 1699833684,
  "iat": 1697241684,
  "type": "refresh"
}
```

**Note**: All tokens are stateless and cryptographically signed. No server-side session storage is required. Workspace limits and seat counts are stored in the database and queried in real-time, allowing immediate effect of plan upgrades without re-authentication.

### Lambda Authorizer

- Validates JWT signatures using secret from Secrets Manager
- Validates license hasn't expired (from token claims)
- Verifies token type matches endpoint requirements (access vs refresh)
- Returns IAM policy allowing/denying invocation
- Caches authorization for 5 minutes per token
- Injects `email` and `license_key_hash` into request context
- **Fully stateless** - no database lookups required for authorization

### Authorization Checks

Each Lambda verifies:
1. User has access to workspace (owner or member) via email
2. User has required role (owner for sensitive operations)
3. Resource limits not exceeded (workspace quota, available seats) - queried from UserSubscriptions table in real-time
4. License is still valid (not expired)

## Error Handling

### Error Response Format

```json
{
  "error": {
    "code": "insufficient_seats",
    "message": "Not enough available seats to add this member",
    "details": {
      "seats_used": 5,
      "seat_count": 5,
      "seats_required": 1,
      "purchase_url": "https://breachline.com/purchase-seats"
    }
  }
}
```

### Retry Strategy

- **Client Errors (4xx)**: No retry, return immediately
- **Server Errors (5xx)**: API Gateway auto-retries
- **DynamoDB Throttling**: Exponential backoff with jitter
- **Lambda Failures**: API Gateway auto-retries failed requests

### Monitoring

- CloudWatch Logs for all Lambda functions
- CloudWatch Metrics for custom business metrics
- X-Ray tracing for distributed operations
- CloudWatch Alarms:
  - Lambda error rate > 1%
  - API Gateway 5xx > 1%
  - DynamoDB throttled requests > 10

## Deployment

### Build & Deploy

```bash
# Build Lambda functions
./build.sh

# Deploy infrastructure
terraform init
terraform plan
terraform apply
```

### State Management

- Terraform state stored in S3: `scrappy-tfstate/sync-api/terraform.tfstate`
- State locking with use_lockfile: true
- Region: ap-southeast-2 (Sydney)

## Performance Considerations

### Cold Start Optimization

- Go compiled binaries: ~100ms cold start
- ARM64 architecture for better performance
- Minimal dependencies to reduce package size
- Connection pooling for DynamoDB

### Scalability

- DynamoDB on-demand billing auto-scales
- Lambda functions scale automatically with request volume
- Direct processing eliminates queue bottlenecks
- Lambda concurrency limits prevent overwhelming DynamoDB

### Caching

- API Gateway caching for GET responses (60s)
- Lambda authorizer cache (5 min)
- Client-side workspace state caching

## Security

- **Encryption**: DynamoDB encryption at rest with KMS
- **TLS**: All API calls use TLS 1.2+
- **Secrets**: JWT keys in AWS Secrets Manager
- **PIN Hashing**: bcrypt with cost factor 10 (PINs are short-lived)
- **License Validation**: ECDSA signature verification of JWT tokens using public key
- **Email Delivery**: AWS SES with DKIM/SPF
- **Rate Limiting**: API Gateway throttling on PIN requests (max 3 per email per hour)
- **IAM**: Least-privilege roles per Lambda
- **API Gateway**: CloudFront + WAF protection
- **VPC**: Lambda functions run outside VPC for performance
- **No Persistent User Data**: User accounts are stateless; all data derived from license
