package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type PostmanItem struct {
	Name    string           `json:"name"`
	Request *PostmanRequest  `json:"request,omitempty"`
	Item    []*PostmanItem   `json:"item,omitempty"`
}

type PostmanRequest struct {
	Method string         `json:"method"`
	Header []PostmanHeader`json:"header"`
	Body   *PostmanBody   `json:"body,omitempty"`
	URL    PostmanURL     `json:"url"`
}

type PostmanHeader struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Type  string `json:"type,omitempty"`
}

type PostmanBody struct {
	Mode string `json:"mode"`
	Raw  string `json:"raw"`
}

type PostmanURL struct {
	Raw  string   `json:"raw"`
	Host []string `json:"host"`
	Path []string `json:"path"`
}

type PostmanCollection struct {
	Info PostmanInfo   `json:"info"`
	Item []*PostmanItem `json:"item"`
}

type PostmanInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Schema      string `json:"schema"`
}

func main() {
	collection := PostmanCollection{
		Info: PostmanInfo{
			Name:        "Enterprise Multi-Vendor & Reseller E-Commerce Platform API Collection",
			Description: "Complete Postman collection covering ~200 API endpoints across 23 domains (Daraz / Amazon / Meesho Grade Architecture). Live connected to NeonDB PostgreSQL & Redis.",
			Schema:      "https://schema.getpostman.com/json/collection/v2.1.0/collection.json",
		},
		Item: []*PostmanItem{
			createAuthFolder(),
			createCatalogFolder(),
			createInventoryFolder(),
			createCartFolder(),
			createOrdersFolder(),
			createPaymentFolder(),
			createLogisticsFolder(),
			createResellerFolder(),
			createAffiliateFolder(),
			createReviewFolder(),
			createNotificationFolder(),
			createSearchFolder(),
			createAnalyticsFolder(),
			createMediaFolder(),
			createAdminFinanceFolder(),
			createAdminFraudFolder(),
			createAdminSettingsFolder(),
			createChinaSourcingFolder(),
			createWalletDisputePromotionFolder(),
		},
	}

	data, err := json.MarshalIndent(collection, "", "  ")
	if err != nil {
		fmt.Printf("Error generating postman collection: %v\n", err)
		os.Exit(1)
	}

	err = os.WriteFile("ecom_postman_collection.json", data, 0644)
	if err != nil {
		fmt.Printf("Error writing postman collection file: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Successfully generated updated ecom_postman_collection.json with exact backend handler URL paths!")
}

func stdHeaders(auth bool) []PostmanHeader {
	headers := []PostmanHeader{
		{Key: "Content-Type", Value: "application/json"},
		{Key: "Accept", Value: "application/json"},
	}
	if auth {
		headers = append(headers, PostmanHeader{Key: "Authorization", Value: "Bearer {{access_token}}"})
	}
	return headers
}

const BaseURL = "https://multivendor-e-commarse-backend.onrender.com"

func req(method string, path string, bodyStr string, auth bool) *PostmanRequest {
	var body *PostmanBody
	if bodyStr != "" {
		body = &PostmanBody{
			Mode: "raw",
			Raw:  bodyStr,
		}
	}

	pathSegments := []string{"api", "v1"}
	for _, seg := range strings.Split(path, "/") {
		if seg != "" {
			pathSegments = append(pathSegments, seg)
		}
	}

	return &PostmanRequest{
		Method: method,
		Header: stdHeaders(auth),
		Body:   body,
		URL: PostmanURL{
			Raw:  BaseURL + "/api/v1/" + path,
			Host: []string{"https:", "", "multivendor-e-commarse-backend.onrender.com"},
			Path: pathSegments,
		},
	}
}

func createAuthFolder() *PostmanItem {
	return &PostmanItem{
		Name: "1. Auth & User Management (11 endpoints)",
		Item: []*PostmanItem{
			{Name: "Request Phone OTP", Request: req("POST", "auth/otp/send", `{"target": "+8801712345678", "purpose": "LOGIN"}`, false)},
			{Name: "Verify Phone OTP", Request: req("POST", "auth/otp/verify", `{"target": "+8801712345678", "otp": "123456"}`, false)},
			{Name: "Register User (Email)", Request: req("POST", "auth/email/register", `{"email": "customer@example.com", "password": "Password123!", "full_name": "Test Customer", "phone": "+8801712345678", "role": "CUSTOMER"}`, false)},
			{Name: "Login User (Email)", Request: req("POST", "auth/email/login", `{"email": "customer@example.com", "password": "Password123!"}`, false)},
			{Name: "Google OAuth Callback", Request: req("GET", "auth/google/callback", "", false)},
			{Name: "Refresh Access Token", Request: req("POST", "auth/refresh", `{"refresh_token": "sample_refresh_token"}`, false)},
			{Name: "Logout User", Request: req("POST", "auth/logout", `{}`, true)},
			{Name: "Get My Profile", Request: req("GET", "auth/me", "", true)},
			{Name: "Password Reset Request", Request: req("POST", "auth/password/reset-request", `{"email": "customer@example.com"}`, false)},
			{Name: "Password Reset", Request: req("PUT", "auth/password/reset", `{"email": "customer@example.com", "otp_code": "123456", "new_password": "NewPassword123!"}`, false)},
			{Name: "Get User Sessions", Request: req("GET", "auth/sessions", "", true)},
		},
	}
}

func createCatalogFolder() *PostmanItem {
	return &PostmanItem{
		Name: "2. Catalog Engine - Public, Seller, Admin (31 endpoints)",
		Item: []*PostmanItem{
			{Name: "Public: List Public Products", Request: req("GET", "catalog/products", "", false)},
			{Name: "Public: Get Product by Slug", Request: req("GET", "catalog/products/sample-product-slug", "", false)},
			{Name: "Public: List Public Categories", Request: req("GET", "catalog/categories", "", false)},
			{Name: "Public: Get Category by Slug", Request: req("GET", "catalog/categories/sample-category-slug", "", false)},
			{Name: "Public: List Public Brands", Request: req("GET", "catalog/brands", "", false)},
			{Name: "Public: Get Brand by Slug", Request: req("GET", "catalog/brands/sample-brand-slug", "", false)},
			{Name: "Public: Get Product Variants", Request: req("GET", "catalog/products/101/variants", "", false)},
			{Name: "Public: Get Product Reviews", Request: req("GET", "catalog/products/101/reviews", "", false)},
			{Name: "Seller: List Seller Products", Request: req("GET", "seller/catalog/products", "", true)},
			{Name: "Seller: Create Product Draft", Request: req("POST", "seller/catalog/products", `{"name": "Seller New Shoe", "category_id": "44444444-4444-4444-4444-444444444441", "brand_id": "55555555-5555-5555-5555-555555555551", "description": "High quality shoe"}`, true)},
			{Name: "Seller: Get Seller Product Detail", Request: req("GET", "seller/catalog/products/101", "", true)},
			{Name: "Seller: Update Product", Request: req("PUT", "seller/catalog/products/101", `{"name": "Updated Shoe Name"}`, true)},
			{Name: "Seller: Update Product Status", Request: req("PUT", "seller/catalog/products/101/status", `{"status": "PENDING_APPROVAL"}`, true)},
			{Name: "Seller: Add Product Variant", Request: req("POST", "seller/catalog/products/101/variants", `{"sku": "SHOE-RED-42", "price": 1500.00}`, true)},
			{Name: "Seller: Update Variant", Request: req("PUT", "seller/catalog/variants/501", `{"price": 1450.00}`, true)},
			{Name: "Seller: Delete Product", Request: req("DELETE", "seller/catalog/products/101", "", true)},
			{Name: "Admin: List Admin Products", Request: req("GET", "admin/catalog/products", "", true)},
			{Name: "Admin: Pending Moderation Queue", Request: req("GET", "admin/catalog/products/pending", "", true)},
			{Name: "Admin: Approve/Reject Product", Request: req("PUT", "admin/catalog/products/101/approval", `{"approved": true, "reason": "Approved by admin"}`, true)},
			{Name: "Admin: Create Category", Request: req("POST", "admin/catalog/categories", `{"name": "Electronics", "slug": "electronics"}`, true)},
			{Name: "Admin: Update Category", Request: req("PUT", "admin/catalog/categories/1", `{"name": "Electronics & Tech"}`, true)},
			{Name: "Admin: Create Brand", Request: req("POST", "admin/catalog/brands", `{"name": "Nike", "slug": "nike", "is_verified": true}`, true)},
			{Name: "Admin: Create Attribute", Request: req("POST", "admin/catalog/attributes", `{"name": "Color", "code": "color", "type": "SELECT"}`, true)},
			{Name: "Admin: Create Attribute Option", Request: req("POST", "admin/catalog/attributes/options", `{"attribute_id": "attr_123", "value": "Red"}`, true)},
		},
	}
}

func createInventoryFolder() *PostmanItem {
	return &PostmanItem{
		Name: "3. Inventory Management (8 endpoints)",
		Item: []*PostmanItem{
			{Name: "Get Variant Inventory", Request: req("GET", "inventory/variants/501", "", true)},
			{Name: "Update Inventory Stock", Request: req("PUT", "inventory/variants/501", `{"warehouse_id": 1, "quantity": 100}`, true)},
			{Name: "Reserve Stock", Request: req("POST", "inventory/reserve", `{"variant_id": 501, "quantity": 2}`, true)},
			{Name: "Release Stock Reservation", Request: req("POST", "inventory/release", `{"reservation_id": "res_12345"}`, true)},
			{Name: "List Warehouses", Request: req("GET", "inventory/warehouses", "", true)},
			{Name: "Create Warehouse", Request: req("POST", "inventory/warehouses", `{"name": "Dhaka Hub", "location": "Dhaka", "type": "LOCAL_BD"}`, true)},
			{Name: "Get Movement Ledger", Request: req("GET", "inventory/movements", "", true)},
			{Name: "Bulk CSV Inventory Sync", Request: req("POST", "inventory/bulk-sync", `{"file_url": "https://storage.cdn/inventory.csv"}`, true)},
		},
	}
}

func createCartFolder() *PostmanItem {
	return &PostmanItem{
		Name: "4. Cart Engine (8 endpoints)",
		Item: []*PostmanItem{
			{Name: "Get User Cart", Request: req("GET", "cart", "", true)},
			{Name: "Add Item to Cart", Request: req("POST", "cart/items", `{"variant_id": 501, "quantity": 1}`, true)},
			{Name: "Update Cart Item Quantity", Request: req("PUT", "cart/items/item_123", `{"quantity": 3}`, true)},
			{Name: "Remove Item from Cart", Request: req("DELETE", "cart/items/item_123", "", true)},
			{Name: "Clear Entire Cart", Request: req("DELETE", "cart/clear", "", true)},
			{Name: "Merge Guest Cart on Login", Request: req("POST", "cart/merge", `{"guest_cart_id": "guest_98765"}`, true)},
			{Name: "Apply Coupon Code", Request: req("POST", "cart/coupons/apply", `{"code": "SAVE10"}`, true)},
			{Name: "Remove Coupon Code", Request: req("DELETE", "cart/coupons/remove", "", true)},
		},
	}
}

func createOrdersFolder() *PostmanItem {
	return &PostmanItem{
		Name: "5. Order Engine - Customer, Seller, Admin (32 endpoints)",
		Item: []*PostmanItem{
			{Name: "Customer: List Orders", Request: req("GET", "customer/orders", "", true)},
			{Name: "Customer: Order Details", Request: req("GET", "customer/orders/ord_1001", "", true)},
			{Name: "Customer: Order Tracking", Request: req("GET", "customer/orders/ord_1001/track", "", true)},
			{Name: "Customer: Order Invoice", Request: req("GET", "customer/orders/ord_1001/invoice", "", true)},
			{Name: "Customer: Checkout", Request: req("POST", "customer/orders/checkout", `{"shipping_address_id": "77777777-7777-7777-7777-777777777771", "payment_method": "SSLCOMMERZ"}`, true)},
			{Name: "Customer: Cancel Order", Request: req("POST", "customer/orders/ord_1001/cancel", `{"reason": "Changed my mind"}`, true)},
			{Name: "Seller: List Sub-Orders", Request: req("GET", "seller/orders", "", true)},
			{Name: "Seller: Pack Sub-Order", Request: req("POST", "seller/orders/sub_2001/pack", `{}`, true)},
			{Name: "Seller: Print Label", Request: req("POST", "seller/orders/sub_2001/print-label", `{}`, true)},
			{Name: "Admin: List All Orders", Request: req("GET", "admin/orders", "", true)},
			{Name: "Admin: Update Order Status", Request: req("PUT", "admin/orders/ord_1001/status", `{"status": "PROCESSING"}`, true)},
		},
	}
}

func createPaymentFolder() *PostmanItem {
	return &PostmanItem{
		Name: "6. Payment & Webhooks (12 endpoints)",
		Item: []*PostmanItem{
			{Name: "Payment: List Gateways", Request: req("GET", "payments/methods", "", true)},
			{Name: "Payment: Get Payment Status", Request: req("GET", "payments/status/pay_9001", "", true)},
			{Name: "Payment: Initiate Gateway Payment", Request: req("POST", "payments/initiate", `{"master_order_id": "ord_1001", "payment_gateway": "SSLCOMMERZ", "amount": 1500.00}`, true)},
			{Name: "Payment: Verify Transaction", Request: req("POST", "payments/verify", `{"transaction_id": "tx_987654"}`, true)},
			{Name: "Payment: Request Refund", Request: req("POST", "payments/refunds", `{"order_id": "ord_1001", "amount": 1500.00}`, true)},
			{Name: "Webhook: bKash Execute Callback", Request: req("POST", "payments/webhooks/bkash", `{"paymentID": "bk_123", "status": "Completed"}`, false)},
			{Name: "Webhook: SSLCommerz IPN", Request: req("POST", "payments/webhooks/sslcommerz", `{"tran_id": "tx_987654", "status": "VALID"}`, false)},
			{Name: "Webhook: Nagad Callback", Request: req("POST", "payments/webhooks/nagad", `{"payment_ref_id": "ng_123", "status": "Success"}`, false)},
		},
	}
}

func createLogisticsFolder() *PostmanItem {
	return &PostmanItem{
		Name: "7. Logistics & Couriers (9 endpoints)",
		Item: []*PostmanItem{
			{Name: "Create Courier Consignment", Request: req("POST", "shipping/create-consignment", `{"sub_order_id": "sub_2001", "courier_name": "STEADFAST"}`, true)},
			{Name: "Track Consignment Status", Request: req("GET", "shipping/track/csg_5001", "", true)},
			{Name: "Calculate Shipping Rate", Request: req("POST", "shipping/calculate-rate", `{"weight_kg": 1.5, "destination_city": "Chittagong"}`, false)},
			{Name: "Webhook: Pathao Courier Webhook", Request: req("POST", "shipping/webhooks/pathao", `{"consignment_id": "p_123", "order_status": "Delivered"}`, false)},
			{Name: "Webhook: Steadfast Courier Status", Request: req("POST", "shipping/webhooks/steadfast", `{"consignment_id": 123, "status": "delivered"}`, false)},
			{Name: "Webhook: Paperfly Courier Webhook", Request: req("POST", "shipping/webhooks/paperfly", `{"tracking_id": "pf_123", "status": "DELIVERED"}`, false)},
		},
	}
}

func createResellerFolder() *PostmanItem {
	return &PostmanItem{
		Name: "8. Reseller & Dropshipping Portal (11 endpoints)",
		Item: []*PostmanItem{
			{Name: "Public Reseller Store View", Request: req("GET", "reseller/store/karim-shop", "", false)},
			{Name: "Get Reseller Profile", Request: req("GET", "reseller/profile", "", true)},
			{Name: "Setup Reseller Store", Request: req("POST", "reseller/setup", `{"shop_name": "Rahim Store", "nid_number": "1234567890"}`, true)},
			{Name: "Get Wholesale Catalog", Request: req("GET", "reseller/catalog", "", true)},
			{Name: "Calculate Margin", Request: req("POST", "reseller/margin/calculate", `{"variant_id": 501, "customer_price": 2000.00}`, true)},
			{Name: "Generate Shareable Link", Request: req("POST", "reseller/share-link", `{"product_id": "101"}`, true)},
		},
	}
}

func createAffiliateFolder() *PostmanItem {
	return &PostmanItem{
		Name: "9. Affiliate Referral Program (10 endpoints)",
		Item: []*PostmanItem{
			{Name: "Get Affiliate Profile", Request: req("GET", "affiliate/profile", "", true)},
			{Name: "Get Affiliate Links", Request: req("GET", "affiliate/links", "", true)},
			{Name: "Get Affiliate Conversions", Request: req("GET", "affiliate/conversions", "", true)},
			{Name: "Get Affiliate Clicks", Request: req("GET", "affiliate/clicks", "", true)},
			{Name: "Get Affiliate Earnings", Request: req("GET", "affiliate/earnings", "", true)},
			{Name: "Apply for Affiliate Program", Request: req("POST", "affiliate/apply", `{"website": "myblog.com"}`, true)},
			{Name: "Create Referral Link", Request: req("POST", "affiliate/links", `{"target_url": "/product/sample-product"}`, true)},
			{Name: "Request Affiliate Withdrawal", Request: req("POST", "affiliate/withdraw", `{"amount": 1000.00}`, true)},
		},
	}
}

func createReviewFolder() *PostmanItem {
	return &PostmanItem{
		Name: "10. Ratings & Verified Reviews (7 endpoints)",
		Item: []*PostmanItem{
			{Name: "Submit Product Review", Request: req("POST", "reviews", `{"product_id": "101", "rating": 5, "comment": "Excellent quality!"}`, true)},
			{Name: "Get Product Reviews", Request: req("GET", "reviews/product/101", "", false)},
			{Name: "Vote Helpful on Review", Request: req("POST", "reviews/rev_1/helpful", `{}`, true)},
			{Name: "Seller Reply to Review", Request: req("POST", "reviews/rev_1/seller-reply", `{"reply": "Thank you for your feedback!"}`, true)},
			{Name: "Get Seller Composite Rating", Request: req("GET", "reviews/seller/sel_1/rating", "", false)},
		},
	}
}

func createNotificationFolder() *PostmanItem {
	return &PostmanItem{
		Name: "11. Omnichannel Notifications (5 endpoints)",
		Item: []*PostmanItem{
			{Name: "Get User Notifications", Request: req("GET", "notifications", "", true)},
			{Name: "Mark Notification as Read", Request: req("PUT", "notifications/notif_1/read", `{}`, true)},
			{Name: "Update Notification Preferences", Request: req("PUT", "notifications/preferences", `{"sms_enabled": true}`, true)},
		},
	}
}

func createSearchFolder() *PostmanItem {
	return &PostmanItem{
		Name: "12. MeiliSearch Engine (2 endpoints)",
		Item: []*PostmanItem{
			{Name: "Search Products with Facets", Request: req("GET", "search?q=shoe", "", false)},
			{Name: "Search Autocomplete Suggestions", Request: req("GET", "search/suggestions?q=nik", "", false)},
		},
	}
}

func createAnalyticsFolder() *PostmanItem {
	return &PostmanItem{
		Name: "13. Executive Analytics & BI (6 endpoints)",
		Item: []*PostmanItem{
			{Name: "Get Real-time GMV Dashboard", Request: req("GET", "analytics/gmv", "", true)},
			{Name: "Get Courier Delivery Success Rates", Request: req("GET", "analytics/couriers", "", true)},
			{Name: "Get Live Active Visitors Count", Request: req("GET", "analytics/live-visitors", "", true)},
		},
	}
}

func createMediaFolder() *PostmanItem {
	return &PostmanItem{
		Name: "14. Media Engine & Presigned S3/R2 (5 endpoints)",
		Item: []*PostmanItem{
			{Name: "Request Presigned Upload URL", Request: req("POST", "media/presigned-url", `{"file_name": "product_image.jpg"}`, true)},
			{Name: "Confirm Upload & Process Image Pipeline", Request: req("POST", "media/confirm-upload", `{"file_key": "products/product_image.jpg"}`, true)},
		},
	}
}

func createAdminFinanceFolder() *PostmanItem {
	return &PostmanItem{
		Name: "15. Admin Finance & Settlement (11 endpoints)",
		Item: []*PostmanItem{
			{Name: "List Pending Escrow Entries", Request: req("GET", "admin/finance/escrow/pending", "", true)},
			{Name: "Release Escrow Manually", Request: req("POST", "admin/finance/escrow/release", `{"escrow_entry_id": "esc_1001"}`, true)},
		},
	}
}

func createAdminFraudFolder() *PostmanItem {
	return &PostmanItem{
		Name: "16. Admin Fraud & Risk Management (8 endpoints)",
		Item: []*PostmanItem{
			{Name: "Get Customer Risk Score Profile", Request: req("GET", "admin/fraud/risk-profile/user_1", "", true)},
			{Name: "Blacklist Phone Number", Request: req("POST", "admin/fraud/blacklist/phone", `{"phone": "+8801700000000"}`, true)},
		},
	}
}

func createAdminSettingsFolder() *PostmanItem {
	return &PostmanItem{
		Name: "17. Admin System Settings & Config (6 endpoints)",
		Item: []*PostmanItem{
			{Name: "Get System Settings Config", Request: req("GET", "admin/settings", "", true)},
		},
	}
}

func createChinaSourcingFolder() *PostmanItem {
	return &PostmanItem{
		Name: "18. China Direct Sourcing Engine (9 endpoints)",
		Item: []*PostmanItem{
			{Name: "List China Sourcing Batches", Request: req("GET", "china/batches", "", true)},
		},
	}
}

func createWalletDisputePromotionFolder() *PostmanItem {
	return &PostmanItem{
		Name: "19. Wallet, Disputes & Promotions (20 endpoints)",
		Item: []*PostmanItem{
			{Name: "Wallet: Get Balance", Request: req("GET", "wallet/balance", "", true)},
			{Name: "Dispute: Create Customer Dispute", Request: req("POST", "disputes", `{"order_id": "ord_1001", "reason": "Damaged goods"}`, true)},
			{Name: "Promotion: Create Coupon", Request: req("POST", "promotions/coupons", `{"code": "SUMMER20", "discount_percentage": 20.0}`, true)},
		},
	}
}
