# API Gateway REST API
resource "aws_api_gateway_rest_api" "main" {
  name        = "breachline-sync-api"
  description = "BreachLine Sync API for workspace synchronization"

  endpoint_configuration {
    types = ["REGIONAL"]
  }

  tags = {
    Name = "breachline-sync-api"
  }
}

# API Gateway Account (for CloudWatch logging)
resource "aws_api_gateway_account" "main" {
  cloudwatch_role_arn = aws_iam_role.api_gateway_cloudwatch.arn
}

# Lambda Authorizer
resource "aws_api_gateway_authorizer" "jwt" {
  name            = "jwt-authorizer"
  rest_api_id     = aws_api_gateway_rest_api.main.id
  authorizer_uri  = aws_lambda_function.authorizer.invoke_arn
  type            = "TOKEN"
  identity_source = "method.request.header.Authorization"
  
  # Cache authorization for 5 minutes
  authorizer_result_ttl_in_seconds = 300
}

# Permission for API Gateway to invoke authorizer
resource "aws_lambda_permission" "authorizer" {
  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.authorizer.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_api_gateway_rest_api.main.execution_arn}/*"
}

# API Resources and Methods
locals {
  # Define API structure - using unique keys combining method and path
  api_endpoints = {
    # Auth endpoints (no authorization required)
    "POST-auth/request-pin" = {
      path        = "auth/request-pin"
      lambda      = "auth-request-pin"
      method      = "POST"
      auth        = false
    }
    "POST-auth/verify-pin" = {
      path        = "auth/verify-pin"
      lambda      = "auth-verify-pin"
      method      = "POST"
      auth        = false
    }
    "POST-auth/refresh" = {
      path        = "auth/refresh"
      lambda      = "auth-refresh"
      method      = "POST"
      auth        = false
    }
    # Auth endpoints (authorization required)
    "POST-auth/logout" = {
      path        = "auth/logout"
      lambda      = "auth-logout"
      method      = "POST"
      auth        = true
    }
    
    # Workspace endpoints
    "GET-workspaces" = {
      path        = "workspaces"
      lambda      = "workspace-list"
      method      = "GET"
      auth        = true
    }
    "POST-workspaces" = {
      path        = "workspaces"
      lambda      = "workspace-create"
      method      = "POST"
      auth        = true
    }
    "GET-workspaces/{workspace_id}" = {
      path        = "workspaces/{workspace_id}"
      lambda      = "workspace-get"
      method      = "GET"
      auth        = true
    }
    "PUT-workspaces/{workspace_id}" = {
      path        = "workspaces/{workspace_id}"
      lambda      = "workspace-update"
      method      = "PUT"
      auth        = true
    }
    "DELETE-workspaces/{workspace_id}" = {
      path        = "workspaces/{workspace_id}"
      lambda      = "workspace-delete"
      method      = "DELETE"
      auth        = true
    }
    "POST-workspaces/{workspace_id}/convert-to-shared" = {
      path        = "workspaces/{workspace_id}/convert-to-shared"
      lambda      = "workspace-convert-to-shared"
      method      = "POST"
      auth        = true
    }
    
    # Annotation endpoints
    "GET-workspaces/{workspace_id}/annotations" = {
      path        = "workspaces/{workspace_id}/annotations"
      lambda      = "annotation-list"
      method      = "GET"
      auth        = true
    }
    "GET-workspaces/{workspace_id}/annotations/{annotation_id}" = {
      path        = "workspaces/{workspace_id}/annotations/{annotation_id}"
      lambda      = "annotation-get"
      method      = "GET"
      auth        = true
    }
    "POST-workspaces/{workspace_id}/annotations" = {
      path        = "workspaces/{workspace_id}/annotations"
      lambda      = "annotation-create"
      method      = "POST"
      auth        = true
    }
    "PUT-workspaces/{workspace_id}/annotations/{annotation_id}" = {
      path        = "workspaces/{workspace_id}/annotations/{annotation_id}"
      lambda      = "annotation-update"
      method      = "PUT"
      auth        = true
    }
    "PUT-workspaces/{workspace_id}/annotations" = {
      path        = "workspaces/{workspace_id}/annotations"
      lambda      = "annotation-update"
      method      = "PUT"
      auth        = true
      skip_permissions = true
    }
    "DELETE-workspaces/{workspace_id}/annotations/{annotation_id}" = {
      path        = "workspaces/{workspace_id}/annotations/{annotation_id}"
      lambda      = "annotation-delete"
      method      = "DELETE"
      auth        = true
    }
    "DELETE-workspaces/{workspace_id}/annotations" = {
      path        = "workspaces/{workspace_id}/annotations"
      lambda      = "annotation-delete"
      method      = "DELETE"
      auth        = true
      skip_permissions = true
    }

    # File endpoints
    "GET-workspaces/{workspace_id}/files" = {
      path        = "workspaces/{workspace_id}/files"
      lambda      = "file-list"
      method      = "GET"
      auth        = true
    }
    "GET-workspaces/{workspace_id}/files/{file_hash}" = {
      path        = "workspaces/{workspace_id}/files/{file_hash}"
      lambda      = "file-get"
      method      = "GET"
      auth        = true
    }
    "POST-workspaces/{workspace_id}/files" = {
      path        = "workspaces/{workspace_id}/files"
      lambda      = "file-create"
      method      = "POST"
      auth        = true
    }
    "PUT-workspaces/{workspace_id}/files/{file_hash}" = {
      path        = "workspaces/{workspace_id}/files/{file_hash}"
      lambda      = "file-update"
      method      = "PUT"
      auth        = true
    }
    "DELETE-workspaces/{workspace_id}/files/{file_hash}" = {
      path        = "workspaces/{workspace_id}/files/{file_hash}"
      lambda      = "file-delete"
      method      = "DELETE"
      auth        = true
    }

    # File location endpoints
    "POST-file-locations" = {
      path        = "file-locations"
      lambda      = "file-location-store"
      method      = "POST"
      auth        = true
    }
    "GET-file-locations" = {
      path        = "file-locations"
      lambda      = "file-location-get"
      method      = "GET"
      auth        = true
    }
    "GET-file-locations/all" = {
      path        = "file-locations/all"
      lambda      = "file-locations-list"
      method      = "GET"
      auth        = true
    }

    # Members endpoints
    "GET-workspaces/{workspace_id}/members" = {
      path        = "workspaces/{workspace_id}/members"
      lambda      = "workspace-list-members"
      method      = "GET"
      auth        = true
    }
    "POST-workspaces/{workspace_id}/members" = {
      path        = "workspaces/{workspace_id}/members"
      lambda      = "workspace-add-member"
      method      = "POST"
      auth        = true
    }
    "PUT-workspaces/{workspace_id}/members/{email}" = {
      path        = "workspaces/{workspace_id}/members/{email}"
      lambda      = "workspace-update-member"
      method      = "PUT"
      auth        = true
    }
    "DELETE-workspaces/{workspace_id}/members/{email}" = {
      path        = "workspaces/{workspace_id}/members/{email}"
      lambda      = "workspace-remove-member"
      method      = "DELETE"
      auth        = true
    }
  }
  
  # Filter out endpoints that should skip lambda permissions
  filtered_api_endpoints = {
    for key, endpoint in local.api_endpoints : key => endpoint
    if !try(endpoint.skip_permissions, false)
  }
}

# Helper to extract unique path segments for creating resources
locals {
  # Level 1: Top-level paths (auth, workspaces, sync, annotations, team)
  level1_paths = toset([
    for endpoint in local.api_endpoints : split("/", endpoint.path)[0]
  ])
  
  # Level 2: Second-level paths
  level2_paths = toset([
    for endpoint in local.api_endpoints : 
      join("/", slice(split("/", endpoint.path), 0, 2))
      if length(split("/", endpoint.path)) >= 2
  ])
  
  # Level 3: Third-level paths
  level3_paths = toset([
    for endpoint in local.api_endpoints : 
      join("/", slice(split("/", endpoint.path), 0, 3))
      if length(split("/", endpoint.path)) >= 3
  ])
  
  # Level 4: Fourth-level paths
  level4_paths = toset([
    for endpoint in local.api_endpoints : 
      join("/", slice(split("/", endpoint.path), 0, 4))
      if length(split("/", endpoint.path)) >= 4
  ])
  
  # Level 5: Fifth-level paths
  level5_paths = toset([
    for endpoint in local.api_endpoints : 
      join("/", slice(split("/", endpoint.path), 0, 5))
      if length(split("/", endpoint.path)) >= 5
  ])
}

# Level 1 resources
resource "aws_api_gateway_resource" "level1" {
  for_each = local.level1_paths

  rest_api_id = aws_api_gateway_rest_api.main.id
  parent_id   = aws_api_gateway_rest_api.main.root_resource_id
  path_part   = each.key
}

# Level 2 resources
resource "aws_api_gateway_resource" "level2" {
  for_each = local.level2_paths

  rest_api_id = aws_api_gateway_rest_api.main.id
  parent_id   = aws_api_gateway_resource.level1[split("/", each.key)[0]].id
  path_part   = split("/", each.key)[1]
}

# Level 3 resources
resource "aws_api_gateway_resource" "level3" {
  for_each = local.level3_paths

  rest_api_id = aws_api_gateway_rest_api.main.id
  parent_id   = aws_api_gateway_resource.level2[join("/", slice(split("/", each.key), 0, 2))].id
  path_part   = split("/", each.key)[2]
}

# Level 4 resources
resource "aws_api_gateway_resource" "level4" {
  for_each = local.level4_paths

  rest_api_id = aws_api_gateway_rest_api.main.id
  parent_id   = aws_api_gateway_resource.level3[join("/", slice(split("/", each.key), 0, 3))].id
  path_part   = split("/", each.key)[3]
}

# Level 5 resources
resource "aws_api_gateway_resource" "level5" {
  for_each = local.level5_paths

  rest_api_id = aws_api_gateway_rest_api.main.id
  parent_id   = aws_api_gateway_resource.level4[join("/", slice(split("/", each.key), 0, 4))].id
  path_part   = split("/", each.key)[4]
}

# Helper to get the correct resource for each endpoint
locals {
  endpoint_resources = {
    for key, endpoint in local.api_endpoints : key => (
      length(split("/", endpoint.path)) == 1 ? aws_api_gateway_resource.level1[endpoint.path].id :
      length(split("/", endpoint.path)) == 2 ? aws_api_gateway_resource.level2[endpoint.path].id :
      length(split("/", endpoint.path)) == 3 ? aws_api_gateway_resource.level3[endpoint.path].id :
      length(split("/", endpoint.path)) == 4 ? aws_api_gateway_resource.level4[endpoint.path].id :
      aws_api_gateway_resource.level5[endpoint.path].id
    )
  }
}

# Create methods and integrations for each endpoint
resource "aws_api_gateway_method" "endpoints" {
  for_each = local.api_endpoints

  rest_api_id   = aws_api_gateway_rest_api.main.id
  resource_id   = local.endpoint_resources[each.key]
  http_method   = each.value.method
  authorization = each.value.auth ? "CUSTOM" : "NONE"
  authorizer_id = each.value.auth ? aws_api_gateway_authorizer.jwt.id : null

  request_parameters = {
    "method.request.header.Authorization" = each.value.auth
  }
}

# Lambda integrations
resource "aws_api_gateway_integration" "endpoints" {
  for_each = local.api_endpoints

  rest_api_id             = aws_api_gateway_rest_api.main.id
  resource_id             = local.endpoint_resources[each.key]
  http_method             = aws_api_gateway_method.endpoints[each.key].http_method
  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = aws_lambda_function.functions[each.value.lambda].invoke_arn
}

# Lambda permissions for API Gateway
resource "aws_lambda_permission" "api_gateway" {
  for_each = local.filtered_api_endpoints

  statement_id  = "AllowAPIGatewayInvoke-${each.value.lambda}"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.functions[each.value.lambda].function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_api_gateway_rest_api.main.execution_arn}/*/${each.value.method}/*"
}

# API Gateway Deployment
resource "aws_api_gateway_deployment" "main" {
  rest_api_id = aws_api_gateway_rest_api.main.id

  triggers = {
    redeployment = sha1(jsonencode([
      aws_api_gateway_resource.level1,
      aws_api_gateway_resource.level2,
      aws_api_gateway_resource.level3,
      aws_api_gateway_resource.level4,
      aws_api_gateway_resource.level5,
      aws_api_gateway_method.endpoints,
      aws_api_gateway_integration.endpoints,
    ]))
  }

  lifecycle {
    create_before_destroy = true
  }

  depends_on = [
    aws_api_gateway_method.endpoints,
    aws_api_gateway_integration.endpoints,
  ]
}

# API Gateway Stage
resource "aws_api_gateway_stage" "v1" {
  deployment_id = aws_api_gateway_deployment.main.id
  rest_api_id   = aws_api_gateway_rest_api.main.id
  stage_name    = "v1"

  # Enable X-Ray tracing
  xray_tracing_enabled = true

  # Access logging
  access_log_settings {
    destination_arn = aws_cloudwatch_log_group.api_gateway_access_logs.arn
    format = jsonencode({
      requestId      = "$context.requestId"
      ip             = "$context.identity.sourceIp"
      caller         = "$context.identity.caller"
      user           = "$context.identity.user"
      requestTime    = "$context.requestTime"
      httpMethod     = "$context.httpMethod"
      resourcePath   = "$context.resourcePath"
      status         = "$context.status"
      protocol       = "$context.protocol"
      responseLength = "$context.responseLength"
    })
  }

  tags = {
    Name = "breachline-sync-api-v1"
  }
}

# CloudWatch Log Group for API Gateway access logs
resource "aws_cloudwatch_log_group" "api_gateway_access_logs" {
  name              = "/aws/apigateway/breachline-sync-api"
  retention_in_days = 30

  tags = {
    Name = "breachline-sync-api-access-logs"
  }
}

# Method settings for caching
resource "aws_api_gateway_method_settings" "all" {
  rest_api_id = aws_api_gateway_rest_api.main.id
  stage_name  = aws_api_gateway_stage.v1.stage_name
  method_path = "*/*"

  settings {
    metrics_enabled        = true
    logging_level          = "INFO"
    data_trace_enabled     = false
    throttling_burst_limit = 100 // Allow up to 100 concurrent bursted requests
    throttling_rate_limit  = 50 // Allow 50 requests per second
    
    # Cache settings for GET requests
    caching_enabled = true
    cache_ttl_in_seconds = 60
  }
}

# Usage Plan for rate limiting
resource "aws_api_gateway_usage_plan" "main" {
  name        = "breachline-sync-usage-plan"
  description = "Usage plan for BreachLine Sync API"

  api_stages {
    api_id = aws_api_gateway_rest_api.main.id
    stage  = aws_api_gateway_stage.v1.stage_name
  }

  quota_settings {
    limit  = 100000
    period = "DAY"
  }

  throttle_settings {
    burst_limit = 5000
    rate_limit  = 10000
  }

  tags = {
    Name = "breachline-sync-usage-plan"
  }
}
