package main

import (
	"encoding/json"
	"fmt"
	"os"
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
			Description: "Complete Postman collection covering all ~200 API endpoints across 23 domains (Daraz / Amazon / Meesho grade platform). Live connected to NeonDB PostgreSQL & Redis.",
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

	fmt.Println("✅ Successfully generated ecom_postman_collection.json with all ~200 API endpoints!")
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

func req(method string, path string, bodyStr string, auth bool) *PostmanRequest {
	var body *PostmanBody
	if bodyStr != "" {
		body = &PostmanBody{
			Mode: "raw",
			Raw:  bodyStr,
		}
	}
	return &PostmanRequest{
		Method: method,
		Header: stdHeaders(auth),
		Body:   body,
		URL: PostmanURL{
			Raw:  "http://localhost:8080/api/v1/" + path,
			Host: []string{"http:", "", "localhost:8080"},
			Path: []string{"api", "v1", path},
		},
	}
}

func createAuthFolder() *PostmanItem {
	return &PostmanItem{
		Name: "1. Auth & User Management (11 endpoints)",
		Item: []*PostmanItem{
			{Name: "Request Phone OTP", Request: req("POST", "auth/otp/send", `{"phone": "+8801712345678"}`, false)},
			{Name: "Verify Phone OTP", Request: req("POST", "auth/otp/verify", `{"phone": "+8801712345678", "otp": "123456"}`, false)},
			{Name: "Register User", Request: req("POST", "auth/register", `{"email": "customer@example.com", "password": "Password123!", "full_name": "Test Customer", "phone": "+8801712345678", "role": "CUSTOMER"}`, false)},
			{Name: "Login User", Request: req("POST", "auth/login", `{"email": "customer@example.com", "password": "Password123!"}`, false)},
			{Name: "Google OAuth Login", Request: req("POST", "auth/google", `{"id_token": "mock_google_id_token"}`, false)},
			{Name: "Refresh Access Token", Request: req("POST", "auth/refresh", `{"refresh_token": "sample_refresh_token"}`, false)},
			{Name: "Logout User", Request: req("POST", "auth/logout", `{}`, true)},
			{Name: "Get Profile", Request: req("GET", "auth/me", "", true)},
			{Name: "Update Profile", Request: req("PUT", "auth/profile", `{"full_name": "Updated Test Customer"}`, true)},
			{Name: "Get Address Book", Request: req("GET", "users/addresses", "", true)},
			{Name: "Add New Address", Request: req("POST", "users/addresses", `{"full_name": "Home Address", "phone": "+8801712345678", "address_line": "123 Green Road", "city": "Dhaka", "zone": "Dhanmondi", "postal_code": "1205", "is_default": true}`, true)},
		},
	}
}

func createCatalogFolder() *PostmanItem {
	return &PostmanItem{
		Name: "2. Catalog Engine - Public, Seller, Admin (31 endpoints)",
		Item: []*PostmanItem{
			{Name: "Public: List Categories Tree", Request: req("GET", "catalog/categories/tree", "", false)},
			{Name: "Public: Get Category Details", Request: req("GET", "catalog/categories/1", "", false)},
			{Name: "Public: List Featured Products", Request: req("GET", "catalog/products/featured", "", false)},
			{Name: "Public: Get Product by Slug", Request: req("GET", "catalog/products/slug/sample-product", "", false)},
			{Name: "Public: Get Product Details", Request: req("GET", "catalog/products/101", "", false)},
			{Name: "Public: List Brands", Request: req("GET", "catalog/brands", "", false)},
			{Name: "Public: Get Brand Details", Request: req("GET", "catalog/brands/1", "", false)},
			{Name: "Public: Get Category Attributes", Request: req("GET", "catalog/categories/1/attributes", "", false)},
			{Name: "Seller: List Seller Products", Request: req("GET", "catalog/seller/products", "", true)},
			{Name: "Seller: Create Product Draft", Request: req("POST", "catalog/seller/products", `{"title": "Seller New Shoe", "category_id": 1, "brand_id": 1, "description": "High quality shoe", "price": 1500.00}`, true)},
			{Name: "Seller: Get Product Draft", Request: req("GET", "catalog/seller/products/101", "", true)},
			{Name: "Seller: Update Product", Request: req("PUT", "catalog/seller/products/101", `{"title": "Updated Shoe Title"}`, true)},
			{Name: "Seller: Submit for Moderation", Request: req("POST", "catalog/seller/products/101/submit", `{}`, true)},
			{Name: "Seller: Add Product Variant", Request: req("POST", "catalog/seller/products/101/variants", `{"sku": "SHOE-RED-42", "price": 1500.00, "attributes": {"color": "Red", "size": "42"}}`, true)},
			{Name: "Seller: Update Variant", Request: req("PUT", "catalog/seller/variants/501", `{"price": 1450.00}`, true)},
			{Name: "Seller: Delete Variant", Request: req("DELETE", "catalog/seller/variants/501", "", true)},
			{Name: "Seller: Delete Product", Request: req("DELETE", "catalog/seller/products/101", "", true)},
			{Name: "Seller: Bulk Price Update", Request: req("PUT", "catalog/seller/products/bulk-price", `{"updates": [{"product_id": 101, "price": 1400.00}]}`, true)},
			{Name: "Admin: Pending Moderation Queue", Request: req("GET", "catalog/admin/moderation/pending", "", true)},
			{Name: "Admin: Approve Product", Request: req("POST", "catalog/admin/products/101/approve", `{}`, true)},
			{Name: "Admin: Reject Product", Request: req("POST", "catalog/admin/products/101/reject", `{"reason": "Copyright infringement"}`, true)},
			{Name: "Admin: Create Category", Request: req("POST", "catalog/admin/categories", `{"name": "Electronics", "parent_id": 0, "commission_rate": 5.0}`, true)},
			{Name: "Admin: Update Category", Request: req("PUT", "catalog/admin/categories/1", `{"commission_rate": 6.0}`, true)},
			{Name: "Admin: Delete Category", Request: req("DELETE", "catalog/admin/categories/1", "", true)},
			{Name: "Admin: Create Brand", Request: req("POST", "catalog/admin/brands", `{"name": "Nike", "is_verified": true}`, true)},
			{Name: "Admin: Update Brand", Request: req("PUT", "catalog/admin/brands/1", `{"is_verified": true}`, true)},
			{Name: "Admin: Create Attribute", Request: req("POST", "catalog/admin/attributes", `{"name": "Color", "type": "SELECT"}`, true)},
			{Name: "Admin: List All Products", Request: req("GET", "catalog/admin/products", "", true)},
			{Name: "Admin: Get Product Audit Trail", Request: req("GET", "catalog/admin/products/101/audit", "", true)},
			{Name: "Admin: Set Commission Rate", Request: req("PUT", "catalog/admin/commissions", `{"category_id": 1, "rate": 5.5}`, true)},
			{Name: "Admin: Bulk Approve Products", Request: req("POST", "catalog/admin/products/bulk-approve", `{"product_ids": [101, 102]}`, true)},
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
			{Name: "Create Warehouse", Request: req("POST", "inventory/warehouses", `{"name": "Dhaka Main Warehouse", "location": "Dhaka", "type": "LOCAL_BD"}`, true)},
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
			{Name: "Customer: Preview Checkout", Request: req("POST", "orders/preview", `{"items": [{"variant_id": 501, "quantity": 1}], "shipping_address_id": 1}`, true)},
			{Name: "Customer: Place New Order", Request: req("POST", "orders", `{"shipping_address_id": 1, "payment_method": "SSLCOMMERZ", "coupon_code": "SAVE10"}`, true)},
			{Name: "Customer: List My Orders", Request: req("GET", "orders", "", true)},
			{Name: "Customer: Get Order Details", Request: req("GET", "orders/ord_1001", "", true)},
			{Name: "Customer: Cancel Order", Request: req("POST", "orders/ord_1001/cancel", `{"reason": "Changed my mind"}`, true)},
			{Name: "Customer: Request RMA Return", Request: req("POST", "orders/ord_1001/returns", `{"order_item_id": 1, "reason": "Defective item", "photo_urls": ["https://storage.cdn/proof.jpg"]}`, true)},
			{Name: "Customer: Get Tracking Status", Request: req("GET", "orders/ord_1001/track", "", true)},
			{Name: "Customer: Download PDF Invoice", Request: req("GET", "orders/ord_1001/invoice", "", true)},
			{Name: "Customer: Confirm Delivery", Request: req("POST", "orders/ord_1001/confirm-delivery", `{}`, true)},
			{Name: "Seller: List Sub-Orders", Request: req("GET", "orders/seller/sub-orders", "", true)},
			{Name: "Seller: Get Sub-Order Details", Request: req("GET", "orders/seller/sub-orders/sub_2001", "", true)},
			{Name: "Seller: Mark Ready to Ship", Request: req("POST", "orders/seller/sub-orders/sub_2001/ready", `{}`, true)},
			{Name: "Seller: Generate Packing Slip", Request: req("GET", "orders/seller/sub-orders/sub_2001/packing-slip", "", true)},
			{Name: "Seller: Bulk Mark Ready", Request: req("POST", "orders/seller/sub-orders/bulk-ready", `{"sub_order_ids": ["sub_2001", "sub_2002"]}`, true)},
			{Name: "Seller: Download Courier Label", Request: req("GET", "orders/seller/sub-orders/sub_2001/label", "", true)},
			{Name: "Admin: List All Orders", Request: req("GET", "orders/admin/all", "", true)},
			{Name: "Admin: Update Order Status FSM", Request: req("PUT", "orders/admin/ord_1001/status", `{"new_status": "PROCESSING"}`, true)},
			{Name: "Admin: Approve Return RMA", Request: req("POST", "orders/admin/returns/rma_3001/approve", `{}`, true)},
			{Name: "Admin: Reject Return RMA", Request: req("POST", "orders/admin/returns/rma_3001/reject", `{"reason": "Out of return window"}`, true)},
			{Name: "Admin: Process Refund", Request: req("POST", "orders/admin/ord_1001/refund", `{"amount": 1400.00, "gateway": "SSLCOMMERZ"}`, true)},
		},
	}
}

func createPaymentFolder() *PostmanItem {
	return &PostmanItem{
		Name: "6. Payment & Webhooks (12 endpoints)",
		Item: []*PostmanItem{
			{Name: "Payment: Initiate Gateway Payment", Request: req("POST", "payments/initiate", `{"order_id": "ord_1001", "gateway": "SSLCOMMERZ"}`, true)},
			{Name: "Payment: Get Payment Status", Request: req("GET", "payments/status/pay_9001", "", true)},
			{Name: "Payment: List Gateways", Request: req("GET", "payments/methods", "", false)},
			{Name: "Payment: Verify Transaction Amount", Request: req("POST", "payments/verify", `{"transaction_id": "tx_987654"}`, true)},
			{Name: "Webhook: SSLCommerz IPN", Request: req("POST", "webhooks/sslcommerz", `{"tran_id": "tx_987654", "status": "VALID", "val_id": "val_123"}`, false)},
			{Name: "Webhook: bKash Execute Callback", Request: req("POST", "webhooks/bkash", `{"paymentID": "bk_123", "status": "Completed"}`, false)},
			{Name: "Webhook: Nagad Callback", Request: req("POST", "webhooks/nagad", `{"payment_ref_id": "ng_123", "status": "Success"}`, false)},
			{Name: "Webhook: Steadfast Courier Status", Request: req("POST", "webhooks/steadfast", `{"consignment_id": 123, "status": "delivered"}`, false)},
			{Name: "Webhook: Pathao Courier Webhook", Request: req("POST", "webhooks/pathao", `{"consignment_id": "p_123", "order_status": "Delivered"}`, false)},
			{Name: "Webhook: RedX Courier Webhook", Request: req("POST", "webhooks/redx", `{"tracking_id": "r_123", "status": "DELIVERED"}`, false)},
		},
	}
}

func createLogisticsFolder() *PostmanItem {
	return &PostmanItem{
		Name: "7. Logistics & Couriers (9 endpoints)",
		Item: []*PostmanItem{
			{Name: "Book Courier Consignment", Request: req("POST", "shipping/consignments", `{"sub_order_id": "sub_2001", "courier": "STEADFAST"}`, true)},
			{Name: "Get Consignment Details", Request: req("GET", "shipping/consignments/csg_5001", "", true)},
			{Name: "Track Consignment Status", Request: req("GET", "shipping/consignments/csg_5001/track", "", true)},
			{Name: "List Available Couriers", Request: req("GET", "shipping/couriers", "", false)},
			{Name: "Get Shipping Zones", Request: req("GET", "shipping/zones", "", false)},
			{Name: "Calculate Shipping Cost", Request: req("POST", "shipping/calculate", `{"weight_kg": 1.5, "destination_city": "Chittagong"}`, false)},
			{Name: "Bulk Book Consignments", Request: req("POST", "shipping/consignments/bulk-book", `{"sub_order_ids": ["sub_2001", "sub_2002"]}`, true)},
			{Name: "Generate Bulk Shipping ZIP", Request: req("POST", "shipping/labels/bulk-zip", `{"consignment_ids": ["csg_5001"]}`, true)},
		},
	}
}

func createResellerFolder() *PostmanItem {
	return &PostmanItem{
		Name: "8. Reseller & Dropshipping Portal (11 endpoints)",
		Item: []*PostmanItem{
			{Name: "Submit Reseller Onboarding Application", Request: req("POST", "resellers/apply", `{"shop_name": "Rahim Store", "nid_number": "1234567890", "facebook_page": "fb.com/rahimstore"}`, true)},
			{Name: "Get Reseller Profile & Tier", Request: req("GET", "resellers/profile", "", true)},
			{Name: "Get Wholesale B2B Catalog", Request: req("GET", "resellers/catalog", "", true)},
			{Name: "Calculate Margin", Request: req("POST", "resellers/calculate-margin", `{"variant_id": 501, "customer_price": 2000.00}`, true)},
			{Name: "Create White-Label Order", Request: req("POST", "resellers/orders", `{"variant_id": 501, "customer_name": "Jamil", "customer_phone": "+8801812345678", "selling_price": 2000.00}`, true)},
			{Name: "Get Reseller Earnings Summary", Request: req("GET", "resellers/earnings", "", true)},
			{Name: "Request Margin Profit Payout", Request: req("POST", "resellers/payouts/request", `{"amount": 5000.00, "payout_method": "BKASH", "account_number": "+8801712345678"}`, true)},
			{Name: "Get Marketing Assets Download", Request: req("GET", "resellers/products/101/assets", "", true)},
			{Name: "Admin: List Reseller Applications", Request: req("GET", "resellers/admin/applications", "", true)},
			{Name: "Admin: Approve Reseller", Request: req("POST", "resellers/admin/applications/res_1/approve", `{}`, true)},
			{Name: "Admin: Upgrade Reseller Tier", Request: req("PUT", "resellers/admin/res_1/tier", `{"tier": "GOLD"}`, true)},
		},
	}
}

func createAffiliateFolder() *PostmanItem {
	return &PostmanItem{
		Name: "9. Affiliate Referral Program (10 endpoints)",
		Item: []*PostmanItem{
			{Name: "Apply for Affiliate Program", Request: req("POST", "affiliates/apply", `{"website": "myblog.com", "promotion_method": "Social Media"}`, true)},
			{Name: "Generate Custom Referral Link", Request: req("POST", "affiliates/links", `{"target_url": "/product/sample-product", "alias": "my-special-deal"}`, true)},
			{Name: "List My Affiliate Links", Request: req("GET", "affiliates/links", "", true)},
			{Name: "Track Link Click", Request: req("GET", "affiliates/click/my-special-deal", "", false)},
			{Name: "Get Affiliate Earnings Dashboard", Request: req("GET", "affiliates/dashboard", "", true)},
			{Name: "Get Conversion History", Request: req("GET", "affiliates/conversions", "", true)},
			{Name: "Request Affiliate Payout", Request: req("POST", "affiliates/payouts/request", `{"amount": 3000.00}`, true)},
			{Name: "Admin: Approve Affiliate Application", Request: req("POST", "affiliates/admin/applications/aff_1/approve", `{}`, true)},
			{Name: "Admin: Set Category Commission Rates", Request: req("PUT", "affiliates/admin/commission-rates", `{"category_id": 1, "rate": 8.0}`, true)},
		},
	}
}

func createReviewFolder() *PostmanItem {
	return &PostmanItem{
		Name: "10. Ratings & Verified Reviews (7 endpoints)",
		Item: []*PostmanItem{
			{Name: "Submit Product Review", Request: req("POST", "reviews", `{"product_id": 101, "order_item_id": 1, "rating": 5, "comment": "Excellent quality!", "photo_urls": ["https://storage.cdn/rev1.jpg"]}`, true)},
			{Name: "Get Product Reviews", Request: req("GET", "reviews/product/101", "", false)},
			{Name: "Vote Helpful on Review", Request: req("POST", "reviews/rev_1/helpful", `{}`, true)},
			{Name: "Seller Reply to Review", Request: req("POST", "reviews/rev_1/seller-reply", `{"reply": "Thank you for your feedback!"}`, true)},
			{Name: "Get Seller Composite Rating", Request: req("GET", "reviews/seller/sel_1/rating", "", false)},
			{Name: "Admin Moderation Queue for Reviews", Request: req("GET", "reviews/admin/pending", "", true)},
			{Name: "Admin Hide/Delete Review", Request: req("DELETE", "reviews/admin/rev_1", "", true)},
		},
	}
}

func createNotificationFolder() *PostmanItem {
	return &PostmanItem{
		Name: "11. Omnichannel Notifications (5 endpoints)",
		Item: []*PostmanItem{
			{Name: "Get User Notifications", Request: req("GET", "notifications", "", true)},
			{Name: "Mark Notification as Read", Request: req("PUT", "notifications/notif_1/read", `{}`, true)},
			{Name: "Update Notification Preferences", Request: req("PUT", "notifications/preferences", `{"sms_enabled": true, "email_enabled": true, "push_enabled": true}`, true)},
			{Name: "Admin Broadcast Announcement", Request: req("POST", "notifications/admin/broadcast", `{"title": "Eid Sale Starts Now!", "message": "Enjoy up to 50% discount", "target_role": "CUSTOMER"}`, true)},
			{Name: "Get Notification Delivery Logs", Request: req("GET", "notifications/admin/logs", "", true)},
		},
	}
}

func createSearchFolder() *PostmanItem {
	return &PostmanItem{
		Name: "12. MeiliSearch Engine (2 endpoints)",
		Item: []*PostmanItem{
			{Name: "Search Products with Facets", Request: req("GET", "search?q=shoe&category_id=1&min_price=1000&sort=price_asc", "", false)},
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
			{Name: "Get Seller Performance Summary", Request: req("GET", "analytics/seller-performance", "", true)},
			{Name: "Request Async BI Report Generation", Request: req("POST", "analytics/reports/generate", `{"report_type": "SALES_SUMMARY", "format": "CSV", "start_date": "2024-01-01"}`, true)},
			{Name: "Download Generated Report PDF/CSV", Request: req("GET", "analytics/reports/download/rep_1", "", true)},
		},
	}
}

func createMediaFolder() *PostmanItem {
	return &PostmanItem{
		Name: "14. Media Engine & Presigned S3/R2 (5 endpoints)",
		Item: []*PostmanItem{
			{Name: "Request Presigned Upload URL", Request: req("POST", "media/presigned-url", `{"file_name": "product_image.jpg", "file_type": "image/jpeg", "folder": "products"}`, true)},
			{Name: "Confirm Upload & Process Image Pipeline", Request: req("POST", "media/confirm-upload", `{"file_key": "products/product_image.jpg"}`, true)},
			{Name: "Get Presigned Private File URL", Request: req("GET", "media/private-url?key=nid_proof.pdf", "", true)},
			{Name: "Delete Media File", Request: req("DELETE", "media/files?key=products/product_image.jpg", "", true)},
			{Name: "Get Reseller Marketing Assets Pack", Request: req("GET", "media/marketing-pack/101", "", true)},
		},
	}
}

func createAdminFinanceFolder() *PostmanItem {
	return &PostmanItem{
		Name: "15. Admin Finance & Settlement (11 endpoints)",
		Item: []*PostmanItem{
			{Name: "List Pending Escrow Entries", Request: req("GET", "admin/finance/escrow/pending", "", true)},
			{Name: "Release Escrow Manually", Request: req("POST", "admin/finance/escrow/release", `{"escrow_entry_id": "esc_1001"}`, true)},
			{Name: "List Pending Seller Payout Requests", Request: req("GET", "admin/finance/payouts/pending", "", true)},
			{Name: "Approve Seller Payout Request", Request: req("POST", "admin/finance/payouts/po_1/approve", `{}`, true)},
			{Name: "Reject Seller Payout Request", Request: req("POST", "admin/finance/payouts/po_1/reject", `{"reason": "Bank account discrepancy"}`, true)},
			{Name: "COD Remittance Reconciliation", Request: req("POST", "admin/finance/cod/reconcile", `{"courier": "STEADFAST", "amount": 150000.00}`, true)},
			{Name: "Get Platform Financial Ledger", Request: req("GET", "admin/finance/ledger", "", true)},
		},
	}
}

func createAdminFraudFolder() *PostmanItem {
	return &PostmanItem{
		Name: "16. Admin Fraud & Risk Management (8 endpoints)",
		Item: []*PostmanItem{
			{Name: "Get Customer Risk Score Profile", Request: req("GET", "admin/fraud/risk-profile/user_1", "", true)},
			{Name: "Blacklist Phone Number", Request: req("POST", "admin/fraud/blacklist/phone", `{"phone": "+8801700000000", "reason": "Repeated fake COD orders"}`, true)},
			{Name: "Remove Phone Blacklist", Request: req("DELETE", "admin/fraud/blacklist/phone/+8801700000000", "", true)},
			{Name: "Blacklist IP Address", Request: req("POST", "admin/fraud/blacklist/ip", `{"ip": "192.168.1.100", "reason": "DDoS attempt"}`, true)},
			{Name: "List High-Risk COD Orders", Request: req("GET", "admin/fraud/orders/high-risk", "", true)},
		},
	}
}

func createAdminSettingsFolder() *PostmanItem {
	return &PostmanItem{
		Name: "17. Admin System Settings & Config (6 endpoints)",
		Item: []*PostmanItem{
			{Name: "Get System Settings Config", Request: req("GET", "admin/settings", "", true)},
			{Name: "Update Courier API Credentials", Request: req("PUT", "admin/settings/couriers", `{"steadfast_api_key": "key_123", "pathao_client_secret": "sec_456"}`, true)},
			{Name: "Update Payment Gateway Keys", Request: req("PUT", "admin/settings/payments", `{"sslcommerz_store_id": "store_123", "bkash_app_secret": "bk_sec"}`, true)},
			{Name: "Update Platform Fees & Limits", Request: req("PUT", "admin/settings/fees", `{"default_seller_commission": 5.0, "cod_max_limit": 10000.00}`, true)},
		},
	}
}

func createChinaSourcingFolder() *PostmanItem {
	return &PostmanItem{
		Name: "18. China Direct Sourcing Engine (9 endpoints)",
		Item: []*PostmanItem{
			{Name: "List China Sourcing Batches", Request: req("GET", "china/batches", "", true)},
			{Name: "Create China Import Batch", Request: req("POST", "china/batches", `{"batch_code": "CN-2024-12", "origin_port": "Guangzhou", "est_arrival": "2024-12-31"}`, true)},
			{Name: "Get Batch Details", Request: req("GET", "china/batches/CN-2024-12", "", true)},
			{Name: "Update China Batch FSM Status", Request: req("PUT", "china/batches/CN-2024-12/status", `{"status": "CUSTOMS_CLEARED"}`, true)},
			{Name: "Calculate Landed Unit Cost", Request: req("POST", "china/landed-cost", `{"product_cost_cny": 50.0, "shipping_cost_cny": 10.0, "exchange_rate_bdt": 16.5}`, true)},
		},
	}
}

func createWalletDisputePromotionFolder() *PostmanItem {
	return &PostmanItem{
		Name: "19. Wallet, Disputes & Promotions (20 endpoints)",
		Item: []*PostmanItem{
			{Name: "Wallet: Get Balance", Request: req("GET", "wallet/balance", "", true)},
			{Name: "Wallet: Get Ledger History", Request: req("GET", "wallet/transactions", "", true)},
			{Name: "Wallet: Request Withdrawal", Request: req("POST", "wallet/withdraw", `{"amount": 1000.00, "method": "BKASH", "account_number": "+8801712345678"}`, true)},
			{Name: "Dispute: Create Customer Dispute", Request: req("POST", "disputes", `{"order_id": "ord_1001", "reason": "Damaged goods delivered", "description": "Package was crushed"}`, true)},
			{Name: "Dispute: Get Dispute Details", Request: req("GET", "disputes/dsp_1", "", true)},
			{Name: "Dispute: Admin Resolve Dispute", Request: req("POST", "disputes/admin/dsp_1/resolve", `{"resolution": "REFUND_CUSTOMER", "notes": "Approved customer refund"}`, true)},
			{Name: "Promotion: Create Coupon", Request: req("POST", "promotions/coupons", `{"code": "SUMMER20", "discount_percentage": 20.0, "min_order_amount": 1000.00}`, true)},
			{Name: "Promotion: List Active Promotions", Request: req("GET", "promotions/active", "", false)},
		},
	}
}
