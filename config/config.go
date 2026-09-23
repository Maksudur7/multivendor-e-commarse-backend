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
	Database DatabaseConfig
	Redis    RedisConfig
	JWT      JWTConfig
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
}

type ServerConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

type DatabaseConfig struct {
	URL             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type RedisConfig struct {
	URL        string
	MaxRetries int
}

type JWTConfig struct {
	AccessSecret  string
	RefreshSecret string
	AccessExpiry  time.Duration
	RefreshExpiry time.Duration
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
	AppKey     string
	AppSecret  string
	Username   string
	Password   string
	IsSandbox  bool
	BaseURL    string
}

type NagadConfig struct {
	MerchantID  string
	PublicKey   string
	PrivateKey  string
	IsSandbox   bool
	CallbackURL string
}

type CourierConfig struct {
	Steadfast SteadfastConfig
	Pathao    PathaoConfig
	RedX      RedXConfig
}

type SteadfastConfig struct {
	APIKey    string
	SecretKey string
	BaseURL   string
}

type PathaoConfig struct {
	ClientID     string
	ClientSecret string
	Username     string
	Password     string
	BaseURL      string
}

type RedXConfig struct {
	APIKey  string
	BaseURL string
}

type SMSConfig struct {
	Provider   string
	APIToken   string
	SenderID   string
}

type WhatsAppConfig struct {
	AccessToken   string
	PhoneNumberID string
	APIVersion    string
}

type EmailConfig struct {
	Provider    string
	SMTPHost    string
	SMTPPort    int
	SMTPUser    string
	SMTPPass    string
	FromName    string
	FromAddress string
}

type FCMConfig struct {
	ServerKey string
	ProjectID string
}

type PlatformConfig struct {
	DefaultCommissionRate float64
	CODLimitDefault       float64
	CODLimitNewUser       float64
	ReturnWindowDays      int
	EscrowReleaseDays     int
	MinSellerPayout       float64
	MinResellerPayout     float64
	MinAffiliatePayout    float64
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

	// Read .env file (ignore error if not found — use env vars directly)
	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("Warning: .env file not found, using environment variables\n")
	}

	cfg := &Config{
		App: AppConfig{
			Env:  viper.GetString("APP_ENV"),
			Name: viper.GetString("APP_NAME"),
		},
		Server: ServerConfig{
			Port:         viper.GetString("SERVER_PORT"),
			ReadTimeout:  viper.GetDuration("SERVER_READ_TIMEOUT"),
			WriteTimeout: viper.GetDuration("SERVER_WRITE_TIMEOUT"),
			IdleTimeout:  viper.GetDuration("SERVER_IDLE_TIMEOUT"),
		},
		Database: DatabaseConfig{
			URL:             viper.GetString("DATABASE_URL"),
			MaxOpenConns:    viper.GetInt("DB_MAX_OPEN_CONNS"),
			MaxIdleConns:    viper.GetInt("DB_MAX_IDLE_CONNS"),
			ConnMaxLifetime: viper.GetDuration("DB_CONN_MAX_LIFETIME"),
		},
		Redis: RedisConfig{
			URL:        viper.GetString("REDIS_URL"),
			MaxRetries: viper.GetInt("REDIS_MAX_RETRIES"),
		},
		JWT: JWTConfig{
			AccessSecret:  viper.GetString("JWT_ACCESS_SECRET"),
			RefreshSecret: viper.GetString("JWT_REFRESH_SECRET"),
			AccessExpiry:  viper.GetDuration("JWT_ACCESS_EXPIRY"),
			RefreshExpiry: viper.GetDuration("JWT_REFRESH_EXPIRY"),
		},
		Meili: MeiliConfig{
			Host:         viper.GetString("MEILI_HOST"),
			MasterKey:    viper.GetString("MEILI_MASTER_KEY"),
			ProductIndex: viper.GetString("MEILI_PRODUCT_INDEX"),
		},
		R2: R2Config{
			AccountID:       viper.GetString("R2_ACCOUNT_ID"),
			AccessKeyID:     viper.GetString("R2_ACCESS_KEY_ID"),
			SecretAccessKey: viper.GetString("R2_SECRET_ACCESS_KEY"),
			PublicBucket:    viper.GetString("R2_PUBLIC_BUCKET"),
			PrivateBucket:   viper.GetString("R2_PRIVATE_BUCKET"),
			PublicCDNURL:    viper.GetString("R2_PUBLIC_CDN_URL"),
		},
		Payment: PaymentConfig{
			SSLCommerz: SSLCommerzConfig{
				StoreID:       viper.GetString("SSLCOMMERZ_STORE_ID"),
				StorePassword: viper.GetString("SSLCOMMERZ_STORE_PASSWORD"),
				IsLive:        viper.GetBool("SSLCOMMERZ_IS_LIVE"),
				SuccessURL:    viper.GetString("SSLCOMMERZ_SUCCESS_URL"),
				FailURL:       viper.GetString("SSLCOMMERZ_FAIL_URL"),
				CancelURL:     viper.GetString("SSLCOMMERZ_CANCEL_URL"),
			},
			BKash: BKashConfig{
				AppKey:    viper.GetString("BKASH_APP_KEY"),
				AppSecret: viper.GetString("BKASH_APP_SECRET"),
				Username:  viper.GetString("BKASH_USERNAME"),
				Password:  viper.GetString("BKASH_PASSWORD"),
				IsSandbox: viper.GetBool("BKASH_IS_SANDBOX"),
				BaseURL:   viper.GetString("BKASH_BASE_URL"),
			},
			Nagad: NagadConfig{
				MerchantID:  viper.GetString("NAGAD_MERCHANT_ID"),
				PublicKey:   viper.GetString("NAGAD_PUBLIC_KEY"),
				PrivateKey:  viper.GetString("NAGAD_PRIVATE_KEY"),
				IsSandbox:   viper.GetBool("NAGAD_IS_SANDBOX"),
				CallbackURL: viper.GetString("NAGAD_CALLBACK_URL"),
			},
		},
		Courier: CourierConfig{
			Steadfast: SteadfastConfig{
				APIKey:    viper.GetString("STEADFAST_API_KEY"),
				SecretKey: viper.GetString("STEADFAST_SECRET_KEY"),
				BaseURL:   viper.GetString("STEADFAST_BASE_URL"),
			},
			Pathao: PathaoConfig{
				ClientID:     viper.GetString("PATHAO_CLIENT_ID"),
				ClientSecret: viper.GetString("PATHAO_CLIENT_SECRET"),
				Username:     viper.GetString("PATHAO_USERNAME"),
				Password:     viper.GetString("PATHAO_PASSWORD"),
				BaseURL:      viper.GetString("PATHAO_BASE_URL"),
			},
			RedX: RedXConfig{
				APIKey:  viper.GetString("REDX_API_KEY"),
				BaseURL: viper.GetString("REDX_BASE_URL"),
			},
		},
		SMS: SMSConfig{
			Provider: viper.GetString("SMS_PROVIDER"),
			APIToken: viper.GetString("GREENWEB_API_TOKEN"),
			SenderID: viper.GetString("GREENWEB_SENDER_ID"),
		},
		WhatsApp: WhatsAppConfig{
			AccessToken:   viper.GetString("WHATSAPP_ACCESS_TOKEN"),
			PhoneNumberID: viper.GetString("WHATSAPP_PHONE_NUMBER_ID"),
			APIVersion:    viper.GetString("WHATSAPP_API_VERSION"),
		},
		Email: EmailConfig{
			Provider:    viper.GetString("EMAIL_PROVIDER"),
			SMTPHost:    viper.GetString("SMTP_HOST"),
			SMTPPort:    viper.GetInt("SMTP_PORT"),
			SMTPUser:    viper.GetString("SMTP_USER"),
			SMTPPass:    viper.GetString("SMTP_PASSWORD"),
			FromName:    viper.GetString("EMAIL_FROM_NAME"),
			FromAddress: viper.GetString("EMAIL_FROM_ADDRESS"),
		},
		FCM: FCMConfig{
			ServerKey: viper.GetString("FCM_SERVER_KEY"),
			ProjectID: viper.GetString("FCM_PROJECT_ID"),
		},
		Platform: PlatformConfig{
			DefaultCommissionRate: viper.GetFloat64("DEFAULT_COMMISSION_RATE"),
			CODLimitDefault:       viper.GetFloat64("DEFAULT_COD_LIMIT"),
			CODLimitNewUser:       viper.GetFloat64("NEW_USER_COD_LIMIT"),
			ReturnWindowDays:      viper.GetInt("RETURN_WINDOW_DAYS"),
			EscrowReleaseDays:     viper.GetInt("ESCROW_RELEASE_DAYS"),
			MinSellerPayout:       viper.GetFloat64("MIN_SELLER_PAYOUT"),
			MinResellerPayout:     viper.GetFloat64("MIN_RESELLER_PAYOUT"),
			MinAffiliatePayout:    viper.GetFloat64("MIN_AFFILIATE_PAYOUT"),
		},
		Fraud: FraudConfig{
			BlockScore:          viper.GetFloat64("FRAUD_BLOCK_SCORE"),
			ReviewScore:         viper.GetFloat64("FRAUD_REVIEW_SCORE"),
			MaxAccountsPerIP:    viper.GetInt("MAX_ACCOUNTS_PER_IP"),
			OTPRateLimitPerHour: viper.GetInt("OTP_RATE_LIMIT_PER_HOUR"),
			CheckoutRateLimit:   viper.GetInt("CHECKOUT_RATE_LIMIT_PER_HOUR"),
		},
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// validate checks required configurations.
func (c *Config) validate() error {
	if c.Database.URL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if c.JWT.AccessSecret == "" {
		return fmt.Errorf("JWT_ACCESS_SECRET is required")
	}
	if c.JWT.RefreshSecret == "" {
		return fmt.Errorf("JWT_REFRESH_SECRET is required")
	}
	if c.Server.Port == "" {
		c.Server.Port = "8080"
	}
	return nil
}

// IsDevelopment returns true if running in development mode.
func (c *Config) IsDevelopment() bool {
	return c.App.Env == "development"
}

// IsProduction returns true if running in production mode.
func (c *Config) IsProduction() bool {
	return c.App.Env == "production"
}
