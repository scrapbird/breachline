package api

import (
	"context"
	"log"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

var (
	logger               *slog.Logger
	ddbClient            *dynamodb.Client
	workspacesTable      string
	annotationsTable     string
	filesTable           string
	auditTable           string
	subscriptionsTable   string
	membersTable         string
	pinsTable            string
	fileLocationsTable   string
)

type InitParams struct {
	Logger               *slog.Logger
	WorkspacesTable      string
	AnnotationsTable     string
	FilesTable           string
	AuditTable           string
	SubscriptionsTable   string
	MembersTable         string
	PinsTable            string
	FileLocationsTable   string
}

func Init(params InitParams) {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}

	ddbClient = dynamodb.NewFromConfig(cfg)

	logger = params.Logger
	workspacesTable = params.WorkspacesTable
	annotationsTable = params.AnnotationsTable
	filesTable = params.FilesTable
	auditTable = params.AuditTable
	subscriptionsTable = params.SubscriptionsTable
	membersTable = params.MembersTable
	pinsTable = params.PinsTable
	fileLocationsTable = params.FileLocationsTable
}
