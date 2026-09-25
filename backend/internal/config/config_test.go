package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
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

func TestLoadConfigEnvironmentOverride(t *testing.T) {
	t.Setenv("CAMPUSHUB_HTTP_PORT", "18082")
	cfg, err := Load(filepath.Join("..", "..", "configs", "config.test.yaml"))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.HTTP.Port != 18082 {
		t.Fatalf("http port override: got %d", cfg.HTTP.Port)
	}
}

func TestProductionRejectsDefaultSecretsAndWildcardCORS(t *testing.T) {
	cfg := Config{
		App:           AppConfig{Env: "prod", Timezone: "Asia/Shanghai"},
		HTTP:          HTTPConfig{Port: 8080, AllowedOrigins: []string{"*"}},
		MySQL:         MySQLConfig{Host: "mysql", Port: 3306, Username: "campushub", Database: "campushub", MaxOpenConns: 10, MaxIdleConns: 2, ConnMaxLifetime: time.Minute},
		Redis:         RedisConfig{Host: "redis", Port: 6379, PoolSize: 10, MinIdleConns: 1, DialTimeout: time.Second, ReadTimeout: time.Second, WriteTimeout: time.Second},
		Kafka:         KafkaConfig{Brokers: []string{"kafka:9092"}, ClientID: "campushub", ProducerAcks: "all"},
		Elasticsearch: ElasticsearchConfig{Addresses: []string{"http://elasticsearch:9200"}, Alias: "activities", IndexPrefix: "campushub", RequestTimeout: time.Second},
		Storage:       StorageConfig{Driver: "local"},
		JWT:           JWTConfig{AccessTTL: time.Hour, RefreshTTL: time.Hour, AccessSecret: "dev-access-secret-change-me", RefreshSecret: "dev-refresh-secret-change-me"},
		Log:           LogConfig{Level: "info", Encoding: "json", Output: "stdout"},
		Observability: ObservabilityConfig{ReadinessTimeout: time.Second},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("production config with default secrets and wildcard CORS must fail")
	}
}
