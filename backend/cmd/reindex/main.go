package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/wangn-tech/campus-hub/internal/config"
	"github.com/wangn-tech/campus-hub/internal/platform/database"
	espkg "github.com/wangn-tech/campus-hub/internal/platform/elasticsearch"
	"github.com/wangn-tech/campus-hub/internal/repository"
	"github.com/wangn-tech/campus-hub/internal/service"
	"go.uber.org/zap"
)

func main() {
	var configPath, target string
	var batchSize int
	flag.StringVar(&configPath, "config", os.Getenv("CAMPUSHUB_CONFIG_FILE"), "config file path")
	flag.StringVar(&target, "index", "", "new physical index name (default: <alias>_v<unix>)")
	flag.IntVar(&batchSize, "batch-size", 200, "MySQL scan batch size")
	flag.Parse()
	if configPath == "" {
		configPath = "configs/config.dev.yaml"
	}
	if err := run(configPath, target, batchSize); err != nil {
		fmt.Fprintln(os.Stderr, "reindex:", err)
		os.Exit(1)
	}
}

func run(configPath, target string, batchSize int) error {
	if batchSize < 1 {
		return fmt.Errorf("batch-size must be positive")
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	db, err := database.Open(cfg.MySQL)
	if err != nil {
		return err
	}
	defer database.Close(db)
	client, err := espkg.Open(cfg.Elasticsearch)
	if err != nil {
		return err
	}
	defer client.Close(context.Background())
	if target == "" {
		target = fmt.Sprintf("%s_v%d", cfg.Elasticsearch.IndexPrefix, time.Now().UTC().Unix())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if err := client.CreateActivityIndex(ctx, target); err != nil {
		return err
	}
	indexer := service.NewActivitySearchIndexer(repository.NewActivityRepository(db), repository.NewCategoryRepository(db), repository.NewUserRepository(db), client, cfg.Elasticsearch.Alias, zap.NewNop())
	result, err := indexer.Rebuild(ctx, target, batchSize)
	if err != nil {
		return fmt.Errorf("build %s after %d documents: %w", target, result.Count, err)
	}
	if err := client.Refresh(ctx, target); err != nil {
		return fmt.Errorf("refresh %s: %w", target, err)
	}
	if count, err := client.Count(ctx, target); err != nil || count != int64(result.Count) {
		if err != nil {
			return fmt.Errorf("count %s: %w", target, err)
		}
		return fmt.Errorf("count %s = %d, want %d", target, count, result.Count)
	}
	for _, id := range result.SampleIDs {
		ok, err := client.HasDocument(ctx, target, id)
		if err != nil || !ok {
			if err != nil {
				return fmt.Errorf("validate sample %s: %w", id, err)
			}
			return fmt.Errorf("validate sample %s: missing", id)
		}
	}
	if err := client.SwitchAlias(ctx, cfg.Elasticsearch.Alias, target); err != nil {
		return fmt.Errorf("switch alias after %d documents: %w", result.Count, err)
	}
	if _, err := indexer.Rebuild(ctx, cfg.Elasticsearch.Alias, batchSize); err != nil {
		return fmt.Errorf("compensate alias: %w", err)
	}
	if err := client.Refresh(ctx, cfg.Elasticsearch.Alias); err != nil {
		return fmt.Errorf("refresh alias after compensation: %w", err)
	}
	fmt.Printf("reindexed %d activities into %s; alias %s switched and compensated\n", result.Count, target, cfg.Elasticsearch.Alias)
	return nil
}
