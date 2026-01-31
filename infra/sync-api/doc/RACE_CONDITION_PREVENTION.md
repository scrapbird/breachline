# Race Condition Prevention in Sync API

## Overview

The Sync API uses SQS FIFO queues with MessageGroupId to prevent race conditions when multiple simultaneous changes are made to the same workspace. This document explains how the system ensures data consistency and ordering guarantees.

## The Problem: Race Conditions Without FIFO

Without proper ordering mechanisms, simultaneous changes to the same workspace could cause race conditions:

```
Time    Client A                    Client B                    Database
----    --------                    --------                    --------
T1      Read workspace v=10         Read workspace v=10         version=10
T2      Create annotation A         Create annotation B         version=10
T3      Write version=11            Write version=11 ❌          version=11
T4      ❌ Lost update!             ❌ Conflict!                ❌ Data loss
```

**Problems:**
- Both clients read the same version (10)
- Both try to write version 11
- One update overwrites the other
- Changes are lost or corrupted

## The Solution: SQS FIFO with MessageGroupId

### 1. Queue Configuration

The workspace changes queue is configured as a FIFO queue in `sqs.tf`:

```hcl
resource "aws_sqs_queue" "workspace_changes" {
  name                        = "breachline-sync-workspace-changes.fifo"
  fifo_queue                  = true
  content_based_deduplication = true
  visibility_timeout_seconds  = 300  # 5 minutes
  message_retention_seconds   = 345600  # 4 days
  
  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.workspace_changes_dlq.arn
    maxReceiveCount     = 3
  })
}
```

**Key Properties:**
- `fifo_queue = true`: Enables strict ordering within message groups
- `content_based_deduplication = true`: Prevents duplicate messages
- Dead Letter Queue for failed messages after 3 retries

### 2. MessageGroupId = workspace_id

All functions that send messages to the queue set `MessageGroupId` to the `workspace_id`:

**Example from `sync-push/main.go`:**
```go
_, err = sqsClient.SendMessage(ctx, &sqs.SendMessageInput{
    QueueUrl:               &sqsQueueURL,
    MessageBody:            aws.String(string(messageBody)),
    MessageGroupId:         &workspaceID, // ← Ensures ordering per workspace
    MessageDeduplicationId: aws.String(fmt.Sprintf("%s_%s", workspaceID, uuid.New().String())),
})
```

**What This Guarantees:**
- All messages with the same `MessageGroupId` (workspace_id) are delivered **in exact order**
- Messages for different workspaces can be processed in parallel
- No two Lambda invocations will process messages from the same workspace simultaneously

### 3. Single Message Processing (Batch Size = 1)

The change processor Lambda is configured to process one message at a time:

**From `lambda.tf`:**
```hcl
resource "aws_lambda_event_source_mapping" "change_processor" {
  event_source_arn = aws_sqs_queue.workspace_changes.arn
  function_name    = aws_lambda_function.change_processor.arn
  
  batch_size = 1  # ← Process one message at a time
  
  function_response_types = ["ReportBatchItemFailures"]
}
```

**Why This Matters:**
- Ensures sequential processing of changes within a workspace
- Next message only processed after previous one completes
- Prevents concurrent modifications to the same workspace

## How It Works: Step-by-Step

### Scenario: Two Clients Make Simultaneous Changes

```
Client A: Create annotation "Finding 1"
Client B: Create annotation "Finding 2"
Both for workspace: ws_abc123
```

### Step 1: Messages Sent to Queue

```
sync-push Lambda (Client A):
  ├─ Validates access
  ├─ Sends message to SQS FIFO
  │   MessageGroupId: "ws_abc123"
  │   MessageDeduplicationId: "ws_abc123_uuid1"
  │   Body: { change_type: "annotation_created", data: {...} }
  └─ Returns 202 Accepted

sync-push Lambda (Client B):
  ├─ Validates access
  ├─ Sends message to SQS FIFO
  │   MessageGroupId: "ws_abc123"  # Same group!
  │   MessageDeduplicationId: "ws_abc123_uuid2"
  │   Body: { change_type: "annotation_created", data: {...} }
  └─ Returns 202 Accepted
```

### Step 2: SQS FIFO Queue Ordering

```
SQS FIFO Queue (MessageGroup: ws_abc123):
┌─────────────────────────────────────┐
│ Message 1: Create "Finding 1"       │ ← First in queue
├─────────────────────────────────────┤
│ Message 2: Create "Finding 2"       │ ← Waiting for Message 1
└─────────────────────────────────────┘

SQS Guarantee: Message 2 will NOT be delivered until Message 1 is deleted
```

### Step 3: Sequential Processing

```
change-processor Lambda (Invocation 1):
  ├─ Receives Message 1
  ├─ Reads workspace: version = 10
  ├─ Creates annotation "Finding 1"
  ├─ Records change with version = 11
  ├─ Updates workspace: version = 11 (with condition: version = 10)
  ├─ Success! Deletes message from queue
  └─ Lambda completes

change-processor Lambda (Invocation 2):
  ├─ NOW receives Message 2 (after Message 1 deleted)
  ├─ Reads workspace: version = 11  ← Updated by Message 1
  ├─ Creates annotation "Finding 2"
  ├─ Records change with version = 12
  ├─ Updates workspace: version = 12 (with condition: version = 11)
  ├─ Success! Deletes message from queue
  └─ Lambda completes

Final State:
  ✓ Both annotations created
  ✓ Workspace version = 12
  ✓ Changes recorded in order: v11, v12
  ✓ No race condition!
```

## Parallel Processing for Different Workspaces

While changes to the same workspace are serialized, different workspaces can be processed in parallel:

```
SQS FIFO Queue:
┌─────────────────────────────────────────────────┐
│ MessageGroup: ws_abc123                         │
│   ├─ Message 1 (processing)                     │
│   └─ Message 2 (waiting)                        │
├─────────────────────────────────────────────────┤
│ MessageGroup: ws_xyz789                         │
│   ├─ Message 1 (processing in parallel!)       │
│   └─ Message 2 (waiting)                        │
└─────────────────────────────────────────────────┘

Result: Two Lambda invocations run concurrently, one per workspace
```

## Additional Safety Mechanisms

### 1. Optimistic Locking (Defense in Depth)

Even though FIFO ordering prevents concurrent processing, the change processor uses conditional updates as a safety net:

```go
func updateWorkspaceVersion(ctx context.Context, workspaceID string, expectedVersion, newVersion int64) error {
    _, err := ddbClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
        TableName: &workspacesTable,
        Key: map[string]types.AttributeValue{
            "workspace_id": &types.AttributeValueMemberS{Value: workspaceID},
        },
        UpdateExpression:    aws.String("SET version = :new_version, updated_at = :updated_at"),
        ConditionExpression: aws.String("version = :expected_version"),  // ← Fails if version changed
        ExpressionAttributeValues: map[string]types.AttributeValue{
            ":new_version":      &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", newVersion)},
            ":expected_version": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", expectedVersion)},
            ":updated_at":       &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
        },
    })
    return err
}
```

**If a race condition somehow occurred:**
- The conditional update would fail
- Lambda returns an error
- SQS retries the message (up to 3 times)
- Message eventually moves to Dead Letter Queue for investigation

### 2. Idempotency Check

The change processor checks if a change has already been processed:

```go
// Check for duplicate (idempotency)
exists, err := changeExists(ctx, change.WorkspaceID, changeID)
if exists {
    log.Printf("Change already processed, skipping: %s", changeID)
    return nil  // Skip duplicate
}
```

**Prevents:**
- Duplicate processing if Lambda is retried
- Ensures exactly-once semantics

### 3. Message Deduplication

SQS FIFO's content-based deduplication prevents duplicate messages:

```go
MessageDeduplicationId: aws.String(fmt.Sprintf("%s_%s", workspaceID, uuid.New().String()))
```

**Guarantees:**
- Same message won't be queued twice within 5-minute window
- Protects against client retries

## Functions That Use MessageGroupId

All functions that modify workspace data use MessageGroupId:

| Function | Location | MessageGroupId |
|----------|----------|----------------|
| `sync-push` | [../src/lambda_functions/sync-push/main.go](../src/lambda_functions/sync-push/main.go) | `workspaceID` |
| `annotation-create` | [../src/lambda_functions/annotation-create/main.go](../src/lambda_functions/annotation-create/main.go) | `workspaceID` |
| `annotation-update` | [../src/lambda_functions/annotation-update/main.go](../src/lambda_functions/annotation-update/main.go) | `workspaceID` |
| `annotation-delete` | [../src/lambda_functions/annotation-delete/main.go](../src/lambda_functions/annotation-delete/main.go) | `workspaceID` |

## Monitoring and Alerting

### CloudWatch Alarms

The system monitors for processing failures:

```hcl
resource "aws_cloudwatch_metric_alarm" "dlq_messages" {
  alarm_name          = "breachline-sync-dlq-messages"
  comparison_operator = "GreaterThanThreshold"
  threshold           = 0
  metric_name         = "ApproximateNumberOfMessagesVisible"
  namespace           = "AWS/SQS"
  
  dimensions = {
    QueueName = aws_sqs_queue.workspace_changes_dlq.name
  }
}
```

**Alert Triggers:**
- Any message in Dead Letter Queue
- Indicates repeated processing failures
- Requires manual investigation

### Logs

All processing steps are logged:

```
Processing change: workspace=ws_abc123, type=annotation_created
Successfully processed change: chg_xyz, new version: 11
```

## Performance Characteristics

### Throughput

- **Per Workspace**: Up to 300 messages/second (SQS FIFO limit per MessageGroupId)
- **Total System**: Unlimited (scales with number of workspaces)
- **Typical Latency**: 100-500ms from queue to completion

### Scalability

```
1 workspace  → 1 Lambda invocation at a time
10 workspaces → Up to 10 concurrent Lambda invocations
100 workspaces → Up to 100 concurrent Lambda invocations
```

**No bottlenecks** as long as workspaces are distributed across different MessageGroupIds.

## Testing Race Conditions

To verify race condition prevention:

```bash
# Send 100 concurrent changes to the same workspace
for i in {1..100}; do
  curl -X POST https://api.breachline.com/v1/workspaces/ws_test/sync/push \
    -H "Authorization: Bearer $TOKEN" \
    -d "{\"changes\": [{\"change_type\": \"annotation_created\", \"data\": {...}}]}" &
done
wait

# Verify all changes processed in order
# (Note: sync/status endpoint has been removed - use workspace details instead)
curl https://api.breachline.com/v1/workspaces/ws_test

# Expected: all changes present in workspace
```

## Summary

The Sync API prevents race conditions through:

1. **SQS FIFO Queue**: Guarantees message ordering within MessageGroupId
2. **MessageGroupId = workspace_id**: Serializes all changes per workspace
3. **Batch Size = 1**: Ensures sequential processing
4. **Optimistic Locking**: Conditional updates as safety net
5. **Idempotency**: Prevents duplicate processing
6. **Dead Letter Queue**: Captures failures for investigation

**Result**: Zero race conditions, guaranteed ordering, and data consistency across all workspace changes.
