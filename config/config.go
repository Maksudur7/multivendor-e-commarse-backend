package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all application configuration.
// Values are loaded from .env file and environment variables.
type Config struct {
	App      AppConfig
	Server   ServerConfig
	DB       DatabaseConfig
	Redis    RedisConfig
	JWT      JWTConfig
	CORS     CORSConfig
	Meili    MeiliConfig
	R2       R2Config
	Payment  PaymentConfig
	Courier  CourierConfig
	SMS      SMSConfig
	WhatsApp WhatsAppConfig
	Email    EmailConfig
	FCM      FCMConfig
	Platform PlatformConfig
	Fraud    FraudConfig
}

type AppConfig struct {
	Env  string
	Name string
	Port int
}

type ServerConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

type DatabaseConfig struct {
	URL             string
	Host            string
	Port            int
	User            string
	Password        string
	Name            string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	MaxConns        int
	MinConns        int
	ConnMaxLifetime time.Duration
}

func (db *DatabaseConfig) DSN() string {
	if db.URL != "" {
		return db.URL
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		db.User, db.Password, db.Host, db.Port, db.Name, db.SSLMode)
}

type RedisConfig struct {
	URL        string
	Host       string
	Port       int
	Password   string
	DB         int
	MaxRetries int
}

type JWTConfig struct {
	AccessSecret  string
	RefreshSecret string
	AccessExpiry  time.Duration
	RefreshExpiry time.Duration
}

type CORSConfig struct {
	AllowedOrigins string
}

type MeiliConfig struct {
	Host         string
	MasterKey    string
	ProductIndex string
}

type R2Config struct {
	AccountID       string
	AccessKeyID     string
	SecretAccessKey string
	PublicBucket    string
	PrivateBucket   string
	PublicCDNURL    string
}

type PaymentConfig struct {
	SSLCommerz SSLCommerzConfig
	BKash      BKashConfig
	Nagad      NagadConfig
}

type SSLCommerzConfig struct {
	StoreID       string
	StorePassword string
	IsLive        bool
	SuccessURL    string
	FailURL       string
	CancelURL     string
}

type BKashConfig struct {
	AppKey    string
	AppSecret string
	Username  string
	Password  string
	IsSandbox bool
	BaseURL   string
}

type NagadConfig struct {
	MerchantID string
	PublicKey  string
	PrivateKey string
	IsSandbox  bool
}

type CourierConfig struct {
	Pathao    PathaoConfig
	Steadfast SteadfastConfig
	Paperfly  PaperflyConfig
}

type PathaoConfig struct {
	ClientID     string
	ClientSecret string
	Username     string
	Password     string
}

type SteadfastConfig struct {
	APIKey    string
	SecretKey string
}

type PaperflyConfig struct {
	APIKey string
}

type SMSConfig struct {
	APIKey   string
	SenderID string
}

type WhatsAppConfig struct {
	PhoneNumberID string
	AccessToken   string
}

type EmailConfig struct {
	SMTPHost string
	SMTPPort int
	Username string
	Password string
	From     string
}

type FCMConfig struct {
	ServerKey string
}

type PlatformConfig struct {
	DefaultCommissionRate float64
	EscrowHoldDays        int
	MinWithdrawalAmount   float64
}

type FraudConfig struct {
	BlockScore          float64
	ReviewScore         float64
	MaxAccountsPerIP    int
	OTPRateLimitPerHour int
	CheckoutRateLimit   int
}

// Load reads configuration from .env file and environment variables.
func Load() (*Config, error) {
	viper.SetConfigFile(".env")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("Warning: .env file not found, using environment variables\n")
	}

	dbURL := viper.GetString("DATABASE_URL")
	if dbURL == "" {
		dbURL = viper.GetString("DB_URL")
	}

	redisURL := viper.GetString("REDIS_URL")
	if redisURL == "" && viper.GetString("REDIS_HOST") != "" {
		redisURL = fmt.Sprintf("%s:%d", viper.GetString("REDIS_HOST"), viper.GetInt("REDIS_PORT"))
	}

	cfg := &Config{
		App: AppConfig{
			Env:  viper.GetString("APP_ENV"),
			Name: viper.GetString("APP_NAME"),
			Port: viper.GetInt("APP_PORT"),
		},
		Server: ServerConfig{
			Port:         viper.GetString("APP_PORT"),
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		DB: DatabaseConfig{
			URL:             dbURL,
			Host:            viper.GetString("DB_HOST"),
			Port:            viper.GetInt("DB_PORT"),
			User:            viper.GetString("DB_USER"),
			Password:        viper.GetString("DB_PASSWORD"),
			Name:            viper.GetString("DB_NAME"),
			SSLMode:         viper.GetString("DB_SSL_MODE"),
			MaxConns:        25,
			MinConns:        5,
			MaxOpenConns:    25,
			MaxIdleConns:    5,
			ConnMaxLifetime: 30 * time.Minute,
		},
		Redis: RedisConfig{
			URL:        redisURL,
			Host:       viper.GetString("REDIS_HOST"),
			Port:       viper.GetInt("REDIS_PORT"),
			MaxRetries: 3,
		},
		JWT: JWTConfig{
			AccessSecret:  viper.GetString("JWT_ACCESS_SECRET"),
			RefreshSecret: viper.GetString("JWT_REFRESH_SECRET"),
			AccessExpiry:  15 * time.Minute,
			RefreshExpiry: 168 * time.Hour,
		},
		CORS: CORSConfig{
			AllowedOrigins: viper.GetString("CORS_ALLOWED_ORIGINS"),
		},
	}

	if cfg.App.Port == 0 {
		cfg.App.Port = 8080
	}
	if cfg.CORS.AllowedOrigins == "" {
		cfg.CORS.AllowedOrigins = "*"
	}

	return cfg, nil
}

// LoadConfig alias for Load to accept optional path string
func LoadConfig(path ...string) (*Config, error) {
	return Load()
}
