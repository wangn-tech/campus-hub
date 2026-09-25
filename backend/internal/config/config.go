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
	Activity      ActivityConfig      `mapstructure:"activity"`
	WebSocket     WebSocketConfig     `mapstructure:"websocket"`
	Mail          MailConfig          `mapstructure:"mail"`
	Security      SecurityConfig      `mapstructure:"security"`
	JWT           JWTConfig           `mapstructure:"jwt"`
	Log           LogConfig           `mapstructure:"log"`
	Observability ObservabilityConfig `mapstructure:"observability"`
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
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	Username        string        `mapstructure:"username"`
	Password        string        `mapstructure:"password"`
	Database        string        `mapstructure:"database"`
	Charset         string        `mapstructure:"charset"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	LogLevel        string        `mapstructure:"log_level"`
}

type RedisConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	Password     string        `mapstructure:"password"`
	Database     int           `mapstructure:"database"`
	PoolSize     int           `mapstructure:"pool_size"`
	MinIdleConns int           `mapstructure:"min_idle_conns"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

type StorageConfig struct {
	Driver       string        `mapstructure:"driver"`
	Endpoint     string        `mapstructure:"endpoint"`
	AccessKey    string        `mapstructure:"access_key"`
	SecretKey    string        `mapstructure:"secret_key"`
	Bucket       string        `mapstructure:"bucket"`
	UsePathStyle bool          `mapstructure:"use_path_style"`
	UseSSL       bool          `mapstructure:"use_ssl"`
	PresignTTL   time.Duration `mapstructure:"presign_ttl"`
	MaxImageSize int64         `mapstructure:"max_image_size"`
}

type ActivityConfig struct {
	// SchedulerInterval drives the periodic activity maintenance tasks:
	// activity status transitions, pending registration expiry.
	SchedulerInterval time.Duration `mapstructure:"scheduler_interval"`
}

// WebSocketConfig tunes the realtime chat endpoint described in the WebSocket
// protocol design document section 3 and section 6.
type WebSocketConfig struct {
	Path              string        `mapstructure:"path"`
	ReadBufferSize    int           `mapstructure:"read_buffer_size"`
	WriteBufferSize   int           `mapstructure:"write_buffer_size"`
	HandshakeTimeout  time.Duration `mapstructure:"handshake_timeout"`
	AuthTimeout       time.Duration `mapstructure:"auth_timeout"`
	HeartbeatInterval time.Duration `mapstructure:"heartbeat_interval"`
	MaxMessageSize    int64         `mapstructure:"max_message_size"`
	AllowedOrigins    []string      `mapstructure:"allowed_origins"`
}

type MailConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	From     string `mapstructure:"from"`
	TLSMode  string `mapstructure:"tls_mode"`
}

type SecurityConfig struct {
	PIIEncryptionKey string `mapstructure:"pii_encryption_key"`
	PIIHashKey       string `mapstructure:"pii_hash_key"`
}

type KafkaConfig struct {
	Brokers                 []string `mapstructure:"brokers"`
	ClientID                string   `mapstructure:"client_id"`
	ProducerAcks            string   `mapstructure:"producer_acks"`
	ProducerBatchSize       int      `mapstructure:"producer_batch_size"`
	ConsumerGroup           string   `mapstructure:"consumer_group"`
	ChatDeliveryGroupPrefix string   `mapstructure:"chat_delivery_group_prefix"`
	RealtimeDeliveryGroupPrefix string   `mapstructure:"realtime_delivery_group_prefix"`
	EnableConsumers         bool     `mapstructure:"enable_consumers"`
	SASLEnabled             bool     `mapstructure:"sasl_enabled"`
	Username                string   `mapstructure:"username"`
	Password                string   `mapstructure:"password"`
}

type ElasticsearchConfig struct {
	Addresses      []string      `mapstructure:"addresses"`
	Username       string        `mapstructure:"username"`
	Password       string        `mapstructure:"password"`
	Alias          string        `mapstructure:"alias"`
	IndexPrefix    string        `mapstructure:"index_prefix"`
	EnableSearch   bool          `mapstructure:"enable_search"`
	RequestTimeout time.Duration `mapstructure:"request_timeout"`
}

type LogConfig struct {
	Level    string `mapstructure:"level"`
	Encoding string `mapstructure:"encoding"`
	Output   string `mapstructure:"output"`
}

type ObservabilityConfig struct {
	ReadinessTimeout time.Duration `mapstructure:"readiness_timeout"`
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
	v.SetDefault("app.timezone", "Asia/Shanghai")
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
	v.SetDefault("mysql.username", "campushub")
	v.SetDefault("mysql.charset", "utf8mb4")
	v.SetDefault("mysql.max_open_conns", 50)
	v.SetDefault("mysql.max_idle_conns", 10)
	v.SetDefault("mysql.conn_max_lifetime", "30m")
	v.SetDefault("mysql.log_level", "warn")
	v.SetDefault("mysql.database", "campushub")
	v.SetDefault("redis.host", "127.0.0.1")
	v.SetDefault("redis.port", 16379)
	v.SetDefault("redis.database", 0)
	v.SetDefault("redis.pool_size", 50)
	v.SetDefault("redis.min_idle_conns", 5)
	v.SetDefault("redis.dial_timeout", "5s")
	v.SetDefault("redis.read_timeout", "3s")
	v.SetDefault("redis.write_timeout", "3s")
	v.SetDefault("kafka.brokers", []string{"127.0.0.1:19092"})
	v.SetDefault("kafka.client_id", "campushub-backend")
	v.SetDefault("kafka.producer_acks", "all")
	v.SetDefault("kafka.producer_batch_size", 100)
	v.SetDefault("kafka.consumer_group", "campushub-backend")
	v.SetDefault("kafka.chat_delivery_group_prefix", "campushub.chat-delivery")
	v.SetDefault("kafka.enable_consumers", true)
	v.SetDefault("kafka.realtime_delivery_group_prefix", "campushub.realtime-delivery")
	v.SetDefault("kafka.sasl_enabled", false)
	v.SetDefault("elasticsearch.addresses", []string{"http://127.0.0.1:19200"})
	v.SetDefault("elasticsearch.alias", "activities")
	v.SetDefault("elasticsearch.index_prefix", "campushub")
	v.SetDefault("elasticsearch.enable_search", true)
	v.SetDefault("elasticsearch.request_timeout", "10s")
	v.SetDefault("storage.driver", "rustfs")
	v.SetDefault("storage.endpoint", "127.0.0.1:19000")
	v.SetDefault("storage.access_key", "rustfsadmin")
	v.SetDefault("storage.secret_key", "change-me")
	v.SetDefault("storage.bucket", "campushub")
	v.SetDefault("storage.use_path_style", true)
	v.SetDefault("storage.presign_ttl", "15m")
	v.SetDefault("storage.max_image_size", 5242880)
	v.SetDefault("activity.scheduler_interval", "1m")
	v.SetDefault("websocket.path", "/ws")
	v.SetDefault("websocket.read_buffer_size", 4096)
	v.SetDefault("websocket.write_buffer_size", 4096)
	v.SetDefault("websocket.handshake_timeout", "10s")
	v.SetDefault("websocket.auth_timeout", "10s")
	v.SetDefault("websocket.heartbeat_interval", "45s")
	v.SetDefault("websocket.max_message_size", 4096)
	v.SetDefault("websocket.allowed_origins", []string{"http://localhost:5173"})
	v.SetDefault("mail.host", "127.0.0.1")
	v.SetDefault("mail.port", 1025)
	v.SetDefault("mail.from", "noreply@campushub.local")
	v.SetDefault("mail.tls_mode", "none")
	v.SetDefault("security.pii_encryption_key", "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	v.SetDefault("security.pii_hash_key", "campushub-dev-pii-hash-key")
	v.SetDefault("jwt.issuer", "campushub")
	v.SetDefault("jwt.access_ttl", "2h")
	v.SetDefault("jwt.refresh_ttl", "720h")
	v.SetDefault("log.level", "info")
	v.SetDefault("log.encoding", "json")
	v.SetDefault("log.output", "stdout")
	v.SetDefault("observability.readiness_timeout", "3s")

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
	if c.App.Env != "dev" && c.App.Env != "test" && c.App.Env != "prod" {
		return fmt.Errorf("app.env must be dev, test, or prod")
	}
	if _, err := time.LoadLocation(c.App.Timezone); err != nil {
		return fmt.Errorf("app.timezone is invalid: %w", err)
	}
	if c.HTTP.Port < 1 || c.HTTP.Port > 65535 {
		return fmt.Errorf("http.port must be between 1 and 65535")
	}
	if c.MySQL.Host == "" || c.MySQL.Username == "" || c.MySQL.Database == "" || c.MySQL.Port < 1 || c.MySQL.Port > 65535 {
		return fmt.Errorf("mysql host, port, username, and database are required")
	}
	if c.MySQL.MaxOpenConns < 1 || c.MySQL.MaxIdleConns < 0 || c.MySQL.MaxIdleConns > c.MySQL.MaxOpenConns || c.MySQL.ConnMaxLifetime <= 0 {
		return fmt.Errorf("mysql connection pool configuration is invalid")
	}
	if c.MySQL.LogLevel != "silent" && c.MySQL.LogLevel != "error" && c.MySQL.LogLevel != "warn" && c.MySQL.LogLevel != "info" {
		return fmt.Errorf("mysql.log_level is invalid")
	}
	if c.Redis.Host == "" || c.Redis.Port < 1 || c.Redis.Port > 65535 {
		return fmt.Errorf("redis host and port are required")
	}
	if c.Redis.PoolSize < 1 || c.Redis.MinIdleConns < 0 || c.Redis.MinIdleConns > c.Redis.PoolSize || c.Redis.DialTimeout <= 0 || c.Redis.ReadTimeout <= 0 || c.Redis.WriteTimeout <= 0 {
		return fmt.Errorf("redis connection configuration is invalid")
	}
	if len(c.Kafka.Brokers) == 0 {
		return fmt.Errorf("kafka.brokers is required")
	}
	if c.Kafka.ClientID == "" || c.Kafka.ProducerBatchSize < 1 || (c.Kafka.ProducerAcks != "all" && c.Kafka.ProducerAcks != "leader" && c.Kafka.ProducerAcks != "none") {
		return fmt.Errorf("kafka client_id, producer_acks, and producer_batch_size are invalid")
	}
	if c.Kafka.SASLEnabled && (c.Kafka.Username == "" || c.Kafka.Password == "") {
		return fmt.Errorf("kafka sasl username and password are required when sasl is enabled")
	}
	if len(c.Elasticsearch.Addresses) == 0 {
		return fmt.Errorf("elasticsearch.addresses is required")
	}
	if c.Elasticsearch.Alias == "" || c.Elasticsearch.IndexPrefix == "" || c.Elasticsearch.RequestTimeout <= 0 {
		return fmt.Errorf("elasticsearch alias, index_prefix, and request_timeout are required")
	}
	if c.JWT.AccessTTL <= 0 || c.JWT.RefreshTTL <= 0 {
		return fmt.Errorf("jwt token ttl must be positive")
	}
	if c.Storage.Driver != "rustfs" || c.Storage.Endpoint == "" || c.Storage.AccessKey == "" || c.Storage.SecretKey == "" || c.Storage.Bucket == "" || c.Storage.PresignTTL <= 0 || c.Storage.MaxImageSize <= 0 {
		return fmt.Errorf("valid rustfs storage configuration is required")
	}
	if c.Activity.SchedulerInterval <= 0 {
		return fmt.Errorf("activity.scheduler_interval must be positive")
	}
	if !strings.HasPrefix(c.WebSocket.Path, "/") {
		return fmt.Errorf("websocket.path must start with /")
	}
	if c.WebSocket.ReadBufferSize < 1 || c.WebSocket.WriteBufferSize < 1 || c.WebSocket.MaxMessageSize < 1 ||
		c.WebSocket.HandshakeTimeout <= 0 || c.WebSocket.AuthTimeout <= 0 || c.WebSocket.HeartbeatInterval <= 0 {
		return fmt.Errorf("websocket buffer sizes, timeouts, and max_message_size must be positive")
	}
	if c.Mail.Host == "" || c.Mail.Port < 1 || c.Mail.Port > 65535 || c.Mail.From == "" || (c.Mail.TLSMode != "none" && c.Mail.TLSMode != "starttls") {
		return fmt.Errorf("mail configuration is invalid")
	}
	if c.Security.PIIEncryptionKey == "" || c.Security.PIIHashKey == "" {
		return fmt.Errorf("security pii keys are required")
	}
	if c.Log.Level != "debug" && c.Log.Level != "info" && c.Log.Level != "warn" && c.Log.Level != "error" {
		return fmt.Errorf("log.level is invalid")
	}
	if c.Log.Encoding != "json" && c.Log.Encoding != "console" {
		return fmt.Errorf("log.encoding must be json or console")
	}
	if c.Log.Output == "" || c.Observability.ReadinessTimeout <= 0 {
		return fmt.Errorf("log.output and observability.readiness_timeout are required")
	}
	if c.App.Env == "prod" {
		if insecureSecret(c.JWT.AccessSecret) || insecureSecret(c.JWT.RefreshSecret) {
			return fmt.Errorf("non-default jwt secrets are required in prod")
		}
		if insecureSecret(c.Storage.SecretKey) || c.Mail.TLSMode != "starttls" || strings.HasPrefix(c.Security.PIIEncryptionKey, "MDEy") || strings.HasPrefix(c.Security.PIIHashKey, "campushub-dev-") {
			return fmt.Errorf("production storage, mail TLS, and pii keys must be secure")
		}
		for _, origin := range c.HTTP.AllowedOrigins {
			if strings.TrimSpace(origin) == "*" {
				return fmt.Errorf("http.allowed_origins cannot contain * in prod")
			}
		}
	}
	return nil
}

func insecureSecret(secret string) bool {
	trimmed := strings.TrimSpace(secret)
	return trimmed == "" || strings.HasPrefix(trimmed, "change-me") || strings.HasPrefix(trimmed, "dev-") || strings.HasPrefix(trimmed, "test-")
}
