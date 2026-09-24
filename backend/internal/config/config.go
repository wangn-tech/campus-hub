package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	App           AppConfig           `mapstructure:"app"`
	HTTP          HTTPConfig          `mapstructure:"http"`
	MySQL         MySQLConfig         `mapstructure:"mysql"`
	Redis         RedisConfig         `mapstructure:"redis"`
	Kafka         KafkaConfig         `mapstructure:"kafka"`
	Elasticsearch ElasticsearchConfig `mapstructure:"elasticsearch"`
	Storage       StorageConfig       `mapstructure:"storage"`
	JWT           JWTConfig           `mapstructure:"jwt"`
}

type AppConfig struct {
	Name       string `mapstructure:"name"`
	Env        string `mapstructure:"env"`
	Timezone   string `mapstructure:"timezone"`
	InstanceID string `mapstructure:"instance_id"`
}

type HTTPConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	IdleTimeout     time.Duration `mapstructure:"idle_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
	AllowedOrigins  []string      `mapstructure:"allowed_origins"`
}

type MySQLConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Database string `mapstructure:"database"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	Database int    `mapstructure:"database"`
}

type StorageConfig struct {
	Driver       string `mapstructure:"driver"`
	Endpoint     string `mapstructure:"endpoint"`
	AccessKey    string `mapstructure:"access_key"`
	SecretKey    string `mapstructure:"secret_key"`
	Bucket       string `mapstructure:"bucket"`
	UsePathStyle bool   `mapstructure:"use_path_style"`
}

type KafkaConfig struct {
	Brokers                 []string `mapstructure:"brokers"`
	ConsumerGroup           string   `mapstructure:"consumer_group"`
	ChatDeliveryGroupPrefix string   `mapstructure:"chat_delivery_group_prefix"`
}

type ElasticsearchConfig struct {
	Addresses []string `mapstructure:"addresses"`
	Username  string   `mapstructure:"username"`
	Password  string   `mapstructure:"password"`
}

type JWTConfig struct {
	Issuer        string        `mapstructure:"issuer"`
	AccessSecret  string        `mapstructure:"access_secret"`
	AccessTTL     time.Duration `mapstructure:"access_ttl"`
	RefreshSecret string        `mapstructure:"refresh_secret"`
	RefreshTTL    time.Duration `mapstructure:"refresh_ttl"`
}

func Load(path string) (Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetEnvPrefix("CAMPUSHUB")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	v.SetDefault("app.name", "campushub")
	v.SetDefault("app.env", "dev")
	v.SetDefault("app.timezone", "UTC")
	v.SetDefault("app.instance_id", "local")
	v.SetDefault("http.host", "127.0.0.1")
	v.SetDefault("http.port", 8080)
	v.SetDefault("http.read_timeout", "15s")
	v.SetDefault("http.write_timeout", "15s")
	v.SetDefault("http.idle_timeout", "60s")
	v.SetDefault("http.shutdown_timeout", "30s")
	v.SetDefault("http.allowed_origins", []string{"http://localhost:5173"})
	v.SetDefault("mysql.host", "127.0.0.1")
	v.SetDefault("mysql.port", 13306)
	v.SetDefault("mysql.database", "campushub")
	v.SetDefault("redis.host", "127.0.0.1")
	v.SetDefault("redis.port", 16379)
	v.SetDefault("redis.database", 0)
	v.SetDefault("kafka.brokers", []string{"127.0.0.1:19092"})
	v.SetDefault("kafka.consumer_group", "campushub-backend")
	v.SetDefault("kafka.chat_delivery_group_prefix", "campushub.chat-delivery")
	v.SetDefault("elasticsearch.addresses", []string{"http://127.0.0.1:19200"})
	v.SetDefault("storage.driver", "local")
	v.SetDefault("storage.endpoint", "")
	v.SetDefault("storage.bucket", "campushub")
	v.SetDefault("storage.use_path_style", true)
	v.SetDefault("jwt.issuer", "campushub")
	v.SetDefault("jwt.access_ttl", "2h")
	v.SetDefault("jwt.refresh_ttl", "720h")

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return Config{}, fmt.Errorf("read config: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if c.HTTP.Port < 1 || c.HTTP.Port > 65535 {
		return fmt.Errorf("http.port must be between 1 and 65535")
	}
	if c.App.Env == "prod" && (c.JWT.AccessSecret == "" || c.JWT.RefreshSecret == "") {
		return fmt.Errorf("jwt secrets are required in prod")
	}
	if c.MySQL.Database == "" {
		return fmt.Errorf("mysql.database is required")
	}
	if len(c.Kafka.Brokers) == 0 {
		return fmt.Errorf("kafka.brokers is required")
	}
	if len(c.Elasticsearch.Addresses) == 0 {
		return fmt.Errorf("elasticsearch.addresses is required")
	}
	if c.JWT.AccessTTL <= 0 || c.JWT.RefreshTTL <= 0 {
		return fmt.Errorf("jwt token ttl must be positive")
	}
	if c.Storage.Driver != "local" && c.Storage.Driver != "minio" {
		return fmt.Errorf("storage.driver must be local or minio")
	}
	return nil
}
