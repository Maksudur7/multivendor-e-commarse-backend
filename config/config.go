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
	Google   GoogleOAuthConfig
}

type AppConfig struct {
	Env     string
	Name    string
	Port    int
	BaseURL string // e.g. https://api.yourdomain.com — used for email verification links
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

// GoogleOAuthConfig holds Google OAuth 2.0 credentials.
type GoogleOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
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
	InstanceID    string
	Token         string
	PhoneNumberID string
	AccessToken   string
}

type EmailConfig struct {
	SMTPHost string
	SMTPPort int
	Username string
	Password string
	From     string
	APIKey   string
	Provider string
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
		redisPass := viper.GetString("REDIS_PASSWORD")
		redisHost := viper.GetString("REDIS_HOST")
		redisPort := viper.GetInt("REDIS_PORT")
		if redisPort == 0 {
			redisPort = 6379
		}
		if redisPass != "" {
			redisURL = fmt.Sprintf("redis://:%s@%s:%d", redisPass, redisHost, redisPort)
		} else {
			redisURL = fmt.Sprintf("redis://%s:%d", redisHost, redisPort)
		}
	}

	cfg := &Config{
		App: AppConfig{
			Env:     viper.GetString("APP_ENV"),
			Name:    viper.GetString("APP_NAME"),
			Port:    viper.GetInt("APP_PORT"),
			BaseURL: viper.GetString("APP_BASE_URL"),
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
		SMS: SMSConfig{
			APIKey:   viper.GetString("SMS_API_KEY"),
			SenderID: viper.GetString("SMS_SENDER_ID"),
		},
		WhatsApp: WhatsAppConfig{
			InstanceID:    viper.GetString("WHATSAPP_INSTANCE_ID"),
			Token:         viper.GetString("WHATSAPP_TOKEN"),
			PhoneNumberID: viper.GetString("WHATSAPP_PHONE_NUMBER_ID"),
			AccessToken:   viper.GetString("WHATSAPP_ACCESS_TOKEN"),
		},
		Email: EmailConfig{
			SMTPHost: viper.GetString("EMAIL_SMTP_HOST"),
			SMTPPort: viper.GetInt("EMAIL_SMTP_PORT"),
			Username: viper.GetString("EMAIL_USERNAME"),
			Password: viper.GetString("EMAIL_PASSWORD"),
			From:     viper.GetString("EMAIL_FROM"),
			APIKey:   getAnyEnv("EMAIL_API_KEY", "RESEND_API_KEY", "SENDGRID_API_KEY", "BREVO_API_KEY"),
			Provider: viper.GetString("EMAIL_PROVIDER"),
		},
		Google: GoogleOAuthConfig{
			ClientID:     viper.GetString("GOOGLE_CLIENT_ID"),
			ClientSecret: viper.GetString("GOOGLE_CLIENT_SECRET"),
			RedirectURL:  viper.GetString("GOOGLE_REDIRECT_URL"),
		},
	}

	port := viper.GetInt("PORT")
	if port == 0 {
		port = viper.GetInt("APP_PORT")
	}
	if port == 0 {
		port = 8080
	}
	cfg.App.Port = port
	cfg.Server.Port = fmt.Sprintf("%d", port)
	if cfg.CORS.AllowedOrigins == "" {
		cfg.CORS.AllowedOrigins = "*"
	}

	return cfg, nil
}

// LoadConfig alias for Load to accept optional path string
func LoadConfig(path ...string) (*Config, error) {
	return Load()
}

func getAnyEnv(keys ...string) string {
	for _, k := range keys {
		if val := strings.TrimSpace(viper.GetString(k)); val != "" {
			return val
		}
	}
	return ""
}
