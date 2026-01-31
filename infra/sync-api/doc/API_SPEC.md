# BreachLine Sync API Specification

## Table of Contents

1. [Overview](#overview)
2. [Authentication](#authentication)
3. [API Endpoints](#api-endpoints)
   - [Authentication & User Management](#authentication--user-management)
   - [Workspace Management](#workspace-management)
   - [Synchronization](#synchronization)
   - [Annotations](#annotations)
   - [Team Management](#team-management)
4. [Data Models](#data-models)
5. [Error Responses](#error-responses)

## Overview

The BreachLine Sync API is a RESTful API that enables real-time collaboration by synchronizing workspaces and annotations across team members and devices.

**Base URL**: `https://api.breachline.com/v1`

**Content Type**: `application/json`

**Authentication**: Bearer token in `Authorization` header

## Pricing

### Premium License

Required to use the Sync API and BreachLine application:
- **Monthly**: $10 USD per month
- **Yearly**: $100 USD per year

### Workspace Sync Service

**Base Plan**: $5 USD per month
- Includes 5 online workspaces
- Single-user workspaces (no collaboration)

**Additional Workspaces**: $5 USD per month per pack
- Each pack adds 5 additional workspaces
- Examples:
  - 10 workspaces: $10/month (2 packs)
  - 15 workspaces: $15/month (3 packs)

**Team Collaboration Seats**: $1 USD per seat per month
- Each seat allows adding 1 team member to 1 workspace
- Adding the same user to multiple workspaces requires multiple seats
- Examples:
  - Add 1 user to 2 workspaces: 2 seats = $2/month
  - Add 3 users to 1 workspace: 3 seats = $3/month
  - Add 2 users to 3 workspaces each: 6 seats = $6/month

**Important Notes**:
- Invited team members need a valid Premium License but do not need their own Workspace Sync subscription
- All sync costs are paid by the workspace owner
- Users who join shared workspaces do not consume their own workspace quota

## Authentication

All API requests (except PIN request and verification) must include a valid JWT token in the Authorization header:

```
Authorization: Bearer <token>
```

Tokens expire after 24 hours and can be refreshed using the refresh endpoint.

### Authentication Flow

1. Application calls `/auth/request-pin` with the user's license file
2. Backend extracts email from the license and validates it
3. Backend sends 6-digit PIN to the extracted email address (valid for 12 hours)
4. User enters PIN in application
5. Application calls `/auth/verify-pin` with the license and PIN
6. Backend extracts email from license, validates PIN, and returns signed JWT tokens (access and refresh)
7. All subsequent requests use the access token for authentication
8. When access token expires, use refresh token to get a new access token
9. **Fully stateless** - no user accounts or sessions stored server-side; all authentication via cryptographically signed JWT tokens

## API Endpoints

### Authentication & User Management

#### POST /auth/request-pin

Request a 6-digit PIN to be sent to the user's email address. The email is extracted from the provided license file.

**Request Body**:
```json
{
  "license": "<base64-encoded-license-data>"
}
```

**Response** (200 OK):
```json
{
  "message": "PIN sent to user@example.com",
  "expires_at": "2025-10-16T07:01:24Z",
  "pin_id": "pin_abc123"
}
```

**Error Response** (400 Bad Request):
```json
{
  "error": {
    "code": "invalid_license",
    "message": "License is invalid or could not extract email"
  }
}
```

**Note**: The PIN is valid for 12 hours. The email address is extracted from the license file. A new PIN request will invalidate any previous PINs for that email.

---

#### POST /auth/verify-pin

Verify the PIN and receive access tokens. The email is extracted from the provided license file.

**Request Body**:
```json
{
  "license": "<base64-encoded-license-data>",
  "pin": "123456"
}
```

**Response** (200 OK):
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "token_type": "Bearer",
  "expires_in": 86400,
  "user": {
    "email": "user@example.com",
    "workspace_limit": 5,
    "workspace_count": 2,
    "seat_count": 10,
    "seats_used": 3
  }
}
```

**Error Response** (400 Bad Request):
```json
{
  "error": {
    "code": "bad_request",
    "message": "Invalid request parameters"
  }
}
```

**Error Response** (401 Unauthorized):
```json
{
  "error": {
    "code": "unauthorized",
    "message": "Missing or invalid authentication token"
  }
}
```

**Note**: 
- The email address is extracted from the license file by the backend
- User subscription data (workspace_limit, seat_count) is stored in the database and can be updated independently of the JWT tokens
- On first login, a user subscription record is created with default values from the license
- Tokens are stateless and cryptographically signed
- Plan upgrades and seat purchases take effect immediately without requiring re-authentication

---

#### POST /auth/refresh

Refresh an expired access token.

**Request Body**:
```json
{
  "license": "<base64-encoded-license-data>",
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

**Response** (200 OK):
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "token_type": "Bearer",
  "expires_in": 86400
}
```

**Error Response** (401 Unauthorized):
```json
{
  "error": {
    "code": "unauthorized",
    "message": "Missing or invalid authentication token"
  }
}
```

---

#### POST /auth/logout

Logout is handled client-side by discarding tokens. Since authentication is stateless with signed JWT tokens, there is no server-side session to invalidate. Tokens will naturally expire based on their expiry time.

**Note**: This endpoint is optional and may be omitted. Clients should simply delete stored tokens locally to logout.

---

### Workspace Management

#### POST /workspaces

Create a new workspace. By default, workspaces are single-user. Each user has a workspace limit that can be increased by purchasing additional workspace slots. This limit governs how many workspaces they can be a member of.

**Headers**: `Authorization: Bearer <token>`

**Request Body**:
```json
{
  "name": "Investigation Alpha",
  "is_shared": false
}
```

**Response** (201 Created):
```json
{
  "workspace_id": "ws_xyz789",
  "name": "Investigation Alpha",
  "owner_id": "usr_abc123",
  "is_shared": false,
  "member_count": 1,
  "created_at": "2025-10-15T07:01:24Z",
  "updated_at": "2025-10-15T07:01:24Z",
  "version": 1
}
```

**Error Response** (403 Forbidden):
```json
{
  "error": {
    "code": "workspace_limit_reached",
    "message": "You have reached your workspace limit",
    "details": {
      "current_count": 5,
      "limit": 5,
      "purchase_url": "https://breachline.com/purchase-workspaces"
    }
  }
}
```

---

#### GET /workspaces

List all workspaces accessible to the current user.

**Headers**: `Authorization: Bearer <token>`

**Query Parameters**:
- `owned` (boolean, optional): Filter by owned workspaces only
- `shared` (boolean, optional): Filter by shared workspaces only
- `limit` (integer, optional, default: 50): Number of results per page
- `offset` (integer, optional, default: 0): Pagination offset

**Response** (200 OK):
```json
{
  "workspaces": [
    {
      "workspace_id": "ws_xyz789",
      "name": "Investigation Alpha",
      "owner_id": "usr_abc123",
      "is_shared": true,
      "role": "owner",
      "member_count": 3,
      "created_at": "2025-10-15T07:01:24Z",
      "updated_at": "2025-10-15T07:01:24Z",
      "last_synced_at": "2025-10-15T07:01:24Z",
      "version": 15
    }
  ],
  "total": 1,
  "limit": 50,
  "offset": 0
}
```

---

#### GET /workspaces/{workspace_id}

Get detailed information about a specific workspace.

**Headers**: `Authorization: Bearer <token>`

**Response** (200 OK):
```json
{
  "workspace_id": "ws_xyz789",
  "name": "Investigation Alpha",
  "owner_id": "usr_abc123",
  "is_shared": true,
  "member_count": 3,
  "created_at": "2025-10-15T07:01:24Z",
  "updated_at": "2025-10-15T07:01:24Z",
  "version": 15,
  "statistics": {
    "total_files": 1234,
    "total_annotations": 567
  }
}
```

---

#### PUT /workspaces/{workspace_id}

Update workspace name (owner only).

**Headers**: `Authorization: Bearer <token>`

**Request Body**:
```json
{
  "name": "Investigation Alpha - Updated"
}
```

**Response** (200 OK):
```json
{
  "workspace_id": "ws_xyz789",
  "name": "Investigation Alpha - Updated",
  "owner_id": "usr_abc123",
  "is_shared": true,
  "member_count": 3,
  "created_at": "2025-10-15T07:01:24Z",
  "updated_at": "2025-10-15T07:01:24Z",
  "version": 16
}
```

**Error Response** (401 Unauthorized):
```json
{
  "error": {
    "code": "unauthorized",
    "message": "Missing or invalid authentication token"
  }
}
```

**Error Response** (404 Not Found):
```json
{
  "error": {
    "code": "not_found",
    "message": "Resource not found"
  }
}
```

---

#### DELETE /workspaces/{workspace_id}

Delete a workspace (owner only).

**Headers**: `Authorization: Bearer <token>`

**Response** (204 No Content)

**Error Response** (401 Unauthorized):
```json
{
  "error": {
    "code": "unauthorized",
    "message": "Missing or invalid authentication token"
  }
}
```

**Error Response** (404 Not Found):
```json
{
  "error": {
    "code": "not_found",
    "message": "Resource not found"
  }
}
```

---

#### POST /workspaces/{workspace_id}/convert-to-shared

Convert a single-user workspace to a shared workspace (owner only). Once converted, a workspace cannot be converted back to single-user.

**Headers**: `Authorization: Bearer <token>`

**Response** (200 OK):
```json
{
  "workspace_id": "ws_xyz789",
  "name": "Investigation Alpha",
  "owner_id": "usr_abc123",
  "is_shared": true,
  "member_count": 1,
  "updated_at": "2025-10-15T07:01:24Z",
  "version": 17
}
```

**Error Response** (409 Conflict):
```json
{
  "error": {
    "code": "already_shared",
    "message": "Workspace is already shared"
  }
}
```

---

### Annotations

#### GET /workspaces/{workspace_id}/annotations

List all annotations in a workspace.

**Headers**: `Authorization: Bearer <token>`

**Query Parameters**:
- `file_path` (string, optional): Filter by file path
- `annotation_type` (string, optional): Filter by type (note, finding, bookmark, etc.)
- `created_by` (string, optional): Filter by user ID
- `tags` (string[], optional): Filter by tags
- `limit` (integer, optional, default: 100): Number of results per page
- `offset` (integer, optional, default: 0): Pagination offset

**Response** (200 OK):
```json
{
  "annotations": [
    {
      "annotation_id": "ann_111",
      "workspace_id": "ws_xyz789",
      "file_path": "/evidence/network.pcap",
      "content": "Suspicious traffic detected",
      "annotation_type": "note",
      "position": {"line": 42},
      "tags": ["network", "suspicious"],
      "created_by": "usr_def456",
      "created_by_name": "Jane Smith",
      "created_at": "2025-10-15T06:15:00Z",
      "updated_at": "2025-10-15T06:15:00Z",
      "version": 11
    }
  ],
  "total": 567,
  "limit": 100,
  "offset": 0
}
```

---

#### GET /workspaces/{workspace_id}/annotations/{annotation_id}

Get details of a specific annotation.

**Headers**: `Authorization: Bearer <token>`

**Response** (200 OK):
```json
{
  "annotation_id": "ann_111",
  "workspace_id": "ws_xyz789",
  "file_path": "/evidence/network.pcap",
  "content": "Suspicious traffic detected",
  "annotation_type": "note",
  "position": {"line": 42},
  "tags": ["network", "suspicious"],
  "created_by": "usr_def456",
  "created_by_name": "Jane Smith",
  "created_at": "2025-10-15T06:15:00Z",
  "updated_at": "2025-10-15T06:15:00Z",
  "version": 11,
  "edit_history": [
    {
      "edited_by": "usr_def456",
      "edited_at": "2025-10-15T06:15:00Z",
      "changes": ["created"]
    }
  ]
}
```

---

#### POST /workspaces/{workspace_id}/annotations

Create a new annotation.

**Headers**: `Authorization: Bearer <token>`

**Request Body**:
```json
{
  "file_path": "/evidence/malware.exe",
  "file_hash": "sha256:abc123...",
  "column_hashes": [
    {"column1": "hash1"},
    {"column2": "hash2"}
  ],
  "note": "PE32 executable analysis",
  "color": "red"
}
```

**Response** (201 Created):
```json
{
  "annotation_id": "ann_444",
  "workspace_id": "ws_xyz789",
  "file_path": "/evidence/malware.exe",
  "content": "PE32 executable analysis",
  "annotation_type": "finding",
  "position": {"offset": 0},
  "tags": ["malware", "critical"],
  "created_by": "usr_abc123",
  "created_by_name": "John Doe",
  "created_at": "2025-10-15T07:01:24Z",
  "version": 17
}
```

---

#### PUT /workspaces/{workspace_id}/annotations/{annotation_id}

Update an existing annotation.

**Headers**: `Authorization: Bearer <token>`

**Request Body**:
```json
{
  "note": "Updated analysis with more details",
  "color": "blue"
}
```

**Response** (200 OK):
```json
{
  "annotation_id": "ann_444",
  "workspace_id": "ws_xyz789",
  "file_path": "/evidence/malware.exe",
  "content": "Updated analysis with more details",
  "annotation_type": "note",
  "position": {"offset": 0},
  "tags": ["malware", "critical", "analyzed"],
  "created_by": "usr_abc123",
  "created_by_name": "John Doe",
  "created_at": "2025-10-15T07:01:24Z",
  "updated_at": "2025-10-15T07:05:24Z",
  "version": 18
}
```

---

#### DELETE /workspaces/{workspace_id}/annotations/{annotation_id}

Delete an annotation.

**Headers**: `Authorization: Bearer <token>`

**Response** (204 No Content)

---

### Team Management

#### GET /workspaces/{workspace_id}/members

List all team members with access to the workspace.

**Headers**: `Authorization: Bearer <token>`

**Response** (200 OK):
```json
{
  "members": [
    {
      "user_id": "usr_abc123",
      "email": "john@example.com",
      "role": "owner",
      "added_at": "2025-10-15T07:01:24Z",
      "last_active": "2025-10-15T07:01:24Z"
    },
    {
      "user_id": "usr_def456",
      "email": "jane@example.com",
      "role": "editor",
      "added_at": "2025-10-15T07:10:00Z",
      "last_active": "2025-10-15T07:00:00Z"
    }
  ],
  "total": 2
}
```

---

#### POST /workspaces/{workspace_id}/members

Add a team member to a shared workspace (owner only). Each member added consumes one seat from the owner's seat pool. Seats can be purchased at $1 per seat per month.

**Headers**: `Authorization: Bearer <token>`

**Request Body**:
```json
{
  "email": "newmember@example.com",
  "role": "editor"
}
```

**Response** (201 Created):
```json
{
  "user_id": "usr_ghi789",
  "email": "newmember@example.com",
  "role": "editor",
  "added_at": "2025-10-15T07:01:24Z",
  "invitation_sent": true
}
```

**Error Response** (403 Forbidden) - Insufficient seats:
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

**Note**: Each member added to a workspace consumes one seat from the owner's seat pool. Adding the same user to multiple workspaces consumes one seat per workspace. Seats can be purchased at $1 per seat per month.

**Error Response** (400 Bad Request) - Not a shared workspace:
```json
{
  "error": {
    "code": "not_shared_workspace",
    "message": "Cannot add members to a single-user workspace. Convert it to a shared workspace first."
  }
}
```

---

#### PUT /workspaces/{workspace_id}/members/{user_id}

Update a team member's role.

**Headers**: `Authorization: Bearer <token>`

**Request Body**:
```json
{
  "role": "viewer"
}
```

**Response** (200 OK):
```json
{
  "user_id": "usr_def456",
  "role": "viewer",
  "updated_at": "2025-10-15T07:01:24Z"
}
```

---

#### DELETE /workspaces/{workspace_id}/members/{user_id}

Remove a team member from the workspace.

**Headers**: `Authorization: Bearer <token>`

**Response** (204 No Content)

---


## Data Models

### User
```json
{
  "email": "string",
  "workspace_limit": "integer (purchasable in increments of 5, default: 5)",
  "workspace_count": "integer",
  "seat_count": "integer (purchasable, $1 per seat)",
  "seats_used": "integer",
  "license_expires_at": "ISO8601 timestamp",
  "last_login": "ISO8601 timestamp"
}
```

**Note**: 
- User subscription data is stored in the database and can be updated independently of authentication
- `email`: Extracted from the license file
- `workspace_limit`: Can be increased by purchasing additional workspace packs (5 workspaces per pack at $5/month). Updates take effect immediately.
- `seat_count`: Total seats purchased. Each seat allows adding 1 team member to 1 workspace ($1/seat/month). Updates take effect immediately.
- `seats_used`: Number of seats currently in use across all shared workspaces
- `license_expires_at`: Expiration date from the license file

### Workspace
```json
{
  "workspace_id": "string",
  "name": "string",
  "owner_email": "string",
  "is_shared": "boolean",
  "member_count": "integer",
  "created_at": "ISO8601 timestamp",
  "updated_at": "ISO8601 timestamp",
  "version": "integer"
}
```

**Note**: 
- `owner_email`: Email of the workspace owner (from their license)
- Adding members to shared workspaces consumes seats from the owner's seat pool
- Each member in each workspace uses one seat (e.g., adding 1 user to 2 workspaces uses 2 seats)

### Annotation
```json
{
  "annotation_id": "string",
  "workspace_id": "string",
  "file_path": "string",
  "content": "string",
  "annotation_type": "string (note|finding|bookmark|highlight)",
  "position": {
    "line": "integer (optional)",
    "offset": "integer (optional)",
    "start": "integer (optional)",
    "end": "integer (optional)"
  },
  "tags": ["string"],
  "created_by": "string",
  "created_by_name": "string",
  "created_at": "ISO8601 timestamp",
  "updated_at": "ISO8601 timestamp",
  "version": "integer"
}
```

### Change
```json
{
  "change_id": "string",
  "workspace_id": "string",
  "version": "integer",
  "change_type": "string (annotation_created|annotation_updated|annotation_deleted|file_added|file_removed|file_updated)",
  "timestamp": "ISO8601 timestamp",
  "user_id": "string",
  "user_name": "string",
  "data": "object (varies by change_type)"
}
```

### WorkspaceMember
```json
{
  "user_id": "string",
  "email": "string",
  "role": "string (owner|editor|viewer)",
  "added_at": "ISO8601 timestamp",
  "last_active": "ISO8601 timestamp"
}
```

## Error Responses

All error responses follow this format:

```json
{
  "error": {
    "code": "string",
    "message": "string",
    "details": "object (optional)"
  }
}
```

### Common Error Codes

| Status Code | Error Code | Description |
|------------|------------|-------------|
| 400 | `bad_request` | Invalid request parameters |
| 401 | `unauthorized` | Missing or invalid authentication token |
| 403 | `forbidden` | Insufficient permissions |
| 403 | `insufficient_seats` | Not enough available seats to add member |
| 403 | `workspace_limit_reached` | User has reached workspace limit |
| 404 | `not_found` | Resource not found |
| 409 | `conflict` | Resource conflict (e.g., workspace already shared) |
| 409 | `already_shared` | Workspace is already shared |
| 429 | `rate_limit_exceeded` | Too many requests |

### Example Error Responses

**Workspace Limit Reached**:
```json
{
  "error": {
    "code": "workspace_limit_reached",
    "message": "You have reached your workspace limit",
    "details": {
      "current_count": 5,
      "limit": 5,
      "purchase_url": "https://breachline.com/purchase-workspaces"
    }
  }
}
```

**Insufficient Seats**:
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
