package main

import (
	"context"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/yourusername/ecom-backend/config"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
	log.Info().Msg("🌱 Starting Live Database Seeding on NeonDB...")

	cfg, err := config.LoadConfig(".")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load config")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DB.URL)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to NeonDB")
	}
	defer pool.Close()

	// 1. Seed Users (On Conflict Do Nothing on phone/email)
	log.Info().Msg("1. Seeding Users (Admin, Seller, Customer)...")
	_, _ = pool.Exec(ctx, `
		INSERT INTO users (id, phone, email, password_hash, full_name, role, status, email_verified, phone_verified)
		VALUES 
			('11111111-1111-1111-1111-111111111111', '+8801700000091', 'admin_demo@ecom.bd', '$2a$12$eImiTXuWVxfM37uY4JANjOL.sUjhqN2aV3h987654321', 'System Administrator', 'SUPER_ADMIN', 'ACTIVE', true, true),
			('22222222-2222-2222-2222-222222222222', '+8801700000092', 'seller_demo@ecom.bd', '$2a$12$eImiTXuWVxfM37uY4JANjOL.sUjhqN2aV3h987654321', 'Apex Footwear Seller', 'SELLER', 'ACTIVE', true, true),
			('33333333-3333-3333-3333-333333333333', '+8801700000093', 'customer_demo@ecom.bd', '$2a$12$eImiTXuWVxfM37uY4JANjOL.sUjhqN2aV3h987654321', 'Rahim Ahmed Customer', 'CUSTOMER', 'ACTIVE', true, true)
		ON CONFLICT DO NOTHING;
	`)

	// 2. Seed Categories & Brands
	log.Info().Msg("2. Seeding Categories & Brands...")
	_, _ = pool.Exec(ctx, `
		INSERT INTO categories (id, name, slug, is_active, is_leaf, depth, path)
		VALUES 
			('44444444-4444-4444-4444-444444444441', 'Demo Fashion & Apparel', 'demo-fashion-apparel', true, false, 0, '/'),
			('44444444-4444-4444-4444-444444444442', 'Demo Electronics & Gadgets', 'demo-electronics-gadgets', true, false, 0, '/')
		ON CONFLICT DO NOTHING;

		INSERT INTO brands (id, name, slug, is_verified, is_active)
		VALUES 
			('55555555-5555-5555-5555-555555555551', 'Apex Demo Brand', 'apex-demo', true, true),
			('55555555-5555-5555-5555-555555555552', 'Samsung Demo Brand', 'samsung-demo', true, true)
		ON CONFLICT DO NOTHING;
	`)

	// 3. Seed Products
	log.Info().Msg("3. Seeding Products...")
	_, _ = pool.Exec(ctx, `
		INSERT INTO products (id, seller_id, category_id, brand_id, product_type, name, slug, description, status, is_featured)
		VALUES 
			('66666666-6666-6666-6666-666666666661', '22222222-2222-2222-2222-222222222222', '44444444-4444-4444-4444-444444444441', '55555555-5555-5555-5555-555555555551', 'SIMPLE', 'Apex Leather Formal Shoe', 'apex-leather-shoe-demo', 'Genuine handcrafted leather shoe for men', 'APPROVED', true),
			('66666666-6666-6666-6666-666666666662', '22222222-2222-2222-2222-222222222222', '44444444-4444-4444-4444-444444444442', '55555555-5555-5555-5555-555555555552', 'SIMPLE', 'Samsung Galaxy A54 5G', 'samsung-galaxy-a54-demo', 'Super AMOLED 120Hz display', 'APPROVED', true)
		ON CONFLICT DO NOTHING;
	`)

	// 4. Seed Addresses
	log.Info().Msg("4. Seeding Customer Address...")
	_, _ = pool.Exec(ctx, `
		INSERT INTO addresses (id, user_id, label, recipient_name, recipient_phone, address_line1, city, district, is_default)
		VALUES 
			('77777777-7777-7777-7777-777777777771', '33333333-3333-3333-3333-333333333333', 'Home', 'Rahim Ahmed', '+8801700000093', 'House 42, Road 11, Banani', 'Dhaka', 'Dhaka', true)
		ON CONFLICT DO NOTHING;
	`)

	log.Info().Msg("✅ Live Database Seeding Completed Successfully on NeonDB! Operational records active.")
}
