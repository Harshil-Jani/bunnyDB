package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/bunnydb/bunnydb/flow/activities"
	"github.com/bunnydb/bunnydb/flow/api"
	"github.com/bunnydb/bunnydb/flow/shared"
	"github.com/bunnydb/bunnydb/flow/workflows"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: bunny <command>")
		fmt.Println("Commands:")
		fmt.Println("  api     - Start the API server")
		fmt.Println("  worker  - Start the Temporal worker")
		os.Exit(1)
	}

	command := os.Args[1]

	// Load configuration
	config, err := shared.LoadConfig()
	if err != nil {
		slog.Error("failed to load config", slog.Any("error", err))
		os.Exit(1)
	}

	// Connect to catalog database
	catalogPool, err := pgxpool.New(context.Background(), config.CatalogConnectionString())
	if err != nil {
		slog.Error("failed to connect to catalog", slog.Any("error", err))
		os.Exit(1)
	}
	defer catalogPool.Close()

	// Create Temporal client
	temporalClient, err := client.Dial(client.Options{
		HostPort:  config.TemporalHostPort,
		Namespace: config.TemporalNamespace,
	})
	if err != nil {
		slog.Error("failed to create Temporal client", slog.Any("error", err))
		os.Exit(1)
	}
	defer temporalClient.Close()

	switch command {
	case "api":
		runAPI(config, catalogPool, temporalClient)
	case "worker":
		runWorker(config, catalogPool, temporalClient)
	default:
		fmt.Printf("Unknown command: %s\n", command)
		os.Exit(1)
	}
}

func runAPI(config *shared.Config, catalogPool *pgxpool.Pool, temporalClient client.Client) {
	slog.Info("starting BunnyDB API server")

	// Ensure auth tables exist and seed admin user
	api.EnsureUsersTable(catalogPool)
	api.SeedAdmin(catalogPool, config)

	handler := api.NewHandler(temporalClient, catalogPool, config)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	server := &http.Server{
		Addr:    ":8112",
		Handler: mux,
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		slog.Info("shutting down API server")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()

	slog.Info("API server listening", slog.String("addr", server.Addr))
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		slog.Error("API server error", slog.Any("error", err))
		os.Exit(1)
	}
}

func runWorker(config *shared.Config, catalogPool *pgxpool.Pool, temporalClient client.Client) {
	slog.Info("starting BunnyDB worker")

	// Create worker
	w := worker.New(temporalClient, config.WorkerTaskQueue, worker.Options{
		MaxConcurrentActivityExecutionSize:     10,
		MaxConcurrentWorkflowTaskExecutionSize: 10,
	})

	// Register workflows
	w.RegisterWorkflow(workflows.CDCFlowWorkflow)
	w.RegisterWorkflow(workflows.SnapshotFlowWorkflow)
	w.RegisterWorkflow(workflows.CloneTableWorkflow)
	w.RegisterWorkflow(workflows.TableResyncWorkflow)
	w.RegisterWorkflow(workflows.DropFlowWorkflow)
	w.RegisterWorkflow(workflows.FullSwapResyncWorkflow)

	// Register activities
	acts := activities.NewActivities(catalogPool, config)
	w.RegisterActivity(acts.SetupMirror)
	w.RegisterActivity(acts.SyncFlow)
	w.RegisterActivity(acts.DropForeignKeys)
	w.RegisterActivity(acts.RecreateForeignKeys)
	w.RegisterActivity(acts.CreateIndexes)
	w.RegisterActivity(acts.CopyTable)
	w.RegisterActivity(acts.UpdateTableSyncStatus)
	w.RegisterActivity(acts.DropSourceReplication)
	w.RegisterActivity(acts.CleanupCatalog)
	w.RegisterActivity(acts.TruncateTable)
	w.RegisterActivity(acts.ExportSnapshot)
	w.RegisterActivity(acts.DropTableForeignKeys)
	w.RegisterActivity(acts.CreateTableIndexes)
	w.RegisterActivity(acts.RecreateTableForeignKeys)
	w.RegisterActivity(acts.DropDestinationTables)
	w.RegisterActivity(acts.GetPartitionInfo)
	w.RegisterActivity(acts.CopyPartition)
	w.RegisterActivity(acts.StartSnapshotSession)
	w.RegisterActivity(acts.HoldSnapshotSession)
	w.RegisterActivity(acts.EndSnapshotSession)
	w.RegisterActivity(acts.SyncSchema)
	w.RegisterActivity(acts.CreateResyncTable)
	w.RegisterActivity(acts.SwapTables)
	w.RegisterActivity(acts.DropResyncTable)

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		slog.Info("shutting down worker")
	}()

	slog.Info("worker listening", slog.String("taskQueue", config.WorkerTaskQueue))
	if err := w.Run(worker.InterruptCh()); err != nil {
		slog.Error("worker error", slog.Any("error", err))
		os.Exit(1)
	}
}
