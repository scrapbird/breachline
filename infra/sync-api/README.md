# BreachLine Sync API

## Overview

The Sync API is a collection of Go Lambda functions that enable team collaboration by synchronizing workspaces and their annotations across multiple machines and users. This API allows teams to work together on the same investigation by maintaining a shared, online representation of their workspaces.

## Purpose

The Sync API provides the backend infrastructure for:

- **User Authentication**: Secure sign-in from within the BreachLine application
- **Workspace Synchronization**: Real-time syncing of workspace changes across team members
- **Annotation Sharing**: Collaborative annotation management across the team
- **Multi-device Support**: Sync the same workspace across multiple machines
- **Conflict Resolution**: Handle concurrent modifications from different team members

## Key Features

- User authentication and session management
- Workspace creation, sharing, and management
- Real-time change synchronization
- Annotation tracking and merging
- Team member management
- Access control and permissions

## Architecture

The Sync API is built using serverless AWS Lambda functions written in Go, providing:

- Scalable, event-driven architecture
- Cost-effective pay-per-use model
- High availability and reliability
- Fast response times with minimal cold starts

## Documentation

For detailed technical documentation, including API specifications, authentication flows, and data models, see the [doc](./doc/) folder.

### Available Documentation

- [API_SPEC.md](./doc/API_SPEC.md) - Complete API endpoint specifications and request/response schemas

## Deployment

Deployment is managed using Terraform. See the terraform files in this directory for infrastructure definitions.

**Note**: All AWS resources are tagged with `project: breachline` and use the `ap-southeast-2` (Sydney) region.

## Development

Each Lambda function is contained in its own subdirectory within the `src/lambda_functions/` folder. Each function includes:

- Go source code
- Handler implementation
- Unit tests
- Function-specific README (if needed)

## State Management

Terraform state is stored in the `scrappy-tfstate` S3 bucket in the `ap-southeast-2` region under the path:
```
sync-api/terraform.tfstate
```

State locking is enabled using a DynamoDB table to prevent concurrent modifications.
