# ============================================================
#  FULL API TEST - ALL TABLES DATA FILL
#  3 Users: Customer | Reseller/Affiliate | Seller+Admin
# ============================================================

$BASE           = "http://localhost:8080/api/v1"
$CUST_TOKEN     = "acc_3e014eef-9c6e-4cfb-b38d-72446dff1ded_1790238767"
$CUST_ID        = "3e014eef-9c6e-4cfb-b38d-72446dff1ded"
$RESELLER_TOKEN = "acc_fb6d7df4-5c46-4e65-a0d7-b84d2a5ea047_1790238836"
$RESELLER_ID    = "fb6d7df4-5c46-4e65-a0d7-b84d2a5ea047"
$SELLER_TOKEN   = "acc_2d73929b-c2d1-4e14-9fd5-84faa87367cc_1790238837"
$SELLER_ID      = "2d73929b-c2d1-4e14-9fd5-84faa87367cc"
$ADMIN_TOKEN    = "acc_ae3a83cd-44fc-4d74-8c6b-c136e01509e9_1790238838"
$ADMIN_ID       = "ae3a83cd-44fc-4d74-8c6b-c136e01509e9"

$global:pass = 0
$global:fail = 0
$global:total = 0
$global:Results = [System.Collections.ArrayList]@()

function MkH($token) {
    $h = @{ "Content-Type" = "application/json" }
    if ($token) { $h["Authorization"] = "Bearer $token" }
    return $h
}

function Call-API {
    param($method, $url, $token="", $body="", $expect=@(200,201,204))
    $headers = MkH $token
    try {
        $p = @{ Method=$method; Uri=$url; Headers=$headers; UseBasicParsing=$true; ErrorAction="Stop" }
        if ($body -and $method -ne "GET") { $p["Body"] = $body }
        $r = Invoke-WebRequest @p
        return @{ code=$r.StatusCode; data=($r.Content | ConvertFrom-Json -ErrorAction SilentlyContinue) }
    } catch {
        $code = [int]$_.Exception.Response.StatusCode
        $data = $_.ErrorDetails.Message | ConvertFrom-Json -ErrorAction SilentlyContinue
        return @{ code=$code; data=$data }
    }
}

function Log-Test {
    param($section, $name, $user, $desc, $result, $expect=@(200,201))
    $global:total++
    $ok = $expect -contains $result.code
    if ($ok) { $global:pass++ } else { $global:fail++ }
    $icon = if ($ok) { "[PASS]" } else { "[FAIL]" }
    $color = if ($ok) { "Green" } else { "Red" }
    Write-Host ("  {0} [{1}] HTTP {2}  | {3}" -f $icon, $name, $result.code, $user) -ForegroundColor $color
    $global:Results.Add([PSCustomObject]@{
        Section=$section; Name=$name; User=$user; Status=if($ok){"PASS"}else{"FAIL"}
        HTTP=$result.code; Description=$desc
    }) | Out-Null
}

$TS = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()

Write-Host "`n================================================================" -ForegroundColor Cyan
Write-Host "  FULL API RUN — ALL DB TABLES DATA FILL" -ForegroundColor Cyan
Write-Host "================================================================" -ForegroundColor Cyan
Write-Host "  Customer  : customer@ecom.test   | ID: $CUST_ID"
Write-Host "  Reseller  : reseller2@ecom.test  | ID: $RESELLER_ID"
Write-Host "  Seller    : seller@ecom.test     | ID: $SELLER_ID"
Write-Host "  SuperAdmin: superadmin@ecom.test | ID: $ADMIN_ID"
Write-Host "================================================================`n" -ForegroundColor Cyan

# ================================================================
# [1] AUTH — otp_requests, sessions, users tables
# ================================================================
Write-Host "[1] AUTH MODULE" -ForegroundColor Yellow

# OTP send
$b = '{"target":"01711000001","purpose":"LOGIN"}'
$r = Call-API "POST" "$BASE/auth/otp/send" "" $b
Log-Test "AUTH" "Send OTP" "Customer" "otp_requests table-এ store হয়" $r @(200,201)

# Email Login Customer
$b = '{"email":"customer@ecom.test","password":"customer456"}'
$r = Call-API "POST" "$BASE/auth/email/login" "" $b
Log-Test "AUTH" "Email Login (Customer)" "Customer" "sessions table-এ refresh_token store হয়" $r @(200,201)
if ($r.data.data.access_token) { $CUST_TOKEN = $r.data.data.access_token }

# Email Login Reseller
$b = '{"email":"reseller2@ecom.test","password":"reseller123"}'
$r = Call-API "POST" "$BASE/auth/email/login" "" $b
Log-Test "AUTH" "Email Login (Reseller)" "Reseller" "sessions table-এ refresh_token store হয়" $r @(200,201)
if ($r.data.data.access_token) { $RESELLER_TOKEN = $r.data.data.access_token }

# Email Login Seller
$b = '{"email":"seller@ecom.test","password":"seller123"}'
$r = Call-API "POST" "$BASE/auth/email/login" "" $b
Log-Test "AUTH" "Email Login (Seller)" "Seller" "sessions table-এ refresh_token store হয়" $r @(200,201)
if ($r.data.data.access_token) { $SELLER_TOKEN = $r.data.data.access_token }

# Email Login Admin
$b = '{"email":"superadmin@ecom.test","password":"admin123"}'
$r = Call-API "POST" "$BASE/auth/email/login" "" $b
Log-Test "AUTH" "Email Login (Admin)" "Admin" "sessions table-এ refresh_token store হয়" $r @(200,201)
if ($r.data.data.access_token) { $ADMIN_TOKEN = $r.data.data.access_token }

# Get Me
$r = Call-API "GET" "$BASE/auth/me" $CUST_TOKEN
Log-Test "AUTH" "Get Me (Customer)" "Customer" "users table থেকে profile" $r @(200)

# Get Sessions
$r = Call-API "GET" "$BASE/auth/sessions" $CUST_TOKEN
Log-Test "AUTH" "Get Sessions" "Customer" "sessions table সব active sessions" $r @(200)

# Password Reset Request
$b = '{"email":"customer@ecom.test"}'
$r = Call-API "POST" "$BASE/auth/password/reset-request" "" $b
Log-Test "AUTH" "Password Reset Request" "Customer" "otp_requests table-এ PURPOSE=PASSWORD_RESET" $r @(200,201)

# Logout + Re-login
$r = Call-API "POST" "$BASE/auth/logout" $CUST_TOKEN
Log-Test "AUTH" "Logout (Customer)" "Customer" "sessions revoke হয়" $r @(200)
$b = '{"email":"customer@ecom.test","password":"customer456"}'
$r2 = Call-API "POST" "$BASE/auth/email/login" "" $b
if ($r2.data.data.access_token) { $CUST_TOKEN = $r2.data.data.access_token }

# Register new session test
$b = '{"email":"customer@ecom.test","password":"customer456"}'
$r = Call-API "POST" "$BASE/auth/email/register" "" $b @(200,201,409)
Log-Test "AUTH" "Register Duplicate (409 expected)" "Customer" "Duplicate email → 409" $r @(409,400)

# ================================================================
# [2] USER — users, user_addresses, user_kyc tables
# ================================================================
Write-Host "`n[2] USER MODULE" -ForegroundColor Yellow

$r = Call-API "GET" "$BASE/user/profile" $CUST_TOKEN
Log-Test "USER" "Get Profile (Customer)" "Customer" "users table: id,email,phone,full_name,role,status" $r @(200)

$b = '{"full_name":"Rahim Ahmed Updated","avatar_url":"https://cdn.ecom.test/avatar1.jpg","phone":"01711000001"}'
$r = Call-API "PUT" "$BASE/user/profile" $CUST_TOKEN $b
Log-Test "USER" "Update Profile (Customer)" "Customer" "users: full_name, profile_picture_url, phone update" $r @(200)

$r = Call-API "GET" "$BASE/user/profile" $RESELLER_TOKEN
Log-Test "USER" "Get Profile (Reseller)" "Reseller" "users table: Reseller profile" $r @(200)

$r = Call-API "GET" "$BASE/user/profile" $SELLER_TOKEN
Log-Test "USER" "Get Profile (Seller)" "Seller" "users table: Seller profile" $r @(200)

$r = Call-API "GET" "$BASE/user/profile" $ADMIN_TOKEN
Log-Test "USER" "Get Profile (Admin)" "Admin" "users table: Admin profile" $r @(200)

# Create Addresses — user_addresses table
$b = '{"recipient_name":"Rahim Ahmed","recipient_phone":"01711000001","address_line1":"123 Dhanmondi","division":"Dhaka","district":"Dhaka","upazila":"Dhanmondi","postal_code":"1205","label":"HOME","is_default":true}'
$r = Call-API "POST" "$BASE/user/addresses" $CUST_TOKEN $b
Log-Test "USER" "Create Address HOME (Customer)" "Customer" "user_addresses table insert, is_default=true" $r @(200,201)
$homeAddrId = $r.data.data.address_id

$b = '{"recipient_name":"Office Rahim","recipient_phone":"01711000001","address_line1":"456 Motijheel","division":"Dhaka","district":"Dhaka","postal_code":"1000","label":"OFFICE","is_default":false}'
$r = Call-API "POST" "$BASE/user/addresses" $CUST_TOKEN $b
Log-Test "USER" "Create Address OFFICE (Customer)" "Customer" "user_addresses table 2nd address" $r @(200,201)
$officeAddrId = $r.data.data.address_id

# Reseller address
$b = '{"recipient_name":"Karim Reseller","recipient_phone":"01722000099","address_line1":"789 Mirpur Road","division":"Dhaka","district":"Dhaka","postal_code":"1216","label":"HOME","is_default":true}'
$r = Call-API "POST" "$BASE/user/addresses" $RESELLER_TOKEN $b
Log-Test "USER" "Create Address (Reseller)" "Reseller" "user_addresses: Reseller home address" $r @(200,201)

# Seller address
$b = '{"recipient_name":"Selim Seller","recipient_phone":"01744000099","address_line1":"10 Old DOHS","division":"Dhaka","district":"Dhaka","postal_code":"1206","label":"HOME","is_default":true}'
$r = Call-API "POST" "$BASE/user/addresses" $SELLER_TOKEN $b
Log-Test "USER" "Create Address (Seller)" "Seller" "user_addresses: Seller address" $r @(200,201)

$r = Call-API "GET" "$BASE/user/addresses" $CUST_TOKEN
Log-Test "USER" "Get Addresses (Customer)" "Customer" "user_addresses: Customer-এর সব addresses" $r @(200)

if ($officeAddrId) {
    $b = '{"recipient_name":"Office Updated","recipient_phone":"01711000001","address_line1":"456 Motijheel Ave","division":"Dhaka","district":"Dhaka","label":"OFFICE"}'
    $r = Call-API "PUT" "$BASE/user/addresses/$officeAddrId" $CUST_TOKEN $b
    Log-Test "USER" "Update Address (Customer)" "Customer" "user_addresses: address update" $r @(200)

    $r = Call-API "PUT" "$BASE/user/addresses/$officeAddrId/default" $CUST_TOKEN
    Log-Test "USER" "Set Default Address (Customer)" "Customer" "user_addresses: is_default toggle" $r @(200)
}

# KYC — user_kyc table
$b = '{"document_type":"NID","document_number":"1234567890","front_image_url":"https://cdn.ecom.test/nid_front.jpg","back_image_url":"https://cdn.ecom.test/nid_back.jpg","selfie_image_url":"https://cdn.ecom.test/selfie.jpg"}'
$r = Call-API "POST" "$BASE/user/kyc/submit" $CUST_TOKEN $b
Log-Test "USER" "Submit KYC (Customer-NID)" "Customer" "user_kyc table: status=PENDING" $r @(200,201)

$b = '{"document_type":"PASSPORT","document_number":"AB1234567","front_image_url":"https://cdn.ecom.test/passport.jpg","back_image_url":"","selfie_image_url":"https://cdn.ecom.test/selfie2.jpg"}'
$r = Call-API "POST" "$BASE/user/kyc/submit" $RESELLER_TOKEN $b
Log-Test "USER" "Submit KYC (Reseller-Passport)" "Reseller" "user_kyc table: Reseller KYC" $r @(200,201)

$b = '{"document_type":"TRADE_LICENSE","document_number":"TL-2024-001","front_image_url":"https://cdn.ecom.test/trade.jpg","back_image_url":"","selfie_image_url":""}'
$r = Call-API "POST" "$BASE/user/kyc/submit" $SELLER_TOKEN $b
Log-Test "USER" "Submit KYC (Seller-TradeLicense)" "Seller" "user_kyc table: Seller business KYC" $r @(200,201)

# Admin reviews KYC
$b = '{"status":"VERIFIED","rejection_reason":""}'
$r = Call-API "PUT" "$BASE/admin/kyc/review/$CUST_ID" $ADMIN_TOKEN $b
Log-Test "USER" "Review KYC VERIFIED (Admin→Customer)" "Admin" "user_kyc: status=VERIFIED" $r @(200)

$b = '{"status":"VERIFIED","rejection_reason":""}'
$r = Call-API "PUT" "$BASE/admin/kyc/review/$RESELLER_ID" $ADMIN_TOKEN $b
Log-Test "USER" "Review KYC VERIFIED (Admin→Reseller)" "Admin" "user_kyc: Reseller VERIFIED" $r @(200)

$b = '{"status":"VERIFIED","rejection_reason":""}'
$r = Call-API "PUT" "$BASE/admin/kyc/review/$SELLER_ID" $ADMIN_TOKEN $b
Log-Test "USER" "Review KYC VERIFIED (Admin→Seller)" "Admin" "user_kyc: Seller VERIFIED" $r @(200)

# ================================================================
# [3] PRODUCT — product_categories, products, product_skus
# ================================================================
Write-Host "`n[3] PRODUCT MODULE" -ForegroundColor Yellow

# Create categories (Admin)
$b = '{"name":"Electronics","slug":"electronics-cat","commission_rate":8.5,"icon_url":"https://cdn.ecom.test/electronics.png"}'
$r = Call-API "POST" "$BASE/admin/categories" $ADMIN_TOKEN $b
Log-Test "PRODUCT" "Create Category: Electronics" "Admin" "product_categories table insert" $r @(200,201,409)
$catElec = $r.data.data.category_id
if (-not $catElec) {
    $r2 = Call-API "GET" "$BASE/categories" ""
    $catElec = ($r2.data.data.categories | Where-Object { $_.slug -eq "electronics-cat" }).id
}

$b = '{"name":"Mobile Phones","slug":"mobile-phones-cat","commission_rate":7.5,"icon_url":"https://cdn.ecom.test/mobile.png"}'
$r = Call-API "POST" "$BASE/admin/categories" $ADMIN_TOKEN $b
Log-Test "PRODUCT" "Create Category: Mobile Phones" "Admin" "product_categories sub-category" $r @(200,201,409)

$b = '{"name":"Fashion & Clothing","slug":"fashion-cat","commission_rate":10.0,"icon_url":"https://cdn.ecom.test/fashion.png"}'
$r = Call-API "POST" "$BASE/admin/categories" $ADMIN_TOKEN $b
Log-Test "PRODUCT" "Create Category: Fashion" "Admin" "product_categories 3rd category" $r @(200,201,409)
$catFashion = $r.data.data.category_id

$r = Call-API "GET" "$BASE/categories" ""
Log-Test "PRODUCT" "Get Categories (Public)" "Public" "product_categories: full tree return" $r @(200)

# Vendor register (Seller)
$b = '{"store_name":"Selim Electronics BD","description":"Premium electronics at best price","division":"Dhaka"}'
$r = Call-API "POST" "$BASE/vendor/register" $SELLER_TOKEN $b
Log-Test "VENDOR" "Register Vendor Store (Seller)" "Seller" "vendor_stores table: new store create" $r @(200,201,409)

# Create Products (Seller)
$slug1 = "samsung-galaxy-a55-$TS"
$b = "{`"category_id`":`"$catElec`",`"title`":`"Samsung Galaxy A55 5G`",`"slug`":`"$slug1`",`"description`":`"Latest Samsung mid-range flagship with 5G`",`"short_description`":`"5G, 50MP Camera, 5000mAh`"}"
$r = Call-API "POST" "$BASE/vendor/products" $SELLER_TOKEN $b
Log-Test "PRODUCT" "Create Product: Samsung A55" "Seller" "products table: vendor_id=SELLER, status=APPROVED" $r @(200,201)
$prod1Id = $r.data.data.product_id
$prod1Slug = $slug1

$slug2 = "iphone-15-pro-$TS"
$b = "{`"category_id`":`"$catElec`",`"title`":`"iPhone 15 Pro 256GB`",`"slug`":`"$slug2`",`"description`":`"Apple iPhone 15 Pro with A17 Bionic chip`",`"short_description`":`"Titanium design, ProMotion display`"}"
$r = Call-API "POST" "$BASE/vendor/products" $SELLER_TOKEN $b
Log-Test "PRODUCT" "Create Product: iPhone 15 Pro" "Seller" "products table: 2nd product" $r @(200,201)
$prod2Id = $r.data.data.product_id

$slug3 = "mens-polo-shirt-$TS"
$b = "{`"category_id`":`"$catFashion`",`"title`":`"Premium Men's Polo Shirt`",`"slug`":`"$slug3`",`"description`":`"100% cotton premium polo shirt for men`",`"short_description`":`"Available in all sizes`"}"
$r = Call-API "POST" "$BASE/vendor/products" $SELLER_TOKEN $b
Log-Test "PRODUCT" "Create Product: Polo Shirt" "Seller" "products table: 3rd product (fashion)" $r @(200,201)
$prod3Id = $r.data.data.product_id

# List & Detail
$r = Call-API "GET" "$BASE/products" ""
Log-Test "PRODUCT" "List Products (Public)" "Public" "products table: approved products list" $r @(200)

$r = Call-API "GET" "$BASE/products?q=samsung&limit=5" ""
Log-Test "PRODUCT" "Search Products (Public)" "Public" "products: ILIKE search" $r @(200)

if ($prod1Slug) {
    $r = Call-API "GET" "$BASE/products/$prod1Slug" ""
    Log-Test "PRODUCT" "Get Product Detail: Samsung" "Public" "products JOIN product_skus JOIN categories" $r @(200)
}

# Add SKUs — product_skus table
if ($prod1Id) {
    $b = '{"sku_code":"SAM-A55-128-BLK","wholesale_price":32000,"retail_price":38000,"stock_quantity":150}'
    $r = Call-API "POST" "$BASE/vendor/products/$prod1Id/skus" $SELLER_TOKEN $b
    Log-Test "PRODUCT" "Add SKU: Samsung A55 128GB Black" "Seller" "product_skus table insert" $r @(200,201)
    $sku1Id = $r.data.data.sku_id

    $b = '{"sku_code":"SAM-A55-256-BLU","wholesale_price":36000,"retail_price":42000,"stock_quantity":80}'
    $r = Call-API "POST" "$BASE/vendor/products/$prod1Id/skus" $SELLER_TOKEN $b
    Log-Test "PRODUCT" "Add SKU: Samsung A55 256GB Blue" "Seller" "product_skus: 2nd variant" $r @(200,201)
    $sku2Id = $r.data.data.sku_id
}

if ($prod2Id) {
    $b = '{"sku_code":"IPH15P-256-BLK","wholesale_price":115000,"retail_price":138000,"stock_quantity":30}'
    $r = Call-API "POST" "$BASE/vendor/products/$prod2Id/skus" $SELLER_TOKEN $b
    Log-Test "PRODUCT" "Add SKU: iPhone 256GB Black" "Seller" "product_skus: iPhone variant" $r @(200,201)
    $skuIPhone = $r.data.data.sku_id
}

if ($prod3Id) {
    $b = '{"sku_code":"POLO-M-RED","wholesale_price":450,"retail_price":650,"stock_quantity":200}'
    $r = Call-API "POST" "$BASE/vendor/products/$prod3Id/skus" $SELLER_TOKEN $b
    Log-Test "PRODUCT" "Add SKU: Polo M Red" "Seller" "product_skus: fashion variant" $r @(200,201)
    $skuPolo = $r.data.data.sku_id
}

# Approve all products (Admin)
if ($prod1Id) {
    $b = '{"approval_status":"APPROVED","admin_note":"Quality verified"}'
    $r = Call-API "PUT" "$BASE/admin/products/$prod1Id/approve" $ADMIN_TOKEN $b
    Log-Test "PRODUCT" "Approve Samsung A55 (Admin)" "Admin" "products: approval_status=APPROVED" $r @(200)
}
if ($prod2Id) {
    $b = '{"approval_status":"APPROVED","admin_note":"Apple products verified"}'
    $r = Call-API "PUT" "$BASE/admin/products/$prod2Id/approve" $ADMIN_TOKEN $b
    Log-Test "PRODUCT" "Approve iPhone 15 Pro (Admin)" "Admin" "products: approval_status=APPROVED" $r @(200)
}
if ($prod3Id) {
    $b = '{"approval_status":"APPROVED","admin_note":"Fashion item verified"}'
    $r = Call-API "PUT" "$BASE/admin/products/$prod3Id/approve" $ADMIN_TOKEN $b
    Log-Test "PRODUCT" "Approve Polo Shirt (Admin)" "Admin" "products: approval_status=APPROVED" $r @(200)
}

# ================================================================
# [4] INVENTORY — inventory, warehouses, inventory_adjustments
# ================================================================
Write-Host "`n[4] INVENTORY MODULE" -ForegroundColor Yellow

$r = Call-API "GET" "$BASE/inventory/stock" $SELLER_TOKEN
Log-Test "INVENTORY" "Get Stock Levels (Seller)" "Seller" "inventory: stock_quantity, reserved_qty" $r @(200)

$r = Call-API "GET" "$BASE/inventory/warehouses" $SELLER_TOKEN
Log-Test "INVENTORY" "Get Warehouses (Seller)" "Seller" "warehouses table list" $r @(200)

$r = Call-API "GET" "$BASE/inventory/reservations" $SELLER_TOKEN
Log-Test "INVENTORY" "Get Reservations (Seller)" "Seller" "inventory_reservations table" $r @(200)

if ($prod1Id) {
    $b = "{`"product_id`":`"$prod1Id`",`"quantity`":50,`"reason`":`"New stock arrived from supplier`",`"warehouse`":`"Main Hub Dhaka`"}"
    $r = Call-API "POST" "$BASE/inventory/adjust" $SELLER_TOKEN $b
    Log-Test "INVENTORY" "Adjust Stock +50 (Samsung)" "Seller" "inventory: +50 adjust, audit log" $r @(200,201)
}

# ================================================================
# [5] CART — cart_items table
# ================================================================
Write-Host "`n[5] CART MODULE" -ForegroundColor Yellow

# Customer cart
$p1 = if ($prod1Id) { $prod1Id } else { "00000000-0000-0000-0000-000000000001" }
$s1 = if ($sku1Id)  { $sku1Id  } else { "00000000-0000-0000-0000-000000000011" }
$p2 = if ($prod2Id) { $prod2Id } else { "00000000-0000-0000-0000-000000000002" }
$s2 = if ($skuIPhone){ $skuIPhone } else { "00000000-0000-0000-0000-000000000022" }
$p3 = if ($prod3Id) { $prod3Id } else { "00000000-0000-0000-0000-000000000003" }
$s3 = if ($skuPolo) { $skuPolo  } else { "00000000-0000-0000-0000-000000000033" }

$b = "{`"product_id`":`"$p1`",`"sku_id`":`"$s1`",`"product_title`":`"Samsung Galaxy A55`",`"unit_price`":38000,`"quantity`":1}"
$r = Call-API "POST" "$BASE/cart/items" $CUST_TOKEN $b
Log-Test "CART" "Add Samsung to Cart (Customer)" "Customer" "cart_items: insert/ON CONFLICT add qty" $r @(200,201)
$cartItem1 = $r.data.data.cart_item_id

$b = "{`"product_id`":`"$p3`",`"sku_id`":`"$s3`",`"product_title`":`"Polo Shirt`",`"unit_price`":650,`"quantity`":3}"
$r = Call-API "POST" "$BASE/cart/items" $CUST_TOKEN $b
Log-Test "CART" "Add Polo to Cart (Customer)" "Customer" "cart_items: 2nd item in cart" $r @(200,201)
$cartItem2 = $r.data.data.cart_item_id

# Reseller cart
$b = "{`"product_id`":`"$p2`",`"sku_id`":`"$s2`",`"product_title`":`"iPhone 15 Pro`",`"unit_price`":138000,`"quantity`":1}"
$r = Call-API "POST" "$BASE/cart/items" $RESELLER_TOKEN $b
Log-Test "CART" "Add iPhone to Cart (Reseller)" "Reseller" "cart_items: Reseller cart" $r @(200,201)

$r = Call-API "GET" "$BASE/cart" $CUST_TOKEN
Log-Test "CART" "Get Cart (Customer)" "Customer" "cart_items: all items, grand_total calculate" $r @(200)

$r = Call-API "GET" "$BASE/cart/summary" $CUST_TOKEN
Log-Test "CART" "Get Cart Summary (Customer)" "Customer" "subtotal, shipping_fee, grand_total" $r @(200)

$r = Call-API "GET" "$BASE/cart/count" $CUST_TOKEN
Log-Test "CART" "Get Cart Count (Customer)" "Customer" "cart_items: SUM(quantity) for badge" $r @(200)

if ($cartItem1) {
    $b = '{"quantity":2}'
    $r = Call-API "PUT" "$BASE/cart/items/$cartItem1" $CUST_TOKEN $b
    Log-Test "CART" "Update Quantity (Samsung→2)" "Customer" "cart_items: quantity update" $r @(200)
}

# ================================================================
# [6] ORDER — orders, sub_orders, order_items tables
# ================================================================
Write-Host "`n[6] ORDER MODULE" -ForegroundColor Yellow

# Customer Checkout
$b = '{"shipping_address":"123 Dhanmondi Road, Dhaka 1205, Bangladesh","payment_method":"COD"}'
$r = Call-API "POST" "$BASE/customer/orders/checkout" $CUST_TOKEN $b
Log-Test "ORDER" "Checkout (Customer)" "Customer" "orders+sub_orders+order_items created. cart cleared" $r @(200,201)
$orderId1 = $r.data.data.order_id

# Reseller checkout
$b = '{"shipping_address":"789 Mirpur Road, Dhaka 1216","payment_method":"BKASH"}'
$r = Call-API "POST" "$BASE/customer/orders/checkout" $RESELLER_TOKEN $b
Log-Test "ORDER" "Checkout (Reseller)" "Reseller" "orders: Reseller order created" $r @(200,201)
$orderId2 = $r.data.data.order_id

# Customer Orders list
$r = Call-API "GET" "$BASE/customer/orders" $CUST_TOKEN
Log-Test "ORDER" "List My Orders (Customer)" "Customer" "orders: customer orders list" $r @(200)

if ($orderId1) {
    $r = Call-API "GET" "$BASE/customer/orders/$orderId1" $CUST_TOKEN
    Log-Test "ORDER" "Order Detail (Customer)" "Customer" "orders+order_items+sub_orders JOIN" $r @(200)

    $r = Call-API "GET" "$BASE/customer/orders/$orderId1/track" $CUST_TOKEN
    Log-Test "ORDER" "Track Order (Customer)" "Customer" "order tracking status timeline" $r @(200)

    $r = Call-API "GET" "$BASE/customer/orders/$orderId1/items" $CUST_TOKEN
    Log-Test "ORDER" "Get Order Items (Customer)" "Customer" "order_items table for this order" $r @(200)

    $r = Call-API "GET" "$BASE/customer/orders/$orderId1/invoice" $CUST_TOKEN
    Log-Test "ORDER" "Get Invoice (Customer)" "Customer" "Invoice URL generated" $r @(200)
}

# Seller order management
$r = Call-API "GET" "$BASE/seller/orders" $SELLER_TOKEN
Log-Test "ORDER" "List Seller Orders" "Seller" "sub_orders: this seller's orders" $r @(200)

$r = Call-API "GET" "$BASE/seller/orders/pending" $SELLER_TOKEN
Log-Test "ORDER" "Seller Pending Orders" "Seller" "sub_orders: status=PENDING" $r @(200)

$r = Call-API "GET" "$BASE/seller/orders/analytics" $SELLER_TOKEN
Log-Test "ORDER" "Seller Order Analytics" "Seller" "revenue, total orders stats" $r @(200)

# Admin order management
$r = Call-API "GET" "$BASE/admin/orders" $ADMIN_TOKEN
Log-Test "ORDER" "List All Orders (Admin)" "Admin" "orders: platform-wide all orders" $r @(200)

$r = Call-API "GET" "$BASE/admin/orders/sub-orders" $ADMIN_TOKEN
Log-Test "ORDER" "List Sub-Orders (Admin)" "Admin" "sub_orders: all vendor sub-orders" $r @(200)

$r = Call-API "GET" "$BASE/admin/orders/analytics" $ADMIN_TOKEN
Log-Test "ORDER" "Admin Order Analytics" "Admin" "GMV, conversion rate, revenue" $r @(200)

$r = Call-API "GET" "$BASE/admin/orders/disputed" $ADMIN_TOKEN
Log-Test "ORDER" "Disputed Orders (Admin)" "Admin" "disputes JOIN orders" $r @(200)

if ($orderId1) {
    $b = '{"status":"PROCESSING"}'
    $r = Call-API "PUT" "$BASE/admin/orders/$orderId1/status" $ADMIN_TOKEN $b
    Log-Test "ORDER" "Update Order Status→PROCESSING (Admin)" "Admin" "orders: status update" $r @(200)

    # Seller pack & ship
    $sellerSubOrderId = ""
    $subOrdResp = Call-API "GET" "$BASE/seller/orders" $SELLER_TOKEN
    $sellerSubOrderId = $subOrdResp.data.data[0].id

    if ($sellerSubOrderId) {
        $r = Call-API "POST" "$BASE/seller/orders/$sellerSubOrderId/pack" $SELLER_TOKEN
        Log-Test "ORDER" "Pack Order (Seller)" "Seller" "sub_orders: status=PACKED" $r @(200,201)

        $r = Call-API "POST" "$BASE/seller/orders/$sellerSubOrderId/ship" $SELLER_TOKEN
        Log-Test "ORDER" "Ship Order (Seller)" "Seller" "sub_orders: status=SHIPPED, tracking created" $r @(200,201)

        $b = '{"tracking_number":"TRK-PATHAO-2024-001","courier":"PATHAO"}'
        $r = Call-API "PUT" "$BASE/seller/orders/$sellerSubOrderId/tracking" $SELLER_TOKEN $b
        Log-Test "ORDER" "Update Tracking (Seller)" "Seller" "sub_orders: tracking_number update" $r @(200)
    }
}

# ================================================================
# [7] PAYMENT — payments, refunds tables
# ================================================================
Write-Host "`n[7] PAYMENT MODULE" -ForegroundColor Yellow

$r = Call-API "GET" "$BASE/payment/methods" ""
Log-Test "PAYMENT" "Get Payment Methods (Public)" "Public" "bKash, Nagad, SSLCommerz, COD list" $r @(200)

if ($orderId1) {
    $b = "{`"order_id`":`"$orderId1`",`"payment_method`":`"BKASH`",`"amount`":78950}"
    $r = Call-API "POST" "$BASE/payment/initiate" $CUST_TOKEN $b
    Log-Test "PAYMENT" "Initiate Payment bKash (Customer)" "Customer" "payments: status=PENDING, gateway URL" $r @(200,201)
    $payId1 = $r.data.data.payment_id

    if ($payId1) {
        $r = Call-API "GET" "$BASE/payment/status/$payId1" ""
        Log-Test "PAYMENT" "Get Payment Status" "Public" "payments: status, method, amount" $r @(200)

        $b = "{`"payment_id`":`"$payId1`",`"transaction_id`":`"BKASH-TXN-$TS`"}"
        $r = Call-API "POST" "$BASE/payment/verify" $CUST_TOKEN $b
        Log-Test "PAYMENT" "Verify Payment (Customer)" "Customer" "payments: status=COMPLETED, order=PROCESSING" $r @(200,201)
    }
}

if ($orderId2) {
    $b = "{`"order_id`":`"$orderId2`",`"payment_method`":`"NAGAD`",`"amount`":138000}"
    $r = Call-API "POST" "$BASE/payment/initiate" $RESELLER_TOKEN $b
    Log-Test "PAYMENT" "Initiate Payment Nagad (Reseller)" "Reseller" "payments: Reseller order payment" $r @(200,201)
    $payId2 = $r.data.data.payment_id

    if ($payId2) {
        $b = "{`"payment_id`":`"$payId2`",`"transaction_id`":`"NAGAD-TXN-$TS`"}"
        $r = Call-API "POST" "$BASE/payment/verify" $RESELLER_TOKEN $b
        Log-Test "PAYMENT" "Verify Payment Nagad (Reseller)" "Reseller" "payments: Nagad verified" $r @(200,201)
    }
}

# Webhooks
$b = "{`"trxID`":`"TXN-BK-$TS`",`"paymentID`":`"PAY-BK-$TS`",`"transactionStatus`":`"Completed`",`"amount`":`"1000`"}"
$r = Call-API "POST" "$BASE/webhooks/payment/bkash" "" $b
Log-Test "PAYMENT" "bKash Webhook" "System" "payments: webhook update" $r @(200,201)

$b = "{`"order_id`":`"$TS`",`"status`":`"VALID`",`"tran_id`":`"SSL-$TS`"}"
$r = Call-API "POST" "$BASE/webhooks/payment/sslcommerz" "" $b
Log-Test "PAYMENT" "SSLCommerz Webhook" "System" "payments: SSLCommerz callback" $r @(200,201)

$b = "{`"order_id`":`"$TS`",`"status`":`"Success`",`"transaction_id`":`"NAGAD-$TS`"}"
$r = Call-API "POST" "$BASE/webhooks/payment/nagad" "" $b
Log-Test "PAYMENT" "Nagad Webhook" "System" "payments: Nagad callback" $r @(200,201)

# Refund
if ($orderId1) {
    $b = "{`"order_id`":`"$orderId1`",`"amount`":650,`"reason`":`"Polo shirt wrong size received`"}"
    $r = Call-API "POST" "$BASE/payment/refunds" $CUST_TOKEN $b
    Log-Test "PAYMENT" "Request Refund (Customer)" "Customer" "refunds: status=PENDING request" $r @(200,201,400)
}

# Escrow release
$b = "{`"user_id`":`"$SELLER_ID`",`"amount`":5000,`"reason`":`"Order completed successfully`"}"
$r = Call-API "POST" "$BASE/payment/escrow/release" $CUST_TOKEN $b
Log-Test "PAYMENT" "Release Escrow (Customer)" "Customer" "escrow: release to seller" $r @(200,201,400)

# ================================================================
# [8] WALLET — wallets, wallet_transactions, withdrawal_requests
# ================================================================
Write-Host "`n[8] WALLET MODULE" -ForegroundColor Yellow

$r = Call-API "GET" "$BASE/wallet" $CUST_TOKEN
Log-Test "WALLET" "Get Balance (Customer)" "Customer" "wallets: balance, currency" $r @(200)

$r = Call-API "GET" "$BASE/wallet" $SELLER_TOKEN
Log-Test "WALLET" "Get Balance (Seller)" "Seller" "wallets: Seller balance" $r @(200)

$r = Call-API "GET" "$BASE/wallet" $RESELLER_TOKEN
Log-Test "WALLET" "Get Balance (Reseller)" "Reseller" "wallets: Reseller balance" $r @(200)

$r = Call-API "GET" "$BASE/wallet/transactions" $CUST_TOKEN
Log-Test "WALLET" "Ledger History (Customer)" "Customer" "wallet_transactions: CREDIT/DEBIT history" $r @(200)

$r = Call-API "GET" "$BASE/wallet/transactions" $SELLER_TOKEN
Log-Test "WALLET" "Ledger History (Seller)" "Seller" "wallet_transactions: Seller earnings" $r @(200)

$b = '{"amount":2500,"bank_account":"1234567890","bank_name":"Dutch-Bangla Bank","account_name":"Selim Seller"}'
$r = Call-API "POST" "$BASE/wallet/withdraw" $SELLER_TOKEN $b
Log-Test "WALLET" "Request Withdrawal (Seller)" "Seller" "withdrawal_requests: PENDING, wallet DEBIT" $r @(200,201,400)
$wdId1 = $r.data.data.request_id

$b = '{"amount":1000,"bank_account":"9876543210","bank_name":"Islami Bank","account_name":"Karim Reseller"}'
$r = Call-API "POST" "$BASE/wallet/withdraw" $RESELLER_TOKEN $b
Log-Test "WALLET" "Request Withdrawal (Reseller)" "Reseller" "withdrawal_requests: Reseller withdrawal" $r @(200,201,400)

$r = Call-API "GET" "$BASE/admin/wallet/requests" $ADMIN_TOKEN
Log-Test "WALLET" "List Withdrawal Requests (Admin)" "Admin" "withdrawal_requests: all pending" $r @(200)
$wdList = $r.data.data.requests
$firstWdId = if ($wdList -and $wdList.Count -gt 0) { $wdList[0].id } else { $wdId1 }

if ($firstWdId) {
    $b = '{"status":"APPROVED","admin_note":"Verified bank account"}'
    $r = Call-API "PUT" "$BASE/admin/wallet/requests/$firstWdId/process" $ADMIN_TOKEN $b
    Log-Test "WALLET" "Process Withdrawal APPROVED (Admin)" "Admin" "withdrawal_requests: APPROVED, wallet deduct" $r @(200)
}

# ================================================================
# [9] VENDOR — vendor_stores table
# ================================================================
Write-Host "`n[9] VENDOR MODULE" -ForegroundColor Yellow

$r = Call-API "GET" "$BASE/vendor/profile" $SELLER_TOKEN
Log-Test "VENDOR" "Get My Store (Seller)" "Seller" "vendor_stores: store profile" $r @(200)

$b = '{"store_name":"Selim Electronics BD Pro","description":"Best electronics at lowest price in Bangladesh"}'
$r = Call-API "PUT" "$BASE/vendor/profile" $SELLER_TOKEN $b
Log-Test "VENDOR" "Update Store (Seller)" "Seller" "vendor_stores: name, description update" $r @(200)

$r = Call-API "GET" "$BASE/vendor/analytics" $SELLER_TOKEN
Log-Test "VENDOR" "Vendor Analytics (Seller)" "Seller" "total_sales, orders, products stats" $r @(200)

$r = Call-API "GET" "$BASE/vendors/store/selim-electronics-bd" ""
Log-Test "VENDOR" "Get Public Store (Public)" "Public" "vendor_stores: public view" $r @(200,404)

$r = Call-API "GET" "$BASE/admin/vendors" $ADMIN_TOKEN
Log-Test "VENDOR" "List All Vendors (Admin)" "Admin" "vendor_stores: all vendors list" $r @(200)

$b = '{"verification_status":"VERIFIED","rejection_reason":""}'
$r = Call-API "PUT" "$BASE/admin/vendors/$SELLER_ID/verify" $ADMIN_TOKEN $b
Log-Test "VENDOR" "Verify Vendor (Admin)" "Admin" "vendor_stores: verification_status=VERIFIED" $r @(200)

# ================================================================
# [10] RESELLER — reseller_stores, reseller_catalog tables
# ================================================================
Write-Host "`n[10] RESELLER MODULE" -ForegroundColor Yellow

$b = '{"store_name":"Karim Online Shop","store_slug":"karim-online-shop","bio":"Best deals every day"}'
$r = Call-API "POST" "$BASE/reseller/setup" $RESELLER_TOKEN $b
Log-Test "RESELLER" "Create Reseller Store" "Reseller" "reseller_stores: new store" $r @(200,201,409)

$r = Call-API "GET" "$BASE/reseller/profile" $RESELLER_TOKEN
Log-Test "RESELLER" "Get My Store (Reseller)" "Reseller" "reseller_stores: profile" $r @(200)

$r = Call-API "GET" "$BASE/reseller/store/karim-online-shop" ""
Log-Test "RESELLER" "Get Public Store (Public)" "Public" "reseller_stores: public view" $r @(200,404)

if ($prod1Id) {
    $b = "{`"product_id`":`"$prod1Id`",`"margin_pct`":18}"
    $r = Call-API "POST" "$BASE/reseller/catalog/add" $RESELLER_TOKEN $b
    Log-Test "RESELLER" "Add Samsung to Catalog" "Reseller" "reseller_catalog: product+margin store" $r @(200,201)

    $b = "{`"product_id`":`"$prod1Id`",`"margin_pct`":18}"
    $r = Call-API "POST" "$BASE/reseller/margin/calculate" $RESELLER_TOKEN $b
    Log-Test "RESELLER" "Calculate Margin (Samsung)" "Reseller" "selling_price=wholesale*(1+margin%)" $r @(200)

    $b = "{`"product_id`":`"$prod1Id`"}"
    $r = Call-API "POST" "$BASE/reseller/share-link" $RESELLER_TOKEN $b
    Log-Test "RESELLER" "Generate Share Link" "Reseller" "reseller_links: unique referral link" $r @(200,201)
}

if ($prod3Id) {
    $b = "{`"product_id`":`"$prod3Id`",`"margin_pct`":25}"
    $r = Call-API "POST" "$BASE/reseller/catalog/add" $RESELLER_TOKEN $b
    Log-Test "RESELLER" "Add Polo to Catalog" "Reseller" "reseller_catalog: 2nd product" $r @(200,201)
}

$r = Call-API "GET" "$BASE/reseller/catalog" $RESELLER_TOKEN
Log-Test "RESELLER" "Get Catalog (Reseller)" "Reseller" "reseller_catalog: all reseller products" $r @(200)

if ($prod1Id) {
    $r = Call-API "DELETE" "$BASE/reseller/catalog/$prod3Id" $RESELLER_TOKEN
    Log-Test "RESELLER" "Remove From Catalog (Reseller)" "Reseller" "reseller_catalog: delete product" $r @(200,404)
}

# ================================================================
# [11] AFFILIATE — affiliate_profiles, affiliate_links tables
# ================================================================
Write-Host "`n[11] AFFILIATE MODULE" -ForegroundColor Yellow

$r = Call-API "GET" "$BASE/affiliate/profile" $RESELLER_TOKEN
Log-Test "AFFILIATE" "Get Affiliate Profile (Reseller)" "Reseller" "affiliate_profiles: registered=true/false" $r @(200)

$b = '{"referral_code":"KARIM2024BD"}'
$r = Call-API "POST" "$BASE/affiliate/apply" $RESELLER_TOKEN $b
Log-Test "AFFILIATE" "Apply for Affiliate (Reseller)" "Reseller" "affiliate_profiles: status=PENDING create" $r @(200,201,409)

$b = '{"bank_account":"9876543210","bank_name":"Islami Bank","payout_method":"BANK"}'
$r = Call-API "PUT" "$BASE/affiliate/profile" $RESELLER_TOKEN $b
Log-Test "AFFILIATE" "Update Affiliate Profile" "Reseller" "affiliate_profiles: bank info update" $r @(200)

if ($prod1Id) {
    $b = "{`"product_id`":`"$prod1Id`",`"campaign`":`"eid-sale-2024`"}"
    $r = Call-API "POST" "$BASE/affiliate/links" $RESELLER_TOKEN $b
    Log-Test "AFFILIATE" "Create Affiliate Link (Samsung)" "Reseller" "affiliate_links: unique link create" $r @(200,201,403)
    $affLinkId = $r.data.data.link_id
}

$r = Call-API "GET" "$BASE/affiliate/links" $RESELLER_TOKEN
Log-Test "AFFILIATE" "Get Affiliate Links" "Reseller" "affiliate_links: all links" $r @(200)

$r = Call-API "GET" "$BASE/affiliate/earnings" $RESELLER_TOKEN
Log-Test "AFFILIATE" "Get Affiliate Earnings" "Reseller" "affiliate_transactions: total_earned" $r @(200)

if ($affLinkId) {
    $r = Call-API "DELETE" "$BASE/affiliate/links/$affLinkId" $RESELLER_TOKEN
    Log-Test "AFFILIATE" "Delete Affiliate Link" "Reseller" "affiliate_links: delete" $r @(200,404)
}

# ================================================================
# [12] REVIEW — product_reviews table
# ================================================================
Write-Host "`n[12] REVIEW MODULE" -ForegroundColor Yellow

if ($prod1Id) {
    $r = Call-API "GET" "$BASE/reviews/product/$prod1Id" ""
    Log-Test "REVIEW" "Get Product Reviews (Public)" "Public" "product_reviews: avg_rating, reviews list" $r @(200)
}

$r = Call-API "GET" "$BASE/reviews/seller/$SELLER_ID" ""
Log-Test "REVIEW" "Get Seller Reviews (Public)" "Public" "seller_reviews: seller rating" $r @(200)

if ($prod1Id) {
    $b = "{`"product_id`":`"$prod1Id`",`"seller_id`":`"$SELLER_ID`",`"order_id`":`"$orderId1`",`"rating`":5,`"title`":`"Excellent Samsung phone!`",`"comment`":`"Very fast delivery, product is exactly as described. Battery life is amazing. Highly recommend!`"}"
    $r = Call-API "POST" "$BASE/reviews" $CUST_TOKEN $b
    Log-Test "REVIEW" "Create Review 5★ (Customer→Samsung)" "Customer" "product_reviews: rating=5, avg update" $r @(200,201)
    $reviewId1 = $r.data.data.review_id
}

if ($prod3Id) {
    $b = "{`"product_id`":`"$prod3Id`",`"seller_id`":`"$SELLER_ID`",`"rating`":4,`"title`":`"Good quality shirt`",`"comment`":`"Material is soft and size is accurate. Color is vibrant.`"}"
    $r = Call-API "POST" "$BASE/reviews" $CUST_TOKEN $b
    Log-Test "REVIEW" "Create Review 4★ (Customer→Polo)" "Customer" "product_reviews: 2nd review" $r @(200,201)
    $reviewId2 = $r.data.data.review_id
}

# Reseller reviews Samsung
if ($prod1Id) {
    $b = "{`"product_id`":`"$prod1Id`",`"seller_id`":`"$SELLER_ID`",`"rating`":5,`"title`":`"Best value phone`",`"comment`":`"I bought this for reselling. Customers love it.`"}"
    $r = Call-API "POST" "$BASE/reviews" $RESELLER_TOKEN $b
    Log-Test "REVIEW" "Create Review 5★ (Reseller→Samsung)" "Reseller" "product_reviews: 3rd review" $r @(200,201)
    $reviewId3 = $r.data.data.review_id
}

$r = Call-API "GET" "$BASE/reviews/my" $CUST_TOKEN
Log-Test "REVIEW" "Get My Reviews (Customer)" "Customer" "product_reviews: user's own reviews" $r @(200)

if ($reviewId1) {
    $r = Call-API "POST" "$BASE/reviews/$reviewId1/helpful" $RESELLER_TOKEN
    Log-Test "REVIEW" "Mark Review Helpful (Reseller)" "Reseller" "review_helpful: helpful_count+1" $r @(200,201)

    $b = '{"rating":5,"title":"Still excellent after 1 month!","comment":"Updated review - still working perfectly"}'
    $r = Call-API "PUT" "$BASE/reviews/$reviewId1" $CUST_TOKEN $b
    Log-Test "REVIEW" "Update Review (Customer)" "Customer" "product_reviews: rating, comment update" $r @(200)
}

# ================================================================
# [13] SEARCH — search_logs table
# ================================================================
Write-Host "`n[13] SEARCH MODULE" -ForegroundColor Yellow

$r = Call-API "GET" "$BASE/search?q=samsung+galaxy&limit=10" ""
Log-Test "SEARCH" "Search: samsung galaxy" "Public" "products: full-text search, search_logs insert" $r @(200)

$r = Call-API "GET" "$BASE/search?q=iphone+pro&limit=5" ""
Log-Test "SEARCH" "Search: iphone pro" "Public" "products: search + log" $r @(200)

$r = Call-API "GET" "$BASE/search?q=polo+shirt&category=fashion-cat" ""
Log-Test "SEARCH" "Search: polo shirt (category filter)" "Public" "products: category filtered search" $r @(200)

$r = Call-API "GET" "$BASE/search/suggestions?q=sam" ""
Log-Test "SEARCH" "Suggestions: sam" "Public" "search_logs: autocomplete" $r @(200)

$r = Call-API "GET" "$BASE/search/suggestions?q=ip" ""
Log-Test "SEARCH" "Suggestions: ip" "Public" "popular queries with 'ip'" $r @(200)

# ================================================================
# [14] SHIPPING — shipments table
# ================================================================
Write-Host "`n[14] SHIPPING MODULE" -ForegroundColor Yellow

$r = Call-API "GET" "$BASE/shipping/track/TRK-PATHAO-2024-001" ""
Log-Test "SHIPPING" "Track Shipment (Public)" "Public" "shipments: tracking_number lookup" $r @(200,404)

$b = '{"from_division":"Dhaka","to_division":"Chittagong","weight_kg":2.5,"courier":"PATHAO"}'
$r = Call-API "POST" "$BASE/shipping/calculate-rate" $SELLER_TOKEN $b
Log-Test "SHIPPING" "Calculate Rate (Seller)" "Seller" "Courier API: rate calculation" $r @(200)

if ($orderId1) {
    $b = "{`"order_id`":`"$orderId1`",`"courier`":`"PATHAO`",`"weight_kg`":1.2}"
    $r = Call-API "POST" "$BASE/shipping/create-consignment" $SELLER_TOKEN $b
    Log-Test "SHIPPING" "Create Consignment (Seller)" "Seller" "shipments: tracking_number assigned" $r @(200,201)
}

$r = Call-API "GET" "$BASE/shipping/consignments" $SELLER_TOKEN
Log-Test "SHIPPING" "List Consignments (Seller)" "Seller" "shipments: all seller consignments" $r @(200)

# Shipping webhooks
$b = "{`"tracking_number`":`"TRK-PATHAO-2024-001`",`"status`":`"DELIVERED`"}"
$r = Call-API "POST" "$BASE/webhooks/shipping/pathao" "" $b
Log-Test "SHIPPING" "Pathao Webhook" "System" "shipments: status update from courier" $r @(200,201)

$b = "{`"consignment_id`":`"SF-2024-001`",`"status`":`"In Transit`"}"
$r = Call-API "POST" "$BASE/webhooks/shipping/steadfast" "" $b
Log-Test "SHIPPING" "Steadfast Webhook" "System" "shipments: steadfast callback" $r @(200,201)

# ================================================================
# [15] NOTIFICATION — notifications table
# ================================================================
Write-Host "`n[15] NOTIFICATION MODULE" -ForegroundColor Yellow

$r = Call-API "GET" "$BASE/notifications" $CUST_TOKEN
Log-Test "NOTIFICATION" "Get Notifications (Customer)" "Customer" "notifications: all (read+unread)" $r @(200)
$notifList = $r.data.data.notifications
$notifId = if ($notifList -and $notifList.Count -gt 0) { $notifList[0].id } else { "" }

$r = Call-API "GET" "$BASE/notifications/unread-count" $CUST_TOKEN
Log-Test "NOTIFICATION" "Unread Count (Customer)" "Customer" "notifications: is_read=false COUNT" $r @(200)

$r = Call-API "GET" "$BASE/notifications" $SELLER_TOKEN
Log-Test "NOTIFICATION" "Get Notifications (Seller)" "Seller" "notifications: Seller notifications" $r @(200)

if ($notifId) {
    $r = Call-API "PUT" "$BASE/notifications/$notifId/read" $CUST_TOKEN
    Log-Test "NOTIFICATION" "Mark Notification Read" "Customer" "notifications: is_read=true, read_at=now()" $r @(200)
}

$r = Call-API "PUT" "$BASE/notifications/read-all" $CUST_TOKEN
Log-Test "NOTIFICATION" "Mark All Read (Customer)" "Customer" "notifications: all is_read=true" $r @(200)

# ================================================================
# [16] PROMOTION — coupons, promotions tables
# ================================================================
Write-Host "`n[16] PROMOTION MODULE" -ForegroundColor Yellow

$r = Call-API "GET" "$BASE/promotions/flash-sales" ""
Log-Test "PROMOTION" "Get Flash Sales (Public)" "Public" "promotions: active flash sales" $r @(200)

$couponCode = "EID2024SALE"
$b = "{`"code`":`"$couponCode`",`"discount_type`":`"PERCENTAGE`",`"discount_value`":15,`"min_order_amount`":2000,`"max_uses`":500,`"expires_at`":`"2026-12-31T23:59:59Z`"}"
$r = Call-API "POST" "$BASE/admin/coupons" $ADMIN_TOKEN $b
Log-Test "PROMOTION" "Create Coupon EID2024SALE (Admin)" "Admin" "coupons: new coupon insert" $r @(200,201,409)

$couponCode2 = "FIRSTORDER100"
$b = "{`"code`":`"$couponCode2`",`"discount_type`":`"FLAT`",`"discount_value`":100,`"min_order_amount`":500,`"max_uses`":1000,`"expires_at`":`"2026-12-31T23:59:59Z`"}"
$r = Call-API "POST" "$BASE/admin/coupons" $ADMIN_TOKEN $b
Log-Test "PROMOTION" "Create Coupon FIRSTORDER100 (Admin)" "Admin" "coupons: flat discount coupon" $r @(200,201,409)

$r = Call-API "GET" "$BASE/admin/coupons" $ADMIN_TOKEN
Log-Test "PROMOTION" "List Coupons (Admin)" "Admin" "coupons: all coupons list" $r @(200)

$b = "{`"code`":`"$couponCode`",`"order_amount`":5000}"
$r = Call-API "POST" "$BASE/promotions/validate-coupon" $CUST_TOKEN $b
Log-Test "PROMOTION" "Validate Coupon (Customer)" "Customer" "coupons: valid=true, discount=750 BDT" $r @(200)

$b = "{`"code`":`"$couponCode2`",`"order_amount`":1200}"
$r = Call-API "POST" "$BASE/promotions/validate-coupon" $RESELLER_TOKEN $b
Log-Test "PROMOTION" "Validate Coupon (Reseller)" "Reseller" "coupons: flat 100 BDT discount" $r @(200)

# ================================================================
# [17] DISPUTE — disputes, dispute_messages tables
# ================================================================
Write-Host "`n[17] DISPUTE MODULE" -ForegroundColor Yellow

if ($orderId1) {
    $b = "{`"order_id`":`"$orderId1`",`"type`":`"ITEM_NOT_RECEIVED`",`"description`":`"I placed an order 7 days ago but haven't received it yet. Please help.`"}"
    $r = Call-API "POST" "$BASE/disputes" $CUST_TOKEN $b
    Log-Test "DISPUTE" "Create Dispute: Not Received (Customer)" "Customer" "disputes: status=OPEN, order flagged" $r @(200,201)
    $disputeId1 = $r.data.data.dispute_id

    if ($disputeId1) {
        $r = Call-API "GET" "$BASE/disputes/$disputeId1" $CUST_TOKEN
        Log-Test "DISPUTE" "Get Dispute Detail (Customer)" "Customer" "disputes: full dispute detail" $r @(200)

        $b = '{"message":"I have attached photos. Please check urgently."}'
        $r = Call-API "POST" "$BASE/disputes/$disputeId1/messages" $CUST_TOKEN $b
        Log-Test "DISPUTE" "Add Message (Customer)" "Customer" "dispute_messages: message store" $r @(200,201)

        $b = '{"message":"We are investigating. Please wait 24 hours."}'
        $r = Call-API "POST" "$BASE/disputes/$disputeId1/messages" $SELLER_TOKEN $b
        Log-Test "DISPUTE" "Add Message (Seller)" "Seller" "dispute_messages: seller reply" $r @(200,201)

        $b = '{"resolution":"REFUND_ISSUED","admin_note":"Investigated and confirmed item not delivered. Refund processed."}'
        $r = Call-API "PUT" "$BASE/admin/disputes/$disputeId1/resolve" $ADMIN_TOKEN $b
        Log-Test "DISPUTE" "Resolve Dispute (Admin)" "Admin" "disputes: status=RESOLVED, refund noted" $r @(200)
    }
}

$r = Call-API "GET" "$BASE/disputes/my" $CUST_TOKEN
Log-Test "DISPUTE" "Get My Disputes (Customer)" "Customer" "disputes: customer's disputes" $r @(200)

$r = Call-API "GET" "$BASE/admin/disputes" $ADMIN_TOKEN
Log-Test "DISPUTE" "List All Disputes (Admin)" "Admin" "disputes: platform-wide all disputes" $r @(200)

# ================================================================
# [18] MEDIA — media table
# ================================================================
Write-Host "`n[18] MEDIA MODULE" -ForegroundColor Yellow

$r = Call-API "GET" "$BASE/media/test-media-001" ""
Log-Test "MEDIA" "Get Media Detail (Public)" "Public" "media: url, type, size" $r @(200,404)

$b = '{"file_name":"samsung_a55_front.jpg","file_type":"image/jpeg","folder":"products"}'
$r = Call-API "POST" "$BASE/media/upload" $SELLER_TOKEN $b
Log-Test "MEDIA" "Upload Product Image (Seller)" "Seller" "media: cdn_url, file_name store" $r @(200,201)

$b = '{"file_name":"polo_shirt_red.jpg","file_type":"image/jpeg","folder":"products"}'
$r = Call-API "POST" "$BASE/media/upload" $SELLER_TOKEN $b
Log-Test "MEDIA" "Upload Fashion Image (Seller)" "Seller" "media: 2nd product image" $r @(200,201)

$b = '{"file_key":"uploads/profile_photo.jpg","expires_in":3600}'
$r = Call-API "POST" "$BASE/media/presigned-url" $CUST_TOKEN $b
Log-Test "MEDIA" "Get Presigned URL (Customer)" "Customer" "R2/S3 presigned URL for direct upload" $r @(200,201)

# ================================================================
# [19] ANALYTICS — analytics_reports, search_logs tables
# ================================================================
Write-Host "`n[19] ANALYTICS MODULE" -ForegroundColor Yellow

$r = Call-API "GET" "$BASE/analytics/overview" $ADMIN_TOKEN
Log-Test "ANALYTICS" "Platform Overview (Admin)" "Admin" "GMV, vendors, orders, users overview" $r @(200)

$r = Call-API "GET" "$BASE/analytics/sales" $ADMIN_TOKEN
Log-Test "ANALYTICS" "Sales Analytics (Admin)" "Admin" "daily_sales, revenue by date range" $r @(200)

$r = Call-API "GET" "$BASE/analytics/customers" $ADMIN_TOKEN
Log-Test "ANALYTICS" "Customer Analytics (Admin)" "Admin" "total, repeat, new customers" $r @(200)

$r = Call-API "GET" "$BASE/analytics/products" $ADMIN_TOKEN
Log-Test "ANALYTICS" "Product Analytics (Admin)" "Admin" "top products by revenue" $r @(200)

$r = Call-API "GET" "$BASE/analytics/traffic" $ADMIN_TOKEN
Log-Test "ANALYTICS" "Traffic Analytics (Admin)" "Admin" "searches, page views, top queries" $r @(200)

$b = '{"report_type":"MONTHLY_SALES","date_from":"2026-01-01","date_to":"2026-12-31"}'
$r = Call-API "POST" "$BASE/analytics/reports" $ADMIN_TOKEN $b
Log-Test "ANALYTICS" "Create Custom Report (Admin)" "Admin" "analytics_reports: QUEUED job create" $r @(200,201)

# ================================================================
# [20] ADMIN FINANCE — escrow, commissions, payouts tables
# ================================================================
Write-Host "`n[20] ADMIN FINANCE MODULE" -ForegroundColor Yellow

$r = Call-API "GET" "$BASE/admin/finance/overview" $ADMIN_TOKEN
Log-Test "ADMIN_FIN" "Finance Overview (Admin)" "Admin" "platform revenue, escrow, payouts" $r @(200)

$r = Call-API "GET" "$BASE/admin/finance/payouts" $ADMIN_TOKEN
Log-Test "ADMIN_FIN" "All Payout Requests (Admin)" "Admin" "withdrawal_requests: all list" $r @(200)

$r = Call-API "GET" "$BASE/admin/finance/payouts?status=PENDING" $ADMIN_TOKEN
Log-Test "ADMIN_FIN" "Pending Payouts Filter (Admin)" "Admin" "withdrawal_requests: status=PENDING" $r @(200)

$r = Call-API "GET" "$BASE/admin/finance/escrow" $ADMIN_TOKEN
Log-Test "ADMIN_FIN" "Escrow Holdings (Admin)" "Admin" "escrow_holdings: held amounts" $r @(200)

$r = Call-API "GET" "$BASE/admin/finance/commissions" $ADMIN_TOKEN
Log-Test "ADMIN_FIN" "Commission Report (Admin)" "Admin" "order_commissions: platform earnings" $r @(200)

$r = Call-API "GET" "$BASE/admin/finance/reports" $ADMIN_TOKEN
Log-Test "ADMIN_FIN" "Finance Reports (Admin)" "Admin" "analytics_reports: finance reports" $r @(200)

$b = '{"category_id":"","commission_rate":9.0}'
$r = Call-API "PUT" "$BASE/admin/finance/commission-rates" $ADMIN_TOKEN $b
Log-Test "ADMIN_FIN" "Update Commission Rate 9% (Admin)" "Admin" "product_categories: commission_rate update" $r @(200)

$b = "{`"user_id`":`"$SELLER_ID`",`"amount`":3000,`"reason`":`"Order #001 completed - release escrow`"}"
$r = Call-API "POST" "$BASE/admin/finance/escrow/release" $ADMIN_TOKEN $b
Log-Test "ADMIN_FIN" "Release Escrow→Seller (Admin)" "Admin" "escrow→wallet CREDIT to Seller" $r @(200,201)

$b = "{`"user_id`":`"$RESELLER_ID`",`"amount`":500,`"type`":`"CREDIT`",`"description`":`"Welcome bonus for new reseller`"}"
$r = Call-API "POST" "$BASE/admin/finance/manual-credit" $ADMIN_TOKEN $b
Log-Test "ADMIN_FIN" "Manual Credit→Reseller (Admin)" "Admin" "wallet_transactions: CREDIT entry" $r @(200,201)

$b = "{`"user_id`":`"$SELLER_ID`",`"amount`":1000,`"type`":`"CREDIT`",`"description`":`"Performance bonus`"}"
$r = Call-API "POST" "$BASE/admin/finance/manual-credit" $ADMIN_TOKEN $b
Log-Test "ADMIN_FIN" "Manual Credit→Seller (Admin)" "Admin" "wallet_transactions: Seller bonus" $r @(200,201)

# ================================================================
# [21] ADMIN FRAUD — fraud_blacklists, ip_blocks, user_risk_profiles
# ================================================================
Write-Host "`n[21] ADMIN FRAUD MODULE" -ForegroundColor Yellow

$r = Call-API "GET" "$BASE/admin/fraud/risk-profiles" $ADMIN_TOKEN
Log-Test "ADMIN_FRAUD" "Risk Profiles (Admin)" "Admin" "user_risk_profiles: high-risk users" $r @(200)

$r = Call-API "GET" "$BASE/admin/fraud/blacklists" $ADMIN_TOKEN
Log-Test "ADMIN_FRAUD" "Blacklist List (Admin)" "Admin" "fraud_blacklists: phones/emails" $r @(200)

$r = Call-API "GET" "$BASE/admin/fraud/ip-blocks" $ADMIN_TOKEN
Log-Test "ADMIN_FRAUD" "IP Block List (Admin)" "Admin" "ip_blocks: blocked IPs" $r @(200)

$b = '{"phone":"01999111222","reason":"FAKE_ORDERS"}'
$r = Call-API "POST" "$BASE/admin/fraud/blacklist-phone" $ADMIN_TOKEN $b
Log-Test "ADMIN_FRAUD" "Blacklist Phone (Admin)" "Admin" "fraud_blacklists: phone blacklisted" $r @(200,201)

$b = '{"phone":"01888333444","reason":"CHARGEBACK_ABUSE"}'
$r = Call-API "POST" "$BASE/admin/fraud/blacklist-phone" $ADMIN_TOKEN $b
Log-Test "ADMIN_FRAUD" "Blacklist Phone 2 (Admin)" "Admin" "fraud_blacklists: 2nd phone" $r @(200,201)

$b = '{"ip_address":"10.0.0.55","reason":"BRUTE_FORCE","duration_hours":24}'
$r = Call-API "POST" "$BASE/admin/fraud/block-ip" $ADMIN_TOKEN $b
Log-Test "ADMIN_FRAUD" "Block IP (Admin)" "Admin" "ip_blocks: IP block with duration" $r @(200,201)

$b = '{"ip_address":"192.168.10.100","reason":"SCRAPING","duration_hours":72}'
$r = Call-API "POST" "$BASE/admin/fraud/block-ip" $ADMIN_TOKEN $b
Log-Test "ADMIN_FRAUD" "Block IP 2 (Admin)" "Admin" "ip_blocks: 2nd IP block" $r @(200,201)

$b = "{`"user_id`":`"$CUST_ID`"}"
$r = Call-API "POST" "$BASE/admin/fraud/recalculate-risk" $ADMIN_TOKEN $b
Log-Test "ADMIN_FRAUD" "Recalculate Risk (Customer)" "Admin" "user_risk_profiles: risk_score update" $r @(200)

$b = "{`"user_id`":`"$RESELLER_ID`"}"
$r = Call-API "POST" "$BASE/admin/fraud/recalculate-risk" $ADMIN_TOKEN $b
Log-Test "ADMIN_FRAUD" "Recalculate Risk (Reseller)" "Admin" "user_risk_profiles: Reseller score" $r @(200)

# ================================================================
# [22] ADMIN SETTINGS — platform_settings table
# ================================================================
Write-Host "`n[22] ADMIN SETTINGS MODULE" -ForegroundColor Yellow

$r = Call-API "GET" "$BASE/admin/settings" $ADMIN_TOKEN
Log-Test "ADMIN_SET" "Get All Settings (Admin)" "Admin" "platform_settings: all key-values" $r @(200)

$r = Call-API "GET" "$BASE/admin/settings/audit-logs" $ADMIN_TOKEN
Log-Test "ADMIN_SET" "Get Audit Logs (Admin)" "Admin" "users: recent activity audit" $r @(200)

$r = Call-API "GET" "$BASE/admin/settings/default_commission_rate" $ADMIN_TOKEN
Log-Test "ADMIN_SET" "Get Setting: commission_rate" "Admin" "platform_settings OR default=5.0" $r @(200)

$r = Call-API "GET" "$BASE/admin/settings/nonexistent_xyz" $ADMIN_TOKEN
Log-Test "ADMIN_SET" "Get Setting: nonexistent (404)" "Admin" "platform_settings: key not found" $r @(404)

$b = '{"settings":{"default_commission_rate":"8.0","min_order_amount":"200","max_products_per_vendor":"1000","free_shipping_threshold":"1000","currency":"BDT"}}'
$r = Call-API "PUT" "$BASE/admin/settings" $ADMIN_TOKEN $b
Log-Test "ADMIN_SET" "Batch Update 5 Settings" "Admin" "platform_settings: UPSERT 5 keys" $r @(200)

$b = '{"value":"14","description":"Days allowed for return after delivery"}'
$r = Call-API "PUT" "$BASE/admin/settings/return_window_days" $ADMIN_TOKEN $b
Log-Test "ADMIN_SET" "Update: return_window_days=14" "Admin" "platform_settings: single key update" $r @(200)

$b = '{"value":"30","description":"Customer support response time in minutes"}'
$r = Call-API "PUT" "$BASE/admin/settings/support_response_time" $ADMIN_TOKEN $b
Log-Test "ADMIN_SET" "Update: support_response_time" "Admin" "platform_settings: new key upsert" $r @(200)

$b = '{"enabled":false,"message":""}'
$r = Call-API "PUT" "$BASE/admin/settings/maintenance-mode" $ADMIN_TOKEN $b
Log-Test "ADMIN_SET" "Maintenance Mode OFF" "Admin" "platform_settings: maintenance_mode=false" $r @(200)

# ================================================================
# [23] CHINA SOURCING — sourcing_batches, sourcing_batch_items
# ================================================================
Write-Host "`n[23] CHINA SOURCING MODULE" -ForegroundColor Yellow

$r = Call-API "GET" "$BASE/china-sourcing/batches" $SELLER_TOKEN
Log-Test "SOURCING" "List Batches (Seller)" "Seller" "sourcing_batches: seller's import batches" $r @(200)

$r = Call-API "GET" "$BASE/china-sourcing/landed-cost-calculator?cny_cost=50&quantity=100&freight_type=SEA&customs_duty_pct=25" ""
Log-Test "SOURCING" "Landed Cost SEA (Public)" "Public" "Calculate: BDT cost with SEA freight" $r @(200)

$r = Call-API "GET" "$BASE/china-sourcing/landed-cost-calculator?cny_cost=80&quantity=20&freight_type=AIR&customs_duty_pct=25" ""
Log-Test "SOURCING" "Landed Cost AIR (Public)" "Public" "Calculate: BDT cost with AIR freight" $r @(200)

$r = Call-API "GET" "$BASE/china-sourcing/customs-declarations" $SELLER_TOKEN
Log-Test "SOURCING" "Customs Declarations (Seller)" "Seller" "customs_declarations: clearance records" $r @(200)

# Create Batch 1 - SEA
$b = '{"product_name":"Samsung Accessories Bundle","supplier":"Shenzhen Electronics Co Ltd","freight_type":"SEA","quantity":500}'
$r = Call-API "POST" "$BASE/china-sourcing/batches" $SELLER_TOKEN $b
Log-Test "SOURCING" "Create Batch: SEA Accessories" "Seller" "sourcing_batches: batch_code=CN-YEAR-SEA-XXXXX" $r @(200,201)
$batchId1 = $r.data.data.batch_id

# Create Batch 2 - AIR
$b = '{"product_name":"Phone Cases Premium","supplier":"Guangzhou Case Factory","freight_type":"AIR","quantity":200}'
$r = Call-API "POST" "$BASE/china-sourcing/batches" $SELLER_TOKEN $b
Log-Test "SOURCING" "Create Batch: AIR Phone Cases" "Seller" "sourcing_batches: 2nd batch AIR freight" $r @(200,201)
$batchId2 = $r.data.data.batch_id

if ($batchId1) {
    $r = Call-API "GET" "$BASE/china-sourcing/batches/$batchId1" $SELLER_TOKEN
    Log-Test "SOURCING" "Get Batch Detail (Seller)" "Seller" "sourcing_batches+items: complete detail" $r @(200)

    $b = '{"product_name":"Samsung Galaxy A55 Case","quantity":200,"unit_cost_cny":25}'
    $r = Call-API "POST" "$BASE/china-sourcing/batches/$batchId1/items" $SELLER_TOKEN $b
    Log-Test "SOURCING" "Add Item: Galaxy A55 Case" "Seller" "sourcing_batch_items: subtotal_cny=5000" $r @(200,201)

    $b = '{"product_name":"USB-C Fast Charger 65W","quantity":300,"unit_cost_cny":15}'
    $r = Call-API "POST" "$BASE/china-sourcing/batches/$batchId1/items" $SELLER_TOKEN $b
    Log-Test "SOURCING" "Add Item: 65W Charger" "Seller" "sourcing_batch_items: subtotal_cny=4500" $r @(200,201)

    $b = '{"status":"ORDERED"}'
    $r = Call-API "PUT" "$BASE/china-sourcing/batches/$batchId1/status" $SELLER_TOKEN $b
    Log-Test "SOURCING" "Update Status→ORDERED" "Seller" "sourcing_batches: SOURCING→ORDERED" $r @(200)

    $b = '{"cny_cost":40,"freight_cost":85,"customs_duty_pct":25,"landed_cost_bdt":1050}'
    $r = Call-API "PUT" "$BASE/china-sourcing/batches/$batchId1/landed-cost" $SELLER_TOKEN $b
    Log-Test "SOURCING" "Update Landed Cost" "Seller" "sourcing_batches: actual cost update" $r @(200)

    $b = '{"hs_code":"3926.90","declaration_no":"CUSTOMS-2026-BD-001","duty_paid_amount":12500}'
    $r = Call-API "PUT" "$BASE/china-sourcing/batches/$batchId1/customs" $SELLER_TOKEN $b
    Log-Test "SOURCING" "Update Customs Info" "Seller" "sourcing_batches: hs_code, declaration" $r @(200)
}

if ($batchId2) {
    $b = '{"product_name":"iPhone 15 Leather Case","quantity":100,"unit_cost_cny":45}'
    $r = Call-API "POST" "$BASE/china-sourcing/batches/$batchId2/items" $SELLER_TOKEN $b
    Log-Test "SOURCING" "Add Item: iPhone Case" "Seller" "sourcing_batch_items: 2nd batch item" $r @(200,201)

    $b = '{"status":"SHIPPED"}'
    $r = Call-API "PUT" "$BASE/china-sourcing/batches/$batchId2/status" $SELLER_TOKEN $b
    Log-Test "SOURCING" "Update Status→SHIPPED" "Seller" "sourcing_batches: ORDERED→SHIPPED" $r @(200)
}

# ================================================================
# [24] SECURITY & VALIDATION TESTS
# ================================================================
Write-Host "`n[24] SECURITY CHECKS" -ForegroundColor Yellow

$r = Call-API "GET" "$BASE/user/profile"
Log-Test "SECURITY" "No Auth → 401 (Profile)" "No Auth" "JWT middleware rejects → 401" $r @(401)

$r = Call-API "GET" "$BASE/wallet"
Log-Test "SECURITY" "No Auth → 401 (Wallet)" "No Auth" "Wallet protected route → 401" $r @(401)

$r = Call-API "GET" "$BASE/admin/vendors"
Log-Test "SECURITY" "No Auth → 401 (Admin)" "No Auth" "Admin route protected → 401" $r @(401)

$b = '{"email":"not-valid-email","password":"123"}'
$r = Call-API "POST" "$BASE/auth/email/register" "" $b
Log-Test "SECURITY" "Invalid Email → 400/422" "No Auth" "Validation: email format check" $r @(400,422)

$b = '{"amount":-500,"bank_account":"123"}'
$r = Call-API "POST" "$BASE/wallet/withdraw" $CUST_TOKEN $b
Log-Test "SECURITY" "Negative Amount → 422" "Customer" "Validation: amount > 0 required" $r @(400,422)

$b = '{"ip_address":"not-valid-ip","reason":"test","duration_hours":24}'
$r = Call-API "POST" "$BASE/admin/fraud/block-ip" $ADMIN_TOKEN $b
Log-Test "SECURITY" "Invalid IP Format → 422" "Admin" "Validation: IP format check" $r @(400,422)

# ================================================================
# FINAL RESULTS
# ================================================================
Write-Host "`n================================================================" -ForegroundColor Cyan
Write-Host " FINAL TEST RESULTS" -ForegroundColor Cyan
Write-Host "================================================================" -ForegroundColor Cyan
Write-Host (" TOTAL  : {0}" -f $global:total)
Write-Host (" PASSED : {0}" -f $global:pass) -ForegroundColor Green
$failColor = if($global:fail -eq 0){"Green"}else{"Red"}
Write-Host (" FAILED : {0}" -f $global:fail) -ForegroundColor $failColor
$rate = if($global:total -gt 0){[math]::Round(($global:pass/$global:total)*100,1)}else{0}
$rateColor = if($rate -ge 95){"Green"}elseif($rate -ge 80){"Yellow"}else{"Red"}
Write-Host (" RATE   : {0}%" -f $rate) -ForegroundColor $rateColor
Write-Host "================================================================" -ForegroundColor Cyan

Write-Host "`n--- Section Summary ---" -ForegroundColor Yellow
$global:Results | Group-Object Section | ForEach-Object {
    $p = ($_.Group | Where-Object {$_.Status -eq "PASS"}).Count
    $t = $_.Count
    $c = if($p -eq $t){"Green"}elseif(($t-$p) -le 2){"Yellow"}else{"Red"}
    Write-Host ("  {0,-15} {1}/{2}" -f $_.Name, $p, $t) -ForegroundColor $c
}

Write-Host "`n All DB tables should now have data. Check NeonDB!" -ForegroundColor Cyan
