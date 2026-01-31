# Lambda function definitions
# Note: Lambda source code should be built and zipped before deployment

locals {
  lambda_functions = {
    # Authentication functions
    "auth-request-pin" = {
      description = "Validate license and send 6-digit PIN via email"
      timeout     = var.lambda_timeout
      environment = {
        PINS_TABLE               = aws_dynamodb_table.pins.name
        USER_SUBSCRIPTIONS_TABLE = aws_dynamodb_table.user_subscriptions.name
        LICENSE_PUBLIC_KEY       = aws_secretsmanager_secret.license_public_key.arn
        SES_FROM_EMAIL           = var.ses_email_from
        SES_CONFIGURATION_SET    = aws_ses_configuration_set.main.name
        PIN_TTL_HOURS            = var.pin_ttl_hours
        RATE_LIMITS_TABLE        = aws_dynamodb_table.rate_limits.name
      }
      iam_policy_statements = [
        {
          effect = "Allow"
          actions = [
            "dynamodb:PutItem",
            "dynamodb:Query",
            "dynamodb:GetItem"
          ]
          resources = [
            aws_dynamodb_table.pins.arn,
            aws_dynamodb_table.user_subscriptions.arn
          ]
        },
        {
          effect = "Allow"
          actions = ["secretsmanager:GetSecretValue"]
          resources = [aws_secretsmanager_secret.license_public_key.arn]
        },
        {
          effect = "Allow"
          actions = [
            "ses:SendEmail",
            "ses:SendRawEmail"
          ]
          resources = ["*"]
        },
        {
          effect = "Allow"
          actions = [
            "dynamodb:GetItem",
            "dynamodb:UpdateItem"
          ]
          resources = [aws_dynamodb_table.rate_limits.arn]
        }
      ]
    }
    "auth-verify-pin" = {
      description = "Verify PIN and issue JWT tokens"
      timeout     = var.lambda_timeout
      environment = {
        PINS_TABLE               = aws_dynamodb_table.pins.name
        USER_SUBSCRIPTIONS_TABLE = aws_dynamodb_table.user_subscriptions.name
        LICENSE_PUBLIC_KEY       = aws_secretsmanager_secret.license_public_key.arn
        JWT_PRIVATE_KEY          = aws_secretsmanager_secret.jwt_private_key.arn
        RATE_LIMITS_TABLE        = aws_dynamodb_table.rate_limits.name
      }
      iam_policy_statements = [
        {
          effect = "Allow"
          actions = [
            "dynamodb:GetItem",
            "dynamodb:Query",
            "dynamodb:DeleteItem",
          ]
          resources = [
            aws_dynamodb_table.pins.arn,
            aws_dynamodb_table.user_subscriptions.arn
          ]
        },
        {
          effect = "Allow"
          actions = ["secretsmanager:GetSecretValue"]
          resources = [
            aws_secretsmanager_secret.license_public_key.arn,
            aws_secretsmanager_secret.jwt_private_key.arn
          ]
        },
        {
          effect = "Allow"
          actions = [
            "dynamodb:GetItem",
            "dynamodb:UpdateItem"
          ]
          resources = [aws_dynamodb_table.rate_limits.arn]
        }
      ]
    }
    "auth-refresh" = {
      description = "Refresh expired access tokens"
      timeout     = var.lambda_timeout
      environment = {
        LICENSE_PUBLIC_KEY = aws_secretsmanager_secret.license_public_key.arn
        JWT_PRIVATE_KEY    = aws_secretsmanager_secret.jwt_private_key.arn
        JWT_PUBLIC_KEY     = aws_secretsmanager_secret.jwt_public_key.arn
        RATE_LIMITS_TABLE  = aws_dynamodb_table.rate_limits.name
      }
      iam_policy_statements = [
        {
          effect = "Allow"
          actions = ["secretsmanager:GetSecretValue"]
          resources = [
            aws_secretsmanager_secret.license_public_key.arn,
            aws_secretsmanager_secret.jwt_private_key.arn,
            aws_secretsmanager_secret.jwt_public_key.arn
          ]
        },
        {
          effect = "Allow"
          actions = [
            "dynamodb:GetItem",
            "dynamodb:UpdateItem"
          ]
          resources = [aws_dynamodb_table.rate_limits.arn]
        }
      ]
    }
    "auth-logout" = {
      description = "Invalidate user sessions"
      timeout     = var.lambda_timeout
      environment = {
        RATE_LIMITS_TABLE = aws_dynamodb_table.rate_limits.name
      }
      iam_policy_statements = [
        {
          effect = "Allow"
          actions = [
            "dynamodb:GetItem",
            "dynamodb:UpdateItem"
          ]
          resources = [aws_dynamodb_table.rate_limits.arn]
        }
      ]
    }
    
    # Workspace functions
    "workspace-create" = {
      description = "Create new workspaces (enforces per-user workspace limit)"
      timeout     = var.lambda_timeout
      environment = {
        WORKSPACES_TABLE         = aws_dynamodb_table.workspaces.name
        USER_SUBSCRIPTIONS_TABLE = aws_dynamodb_table.user_subscriptions.name
        RATE_LIMITS_TABLE       = aws_dynamodb_table.rate_limits.name
      }
      iam_policy_statements = [
        {
          effect = "Allow"
          actions = [
            "dynamodb:PutItem",
            "dynamodb:Query",
            "dynamodb:GetItem",
            "dynamodb:UpdateItem"
          ]
          resources = [
            aws_dynamodb_table.workspaces.arn,
            "${aws_dynamodb_table.workspaces.arn}/index/*",
            aws_dynamodb_table.user_subscriptions.arn
          ]
        },
        {
          effect = "Allow"
          actions = [
            "dynamodb:GetItem",
            "dynamodb:UpdateItem"
          ]
          resources = [aws_dynamodb_table.rate_limits.arn]
        }
      ]
    }
    "workspace-list" = {
      description = "List accessible workspaces"
      timeout     = var.lambda_timeout
      environment = {
        WORKSPACES_TABLE        = aws_dynamodb_table.workspaces.name
        WORKSPACE_MEMBERS_TABLE = aws_dynamodb_table.workspace_members.name
        RATE_LIMITS_TABLE       = aws_dynamodb_table.rate_limits.name
      }
      iam_policy_statements = [
        {
          effect = "Allow"
          actions = [
            "dynamodb:Query",
            "dynamodb:GetItem"
          ]
          resources = [
            aws_dynamodb_table.workspaces.arn,
            "${aws_dynamodb_table.workspaces.arn}/index/*",
            aws_dynamodb_table.workspace_members.arn,
            "${aws_dynamodb_table.workspace_members.arn}/index/*"
          ]
        },
        {
          effect = "Allow"
          actions = [
            "dynamodb:GetItem",
            "dynamodb:UpdateItem"
          ]
          resources = [aws_dynamodb_table.rate_limits.arn]
        }
      ]
    }
    "workspace-get" = {
      description = "Get workspace details"
      timeout     = var.lambda_timeout
      environment = {
        WORKSPACES_TABLE        = aws_dynamodb_table.workspaces.name
        WORKSPACE_MEMBERS_TABLE = aws_dynamodb_table.workspace_members.name
        RATE_LIMITS_TABLE       = aws_dynamodb_table.rate_limits.name
      }
      iam_policy_statements = [
        {
          effect = "Allow"
          actions = ["dynamodb:GetItem"]
          resources = [
            aws_dynamodb_table.workspaces.arn,
            aws_dynamodb_table.workspace_members.arn
          ]
        },
        {
          effect = "Allow"
          actions = [
            "dynamodb:GetItem",
            "dynamodb:UpdateItem"
          ]
          resources = [aws_dynamodb_table.rate_limits.arn]
        }
      ]
    }
    "workspace-update" = {
      description = "Update workspace name"
      timeout     = var.lambda_timeout
      environment = {
        WORKSPACES_TABLE        = aws_dynamodb_table.workspaces.name
        WORKSPACE_MEMBERS_TABLE = aws_dynamodb_table.workspace_members.name
        RATE_LIMITS_TABLE       = aws_dynamodb_table.rate_limits.name
      }
      iam_policy_statements = [
        {
          effect = "Allow"
          actions = [
            "dynamodb:GetItem",
            "dynamodb:UpdateItem"
          ]
          resources = [aws_dynamodb_table.workspaces.arn]
        },
        {
          effect = "Allow"
          actions = [
            "dynamodb:GetItem",
            "dynamodb:UpdateItem"
          ]
          resources = [aws_dynamodb_table.rate_limits.arn]
        }
      ]
    }
    "workspace-delete" = {
      description = "Delete workspace and all data"
      timeout     = var.lambda_timeout
      environment = {
        WORKSPACES_TABLE        = aws_dynamodb_table.workspaces.name
        WORKSPACE_MEMBERS_TABLE = aws_dynamodb_table.workspace_members.name
        ANNOTATIONS_TABLE       = aws_dynamodb_table.annotations.name
        FILES_TABLE             = aws_dynamodb_table.workspace_files.name
        FILE_LOCATIONS_TABLE    = aws_dynamodb_table.workspace_file_locations.name
        AUDIT_TABLE             = aws_dynamodb_table.audit.name
        RATE_LIMITS_TABLE       = aws_dynamodb_table.rate_limits.name
      }
      iam_policy_statements = [
        {
          effect = "Allow"
          actions = [
            "dynamodb:GetItem",
            "dynamodb:DeleteItem",
            "dynamodb:Query"
          ]
          resources = [
            aws_dynamodb_table.workspaces.arn,
            aws_dynamodb_table.workspace_members.arn,
            aws_dynamodb_table.annotations.arn,
            aws_dynamodb_table.workspace_files.arn,
            aws_dynamodb_table.workspace_file_locations.arn,
            aws_dynamodb_table.audit.arn
          ]
        },
        {
          effect = "Allow"
          actions = [
            "dynamodb:GetItem",
            "dynamodb:UpdateItem"
          ]
          resources = [aws_dynamodb_table.rate_limits.arn]
        }
      ]
    }
    "workspace-convert-to-shared" = {
      description = "Convert to shared workspace"
      timeout     = var.lambda_timeout
      environment = {
        WORKSPACES_TABLE        = aws_dynamodb_table.workspaces.name
        WORKSPACE_MEMBERS_TABLE = aws_dynamodb_table.workspace_members.name
        RATE_LIMITS_TABLE       = aws_dynamodb_table.rate_limits.name
      }
      iam_policy_statements = [
        {
          effect = "Allow"
          actions = [
            "dynamodb:GetItem",
            "dynamodb:UpdateItem"
          ]
          resources = [aws_dynamodb_table.workspaces.arn]
        },
        {
          effect = "Allow"
          actions = [
            "dynamodb:GetItem",
            "dynamodb:UpdateItem"
          ]
          resources = [aws_dynamodb_table.rate_limits.arn]
        }
      ]
    }
    
    # Annotation functions
    "annotation-list" = {
      description = "List annotations with filters"
      timeout     = var.lambda_timeout
      environment = {
        WORKSPACES_TABLE        = aws_dynamodb_table.workspaces.name
        WORKSPACE_MEMBERS_TABLE = aws_dynamodb_table.workspace_members.name
        ANNOTATIONS_TABLE       = aws_dynamodb_table.annotations.name
        RATE_LIMITS_TABLE       = aws_dynamodb_table.rate_limits.name
      }
      iam_policy_statements = [
        {
          effect = "Allow"
          actions = [
            "dynamodb:GetItem",
            "dynamodb:Query"
          ]
          resources = [
            aws_dynamodb_table.workspaces.arn,
            aws_dynamodb_table.workspace_members.arn,
            aws_dynamodb_table.annotations.arn,
            "${aws_dynamodb_table.annotations.arn}/index/*"
          ]
        },
        {
          effect = "Allow"
          actions = [
            "dynamodb:GetItem",
            "dynamodb:UpdateItem"
          ]
          resources = [aws_dynamodb_table.rate_limits.arn]
        }
      ]
    }
    "annotation-get" = {
      description = "Get annotation details"
      timeout     = var.lambda_timeout
      environment = {
        WORKSPACES_TABLE        = aws_dynamodb_table.workspaces.name
        WORKSPACE_MEMBERS_TABLE = aws_dynamodb_table.workspace_members.name
        ANNOTATIONS_TABLE       = aws_dynamodb_table.annotations.name
        RATE_LIMITS_TABLE       = aws_dynamodb_table.rate_limits.name
      }
      iam_policy_statements = [
        {
          effect = "Allow"
          actions = ["dynamodb:GetItem"]
          resources = [
            aws_dynamodb_table.workspaces.arn,
            aws_dynamodb_table.workspace_members.arn,
            aws_dynamodb_table.annotations.arn
          ]
        },
        {
          effect = "Allow"
          actions = [
            "dynamodb:GetItem",
            "dynamodb:UpdateItem"
          ]
          resources = [aws_dynamodb_table.rate_limits.arn]
        }
      ]
    }
    "annotation-create" = {
      description = "Create annotation directly in DynamoDB"
      timeout     = var.lambda_timeout
      environment = {
        WORKSPACES_TABLE        = aws_dynamodb_table.workspaces.name
        ANNOTATIONS_TABLE       = aws_dynamodb_table.annotations.name
        WORKSPACE_MEMBERS_TABLE = aws_dynamodb_table.workspace_members.name
        AUDIT_TABLE             = aws_dynamodb_table.audit.name
        FILES_TABLE             = aws_dynamodb_table.workspace_files.name
        SUBSCRIPTIONS_TABLE     = aws_dynamodb_table.user_subscriptions.name
        PINS_TABLE              = aws_dynamodb_table.pins.name
        FILE_LOCATIONS_TABLE    = aws_dynamodb_table.workspace_file_locations.name
        RATE_LIMITS_TABLE       = aws_dynamodb_table.rate_limits.name
      }
      iam_policy_statements = [
        {
          effect = "Allow"
          actions = ["dynamodb:GetItem", "dynamodb:PutItem", "dynamodb:UpdateItem"]
          resources = [
            aws_dynamodb_table.workspaces.arn,
            aws_dynamodb_table.workspace_members.arn,
            aws_dynamodb_table.annotations.arn,
            aws_dynamodb_table.audit.arn,
            aws_dynamodb_table.rate_limits.arn
          ]
        }
      ]
    }
    "annotation-update" = {
      description = "Update annotation directly in DynamoDB"
      timeout     = var.lambda_timeout
      environment = {
        WORKSPACES_TABLE        = aws_dynamodb_table.workspaces.name
        ANNOTATIONS_TABLE       = aws_dynamodb_table.annotations.name
        WORKSPACE_MEMBERS_TABLE = aws_dynamodb_table.workspace_members.name
        AUDIT_TABLE             = aws_dynamodb_table.audit.name
        FILES_TABLE             = aws_dynamodb_table.workspace_files.name
        SUBSCRIPTIONS_TABLE     = aws_dynamodb_table.user_subscriptions.name
        PINS_TABLE              = aws_dynamodb_table.pins.name
        FILE_LOCATIONS_TABLE    = aws_dynamodb_table.workspace_file_locations.name
        RATE_LIMITS_TABLE       = aws_dynamodb_table.rate_limits.name
      }
      iam_policy_statements = [
        {
          effect = "Allow"
          actions = ["dynamodb:GetItem", "dynamodb:PutItem", "dynamodb:UpdateItem"]
          resources = [
            aws_dynamodb_table.workspaces.arn,
            aws_dynamodb_table.workspace_members.arn,
            aws_dynamodb_table.annotations.arn,
            aws_dynamodb_table.audit.arn,
            aws_dynamodb_table.rate_limits.arn
          ]
        }
      ]
    }
    "annotation-delete" = {
      description = "Delete annotation directly from DynamoDB"
      timeout     = var.lambda_timeout
      environment = {
        WORKSPACES_TABLE        = aws_dynamodb_table.workspaces.name
        ANNOTATIONS_TABLE       = aws_dynamodb_table.annotations.name
        WORKSPACE_MEMBERS_TABLE = aws_dynamodb_table.workspace_members.name
        AUDIT_TABLE             = aws_dynamodb_table.audit.name
        FILES_TABLE             = aws_dynamodb_table.workspace_files.name
        SUBSCRIPTIONS_TABLE     = aws_dynamodb_table.user_subscriptions.name
        PINS_TABLE              = aws_dynamodb_table.pins.name
        FILE_LOCATIONS_TABLE    = aws_dynamodb_table.workspace_file_locations.name
        RATE_LIMITS_TABLE       = aws_dynamodb_table.rate_limits.name
      }
      iam_policy_statements = [
        {
          effect = "Allow"
          actions = ["dynamodb:GetItem", "dynamodb:DeleteItem", "dynamodb:UpdateItem"]
          resources = [
            aws_dynamodb_table.workspaces.arn,
            aws_dynamodb_table.workspace_members.arn,
            aws_dynamodb_table.annotations.arn,
            aws_dynamodb_table.audit.arn,
            aws_dynamodb_table.rate_limits.arn
          ]
        }
      ]
    }

    # File management functions
    "file-list" = {
      description = "List files in workspace"
      timeout     = var.lambda_timeout
      environment = {
        WORKSPACES_TABLE        = aws_dynamodb_table.workspaces.name
        WORKSPACE_MEMBERS_TABLE = aws_dynamodb_table.workspace_members.name
        FILES_TABLE             = aws_dynamodb_table.workspace_files.name
        RATE_LIMITS_TABLE       = aws_dynamodb_table.rate_limits.name
      }
      iam_policy_statements = [
        {
          effect = "Allow"
          actions = [
            "dynamodb:GetItem",
            "dynamodb:Query"
          ]
          resources = [
            aws_dynamodb_table.workspaces.arn,
            aws_dynamodb_table.workspace_members.arn,
            aws_dynamodb_table.workspace_files.arn
          ]
        },
        {
          effect = "Allow"
          actions = [
            "dynamodb:GetItem",
            "dynamodb:UpdateItem"
          ]
          resources = [aws_dynamodb_table.rate_limits.arn]
        }
      ]
    }
    "file-get" = {
      description = "Get file details"
      timeout     = var.lambda_timeout
      environment = {
        WORKSPACES_TABLE        = aws_dynamodb_table.workspaces.name
        WORKSPACE_MEMBERS_TABLE = aws_dynamodb_table.workspace_members.name
        FILES_TABLE             = aws_dynamodb_table.workspace_files.name
        RATE_LIMITS_TABLE       = aws_dynamodb_table.rate_limits.name
      }
      iam_policy_statements = [
        {
          effect = "Allow"
          actions = ["dynamodb:GetItem"]
          resources = [
            aws_dynamodb_table.workspaces.arn,
            aws_dynamodb_table.workspace_members.arn,
            aws_dynamodb_table.workspace_files.arn
          ]
        },
        {
          effect = "Allow"
          actions = [
            "dynamodb:GetItem",
            "dynamodb:UpdateItem"
          ]
          resources = [aws_dynamodb_table.rate_limits.arn]
        }
      ]
    }
    "file-create" = {
      description = "Create file directly in DynamoDB"
      timeout     = var.lambda_timeout
      environment = {
        WORKSPACES_TABLE        = aws_dynamodb_table.workspaces.name
        FILES_TABLE             = aws_dynamodb_table.workspace_files.name
        WORKSPACE_MEMBERS_TABLE = aws_dynamodb_table.workspace_members.name
        AUDIT_TABLE             = aws_dynamodb_table.audit.name
        ANNOTATIONS_TABLE       = aws_dynamodb_table.annotations.name
        SUBSCRIPTIONS_TABLE     = aws_dynamodb_table.user_subscriptions.name
        PINS_TABLE              = aws_dynamodb_table.pins.name
        FILE_LOCATIONS_TABLE    = aws_dynamodb_table.workspace_file_locations.name
        RATE_LIMITS_TABLE       = aws_dynamodb_table.rate_limits.name
      }
      iam_policy_statements = [
        {
          effect = "Allow"
          actions = ["dynamodb:GetItem", "dynamodb:PutItem", "dynamodb:UpdateItem"]
          resources = [
            aws_dynamodb_table.workspaces.arn,
            aws_dynamodb_table.workspace_members.arn,
            aws_dynamodb_table.workspace_files.arn,
            aws_dynamodb_table.audit.arn,
            aws_dynamodb_table.workspace_file_locations.arn,
            aws_dynamodb_table.rate_limits.arn
          ]
        }
      ]
    }
    "file-update" = {
      description = "Update file directly in DynamoDB"
      timeout     = var.lambda_timeout
      environment = {
        WORKSPACES_TABLE        = aws_dynamodb_table.workspaces.name
        FILES_TABLE             = aws_dynamodb_table.workspace_files.name
        WORKSPACE_MEMBERS_TABLE = aws_dynamodb_table.workspace_members.name
        AUDIT_TABLE             = aws_dynamodb_table.audit.name
        ANNOTATIONS_TABLE       = aws_dynamodb_table.annotations.name
        SUBSCRIPTIONS_TABLE     = aws_dynamodb_table.user_subscriptions.name
        PINS_TABLE              = aws_dynamodb_table.pins.name
        FILE_LOCATIONS_TABLE    = aws_dynamodb_table.workspace_file_locations.name
        RATE_LIMITS_TABLE       = aws_dynamodb_table.rate_limits.name
      }
      iam_policy_statements = [
        {
          effect = "Allow"
          actions = ["dynamodb:GetItem", "dynamodb:UpdateItem", "dynamodb:Query", "dynamodb:PutItem"]
          resources = [
            aws_dynamodb_table.workspaces.arn,
            aws_dynamodb_table.workspace_members.arn,
            aws_dynamodb_table.workspace_files.arn,
            "${aws_dynamodb_table.workspace_files.arn}/index/*",
            aws_dynamodb_table.audit.arn,
            aws_dynamodb_table.rate_limits.arn
          ]
        }
      ]
    }
    "file-delete" = {
      description = "Delete file directly from DynamoDB"
      timeout     = var.lambda_timeout
      environment = {
        WORKSPACES_TABLE        = aws_dynamodb_table.workspaces.name
        FILES_TABLE             = aws_dynamodb_table.workspace_files.name
        WORKSPACE_MEMBERS_TABLE = aws_dynamodb_table.workspace_members.name
        AUDIT_TABLE             = aws_dynamodb_table.audit.name
        ANNOTATIONS_TABLE       = aws_dynamodb_table.annotations.name
        SUBSCRIPTIONS_TABLE     = aws_dynamodb_table.user_subscriptions.name
        PINS_TABLE              = aws_dynamodb_table.pins.name
        FILE_LOCATIONS_TABLE    = aws_dynamodb_table.workspace_file_locations.name
        RATE_LIMITS_TABLE       = aws_dynamodb_table.rate_limits.name
      }
      iam_policy_statements = [
        {
          effect = "Allow"
          actions = ["dynamodb:GetItem", "dynamodb:DeleteItem", "dynamodb:UpdateItem", "dynamodb:Query", "dynamodb:PutItem"]
          resources = [
            aws_dynamodb_table.workspaces.arn,
            aws_dynamodb_table.workspace_members.arn,
            aws_dynamodb_table.workspace_files.arn,
            aws_dynamodb_table.audit.arn,
            aws_dynamodb_table.annotations.arn,
            "${aws_dynamodb_table.annotations.arn}/index/*",
            aws_dynamodb_table.workspace_file_locations.arn,
            "${aws_dynamodb_table.workspace_file_locations.arn}/index/*",
            aws_dynamodb_table.rate_limits.arn
          ]
        }
      ]
    }

    # Workspace member management functions
    "workspace-list-members" = {
      description = "List workspace members"
      timeout     = var.lambda_timeout
      environment = {
        WORKSPACES_TABLE        = aws_dynamodb_table.workspaces.name
        WORKSPACE_MEMBERS_TABLE = aws_dynamodb_table.workspace_members.name
        RATE_LIMITS_TABLE       = aws_dynamodb_table.rate_limits.name
      }
      iam_policy_statements = [
        {
          effect = "Allow"
          actions = [
            "dynamodb:GetItem",
            "dynamodb:Query"
          ]
          resources = [
            aws_dynamodb_table.workspaces.arn,
            aws_dynamodb_table.workspace_members.arn
          ]
        },
        {
          effect = "Allow"
          actions = [
            "dynamodb:GetItem",
            "dynamodb:UpdateItem"
          ]
          resources = [aws_dynamodb_table.rate_limits.arn]
        }
      ]
    }
    "workspace-add-member" = {
      description = "Add member (consumes owner's seats)"
      timeout     = var.lambda_timeout
      environment = {
        WORKSPACES_TABLE         = aws_dynamodb_table.workspaces.name
        WORKSPACE_MEMBERS_TABLE  = aws_dynamodb_table.workspace_members.name
        USER_SUBSCRIPTIONS_TABLE = aws_dynamodb_table.user_subscriptions.name
        RATE_LIMITS_TABLE        = aws_dynamodb_table.rate_limits.name
      }
      iam_policy_statements = [
        {
          effect = "Allow"
          actions = [
            "dynamodb:GetItem",
            "dynamodb:PutItem",
            "dynamodb:UpdateItem"
          ]
          resources = [
            aws_dynamodb_table.workspaces.arn,
            aws_dynamodb_table.workspace_members.arn,
            aws_dynamodb_table.user_subscriptions.arn
          ]
        },
        {
          effect = "Allow"
          actions = [
            "dynamodb:GetItem",
            "dynamodb:UpdateItem"
          ]
          resources = [aws_dynamodb_table.rate_limits.arn]
        }
      ]
    }
    "workspace-update-member" = {
      description = "Update member role"
      timeout     = var.lambda_timeout
      environment = {
        WORKSPACES_TABLE        = aws_dynamodb_table.workspaces.name
        WORKSPACE_MEMBERS_TABLE = aws_dynamodb_table.workspace_members.name
        RATE_LIMITS_TABLE       = aws_dynamodb_table.rate_limits.name
      }
      iam_policy_statements = [
        {
          effect = "Allow"
          actions = [
            "dynamodb:GetItem",
            "dynamodb:UpdateItem"
          ]
          resources = [
            aws_dynamodb_table.workspaces.arn,
            aws_dynamodb_table.workspace_members.arn
          ]
        },
        {
          effect = "Allow"
          actions = [
            "dynamodb:GetItem",
            "dynamodb:UpdateItem"
          ]
          resources = [aws_dynamodb_table.rate_limits.arn]
        }
      ]
    }
    "workspace-remove-member" = {
      description = "Remove member"
      timeout     = var.lambda_timeout
      environment = {
        WORKSPACES_TABLE        = aws_dynamodb_table.workspaces.name
        WORKSPACE_MEMBERS_TABLE = aws_dynamodb_table.workspace_members.name
        RATE_LIMITS_TABLE       = aws_dynamodb_table.rate_limits.name
      }
      iam_policy_statements = [
        {
          effect = "Allow"
          actions = [
            "dynamodb:GetItem",
            "dynamodb:DeleteItem",
            "dynamodb:UpdateItem"
          ]
          resources = [
            aws_dynamodb_table.workspaces.arn,
            aws_dynamodb_table.workspace_members.arn
          ]
        },
        {
          effect = "Allow"
          actions = [
            "dynamodb:GetItem",
            "dynamodb:UpdateItem"
          ]
          resources = [aws_dynamodb_table.rate_limits.arn]
        }
      ]
    }

    # File location management functions
    "file-location-store" = {
      description = "Store file location for Breachline instance"
      timeout     = var.lambda_timeout
      environment = {
        WORKSPACES_TABLE         = aws_dynamodb_table.workspaces.name
        ANNOTATIONS_TABLE        = aws_dynamodb_table.annotations.name
        FILES_TABLE              = aws_dynamodb_table.workspace_files.name
        AUDIT_TABLE              = aws_dynamodb_table.audit.name
        SUBSCRIPTIONS_TABLE      = aws_dynamodb_table.user_subscriptions.name
        MEMBERS_TABLE            = aws_dynamodb_table.workspace_members.name
        PINS_TABLE               = aws_dynamodb_table.pins.name
        FILE_LOCATIONS_TABLE     = aws_dynamodb_table.workspace_file_locations.name
        RATE_LIMITS_TABLE        = aws_dynamodb_table.rate_limits.name
      }
      iam_policy_statements = [
        {
          effect = "Allow"
          actions = [
            "dynamodb:PutItem",
            "dynamodb:GetItem"
          ]
          resources = [aws_dynamodb_table.workspace_file_locations.arn]
        },
        {
          effect = "Allow"
          actions = [
            "dynamodb:GetItem",
            "dynamodb:UpdateItem"
          ]
          resources = [aws_dynamodb_table.rate_limits.arn]
        }
      ]
    }
    "file-location-get" = {
      description = "Get file location for Breachline instance"
      timeout     = var.lambda_timeout
      environment = {
        WORKSPACES_TABLE         = aws_dynamodb_table.workspaces.name
        ANNOTATIONS_TABLE        = aws_dynamodb_table.annotations.name
        FILES_TABLE              = aws_dynamodb_table.workspace_files.name
        AUDIT_TABLE              = aws_dynamodb_table.audit.name
        SUBSCRIPTIONS_TABLE      = aws_dynamodb_table.user_subscriptions.name
        MEMBERS_TABLE            = aws_dynamodb_table.workspace_members.name
        PINS_TABLE               = aws_dynamodb_table.pins.name
        FILE_LOCATIONS_TABLE     = aws_dynamodb_table.workspace_file_locations.name
        RATE_LIMITS_TABLE        = aws_dynamodb_table.rate_limits.name
      }
      iam_policy_statements = [
        {
          effect = "Allow"
          actions = ["dynamodb:GetItem"]
          resources = [aws_dynamodb_table.workspace_file_locations.arn]
        },
        {
          effect = "Allow"
          actions = [
            "dynamodb:GetItem",
            "dynamodb:UpdateItem"
          ]
          resources = [aws_dynamodb_table.rate_limits.arn]
        }
      ]
    }
    "file-locations-list" = {
      description = "List all file locations for Breachline instance"
      timeout     = var.lambda_timeout
      environment = {
        WORKSPACES_TABLE         = aws_dynamodb_table.workspaces.name
        ANNOTATIONS_TABLE        = aws_dynamodb_table.annotations.name
        FILES_TABLE              = aws_dynamodb_table.workspace_files.name
        AUDIT_TABLE              = aws_dynamodb_table.audit.name
        SUBSCRIPTIONS_TABLE      = aws_dynamodb_table.user_subscriptions.name
        MEMBERS_TABLE            = aws_dynamodb_table.workspace_members.name
        PINS_TABLE               = aws_dynamodb_table.pins.name
        FILE_LOCATIONS_TABLE     = aws_dynamodb_table.workspace_file_locations.name
        RATE_LIMITS_TABLE        = aws_dynamodb_table.rate_limits.name
      }
      iam_policy_statements = [
        {
          effect = "Allow"
          actions = ["dynamodb:Query"]
          resources = [aws_dynamodb_table.workspace_file_locations.arn]
        },
        {
          effect = "Allow"
          actions = [
            "dynamodb:GetItem",
            "dynamodb:UpdateItem"
          ]
          resources = [aws_dynamodb_table.rate_limits.arn]
        }
      ]
    }
  }
}

# IAM roles for each Lambda function
resource "aws_iam_role" "lambda_function_roles" {
  for_each = local.lambda_functions

  name = "breachline-sync-${each.key}-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "lambda.amazonaws.com"
        }
      }
    ]
  })

  tags = {
    Name = "breachline-sync-${each.key}-role"
  }
}

# Attach basic Lambda execution policy to each role
resource "aws_iam_role_policy_attachment" "lambda_function_basic" {
  for_each = local.lambda_functions

  role       = aws_iam_role.lambda_function_roles[each.key].name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

# Attach X-Ray policy to each role
resource "aws_iam_role_policy_attachment" "lambda_function_xray" {
  for_each = local.lambda_functions

  role       = aws_iam_role.lambda_function_roles[each.key].name
  policy_arn = "arn:aws:iam::aws:policy/AWSXRayDaemonWriteAccess"
}

# Custom IAM policies for each Lambda function
resource "aws_iam_role_policy" "lambda_function_custom" {
  for_each = { for k, v in local.lambda_functions : k => v if lookup(v, "iam_policy_statements", null) != null }

  name = "breachline-sync-${each.key}-custom-policy"
  role = aws_iam_role.lambda_function_roles[each.key].id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      for stmt in each.value.iam_policy_statements : {
        Effect   = stmt.effect
        Action   = stmt.actions
        Resource = stmt.resources
      }
    ]
  })
}

# Create Lambda functions
resource "aws_lambda_function" "functions" {
  for_each = local.lambda_functions

  function_name = "breachline-sync-${each.key}"
  role          = aws_iam_role.lambda_function_roles[each.key].arn
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  architectures = ["arm64"]
  
  filename         = "${path.module}/build/${each.key}.zip"
  source_code_hash = fileexists("${path.module}/build/${each.key}.zip") ? filebase64sha256("${path.module}/build/${each.key}.zip") : null

  memory_size = var.lambda_memory_size
  timeout     = each.value.timeout

  environment {
    variables = each.value.environment
  }

  tracing_config {
    mode = "Active"
  }

  tags = {
    Name = "breachline-sync-${each.key}"
  }
}

# Lambda Authorizer function
resource "aws_lambda_function" "authorizer" {
  function_name = "breachline-sync-authorizer"
  role          = aws_iam_role.lambda_execution.arn
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  architectures = ["arm64"]
  
  filename         = "${path.module}/build/authorizer.zip"
  source_code_hash = fileexists("${path.module}/build/authorizer.zip") ? filebase64sha256("${path.module}/build/authorizer.zip") : null

  memory_size = var.lambda_memory_size
  timeout     = var.lambda_timeout

  environment {
    variables = {
      JWT_PUBLIC_KEY = aws_secretsmanager_secret.jwt_public_key.arn
    }
  }

  tracing_config {
    mode = "Active"
  }

  tags = {
    Name = "breachline-sync-authorizer"
  }
}

# Change processor lambda removed - using direct DynamoDB operations instead

# CloudWatch Log Groups for all Lambda functions
resource "aws_cloudwatch_log_group" "lambda_logs" {
  for_each = local.lambda_functions

  name              = "/aws/lambda/breachline-sync-${each.key}"
  retention_in_days = 30

  tags = {
    Name = "breachline-sync-${each.key}-logs"
  }
}

resource "aws_cloudwatch_log_group" "authorizer_logs" {
  name              = "/aws/lambda/breachline-sync-authorizer"
  retention_in_days = 30

  tags = {
    Name = "breachline-sync-authorizer-logs"
  }
}

# CloudWatch Log Group for sync API (used by rate limiting metric filter)
resource "aws_cloudwatch_log_group" "sync_api" {
  name              = "/aws/apigateway/breachline-sync"
  retention_in_days = 30

  tags = {
    Name = "breachline-sync-api-logs"
  }
}

# Change processor log group removed
