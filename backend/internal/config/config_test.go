package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte("app:\n  env: test\nhttp:\n  port: 18081\nmysql:\n  database: testdb\nkafka:\n  brokers: [localhost:19092]\nelasticsearch:\n  addresses: [http://localhost:19200]\njwt:\n  access_secret: access\n  refresh_secret: refresh\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.HTTP.Port != 18081 || cfg.MySQL.Database != "testdb" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}
