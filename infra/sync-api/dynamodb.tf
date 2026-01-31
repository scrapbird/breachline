# PINs Table - Temporary storage for authentication PINs
resource "aws_dynamodb_table" "pins" {
  name         = "breachline-sync-pins"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "email"
  range_key    = "pin_hash"

  attribute {
    name = "email"
    type = "S"
  }

  attribute {
    name = "pin_hash"
    type = "S"
  }

  ttl {
    attribute_name = "expires_at"
    enabled        = true
  }

  point_in_time_recovery {
    enabled = true
  }

  server_side_encryption {
    enabled = true
  }

  tags = {
    Name = "breachline-sync-pins"
  }
}

# Workspaces Table
resource "aws_dynamodb_table" "workspaces" {
  name         = "breachline-sync-workspaces"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "workspace_id"

  attribute {
    name = "workspace_id"
    type = "S"
  }

  attribute {
    name = "owner_email"
    type = "S"
  }

  attribute {
    name = "created_at"
    type = "S"
  }

  # GSI for querying workspaces by owner
  global_secondary_index {
    name            = "owner-index"
    hash_key        = "owner_email"
    range_key       = "created_at"
    projection_type = "ALL"
  }

  point_in_time_recovery {
    enabled = true
  }

  server_side_encryption {
    enabled = true
  }

  tags = {
    Name = "breachline-sync-workspaces"
  }
}

# WorkspaceMembers Table
resource "aws_dynamodb_table" "workspace_members" {
  name         = "breachline-sync-workspace-members"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "workspace_id"
  range_key    = "email"

  attribute {
    name = "workspace_id"
    type = "S"
  }

  attribute {
    name = "email"
    type = "S"
  }

  # GSI for querying all workspaces a user is a member of
  global_secondary_index {
    name            = "user-workspaces-index"
    hash_key        = "email"
    range_key       = "workspace_id"
    projection_type = "ALL"
  }

  point_in_time_recovery {
    enabled = true
  }

  server_side_encryption {
    enabled = true
  }

  tags = {
    Name = "breachline-sync-workspace-members"
  }
}

# Annotations Table
resource "aws_dynamodb_table" "annotations" {
  name         = "breachline-sync-annotations"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "workspace_id"
  range_key    = "annotation_id"

  attribute {
    name = "workspace_id"
    type = "S"
  }

  attribute {
    name = "annotation_id"
    type = "S"
  }

  attribute {
    name = "file_hash"
    type = "S"
  }

  attribute {
    name = "color"
    type = "S"
  }

  attribute {
    name = "created_by"
    type = "S"
  }

  attribute {
    name = "jpath"
    type = "S"
  }

  # GSI for querying annotations by file hash
  global_secondary_index {
    name            = "file-hash-index"
    hash_key        = "workspace_id"
    range_key       = "file_hash"
    projection_type = "ALL"
  }

  # GSI for querying annotations by file hash + jpath (for JSON files)
  global_secondary_index {
    name            = "file-hash-jpath-index"
    hash_key        = "file_hash"
    range_key       = "jpath"
    projection_type = "ALL"
  }

  # GSI for querying annotations by color
  global_secondary_index {
    name            = "color-index"
    hash_key        = "workspace_id"
    range_key       = "color"
    projection_type = "ALL"
  }

  # GSI for querying annotations by user
  global_secondary_index {
    name            = "user-index"
    hash_key        = "workspace_id"
    range_key       = "created_by"
    projection_type = "ALL"
  }

  point_in_time_recovery {
    enabled = true
  }

  server_side_encryption {
    enabled = true
  }

  tags = {
    Name = "breachline-sync-annotations"
  }
}

# Workspace Files Table - Stores file metadata for workspaces
# Uses file_identifier as range key to support multiple entries of the same file
# with different jpath and no_header_row values
resource "aws_dynamodb_table" "workspace_files" {
  name         = "breachline-sync-workspace-files"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "workspace_id"
  range_key    = "file_identifier"

  attribute {
    name = "workspace_id"
    type = "S"
  }

  attribute {
    name = "file_identifier"
    type = "S"
  }

  attribute {
    name = "file_hash"
    type = "S"
  }

  # GSI for querying files by file_hash alone (for lookups and migrations)
  global_secondary_index {
    name            = "file-hash-index"
    hash_key        = "workspace_id"
    range_key       = "file_hash"
    projection_type = "ALL"
  }

  point_in_time_recovery {
    enabled = true
  }

  server_side_encryption {
    enabled = true
  }

  tags = {
    Name    = "breachline-sync-workspace-files"
    project = "breachline"
  }
}

# Audit Table - Stores workspace change audit log
resource "aws_dynamodb_table" "audit" {
  name         = "breachline-sync-audit"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "audit_id"
  range_key    = "created_at"

  attribute {
    name = "audit_id"
    type = "S"
  }

  attribute {
    name = "created_at"
    type = "S"
  }

  attribute {
    name = "workspace_id"
    type = "S"
  }

  # GSI for querying audit entries by workspace
  global_secondary_index {
    name            = "workspace-audit-index"
    hash_key        = "workspace_id"
    range_key       = "created_at"
    projection_type = "ALL"
  }

  ttl {
    attribute_name = "ttl"
    enabled        = true
  }

  point_in_time_recovery {
    enabled = true
  }

  server_side_encryption {
    enabled = true
  }

  tags = {
    Name    = "breachline-sync-audit"
    project = "breachline"
  }
}

# User Subscriptions Table - Stores user workspace limits and seat counts
resource "aws_dynamodb_table" "user_subscriptions" {
  name         = "breachline-sync-user-subscriptions"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "email"

  attribute {
    name = "email"
    type = "S"
  }

  point_in_time_recovery {
    enabled = true
  }

  server_side_encryption {
    enabled = true
  }

  tags = {
    Name = "breachline-sync-user-subscriptions"
  }
}

# Workspace File Locations Table - Stores absolute file paths for each Breachline instance
resource "aws_dynamodb_table" "workspace_file_locations" {
  name         = "breachline-sync-workspace-file-locations"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "instance_id"
  range_key    = "file_hash"

  attribute {
    name = "instance_id"
    type = "S"
  }

  attribute {
    name = "file_hash"
    type = "S"
  }

  attribute {
    name = "workspace_id"
    type = "S"
  }

  # GSI for querying by workspace_id + file_hash
  global_secondary_index {
    name            = "workspace-file-index"
    hash_key        = "workspace_id"
    range_key       = "file_hash"
    projection_type = "ALL"
  }

  point_in_time_recovery {
    enabled = true
  }

  server_side_encryption {
    enabled = true
  }

  tags = {
    Name    = "breachline-sync-workspace-file-locations"
    project = "breachline"
  }
}
