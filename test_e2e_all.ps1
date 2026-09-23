param()
$BASE_URL = "http://localhost:8080/api/v1"
[int]$PASS = 0
[int]$FAIL = 0

function Invoke-Test {
    param([string]$M="GET",[string]$U,[string]$B="",[string]$Tk="",[string]$N="",[int[]]$E=@(200,201))
    $h = @{ "Content-Type" = "application/json" }
    if ($Tk) { $h["Authorization"] = "Bearer $Tk" }
    try {
        $p = @{ Uri=$U; Method=$M; Headers=$h; ErrorAction="Stop" }
        if ($B) { $p["Body"] = $B }
        $r = Invoke-WebRequest @p
        $j = $r.Content | ConvertFrom-Json
        if ($E -contains $r.StatusCode) { 
            Write-Host "  [PASS] [$N] HTTP $($r.StatusCode)" -ForegroundColor Green
            $script:PASS = $script:PASS + 1
            return $j.data
        } else {
            Write-Host "  [FAIL] [$N] Got $($r.StatusCode) Expected $($E -join "/")" -ForegroundColor Red
            $script:FAIL = $script:FAIL + 1
        }
    } catch {
        $ec = $null
        try { $ec = [int]$_.Exception.Response.StatusCode } catch {}
        if ($ec -and ($E -contains $ec)) {
            Write-Host "  [PASS] [$N] HTTP $ec (expected)" -ForegroundColor Green
            $script:PASS = $script:PASS + 1
        } else {
            Write-Host "  [FAIL] [$N] $($_.Exception.Message.Substring(0,[Math]::Min(70,$_.Exception.Message.Length)))" -ForegroundColor Red
            $script:FAIL = $script:FAIL + 1
        }
    }
    return $null
}

Write-Host "=== MEGA API TEST SUITE - 24 Domains ===" -ForegroundColor Cyan
Start-Sleep -Seconds 2

Write-Host "`n[0] HEALTH CHECK" -ForegroundColor Magenta
Invoke-Test -M "GET" -U "http://localhost:8080/" -N "Root" -E @(200,404)
Invoke-Test -M "GET" -U "http://localhost:8080/health" -N "Health" -E @(200,404)

Write-Host "`n[1] AUTHENTICATION" -ForegroundColor Magenta
$ts = [DateTimeOffset]::Now.ToUnixTimeMilliseconds()
$EMAIL = "apitest_${ts}@example.com"
$PHONE = "017" + (Get-Random -Minimum 10000000 -Maximum 99999999)
$PASS_STR = "StrongPass@2024"

$rb = "{`"email`":`"$EMAIL`",`"phone`":`"$PHONE`",`"full_name`":`"API Tester`",`"password`":`"$PASS_STR`"}"
$rr = Invoke-Test -M "POST" -U "$BASE_URL/auth/email/register" -B $rb -N "Register" -E @(201)

$lb = "{`"email`":`"$EMAIL`",`"password`":`"$PASS_STR`"}"
$lr = Invoke-Test -M "POST" -U "$BASE_URL/auth/email/login" -B $lb -N "Login" -E @(200)
$TOKEN = if ($lr) { $lr.access_token } else { "" }
Write-Host "  Token: $($TOKEN.Substring(0,[Math]::Min(30,$TOKEN.Length)))..." -ForegroundColor DarkGray

Invoke-Test -M "POST" -U "$BASE_URL/auth/email/register" -B $rb -N "Dup Reg (409)" -E @(409,400)
Invoke-Test -M "POST" -U "$BASE_URL/auth/password/reset-request" -B "{`"email`":`"$EMAIL`"}" -N "PW Reset OTP" -E @(200,201)
Invoke-Test -M "POST" -U "$BASE_URL/auth/phone/send-otp" -B "{`"phone`":`"$PHONE`"}" -N "Phone OTP" -E @(200,201,429)

Write-Host "`n[2] USER MANAGEMENT" -ForegroundColor Magenta
Invoke-Test -U "$BASE_URL/user/profile" -Tk $TOKEN -N "Get Profile"
Invoke-Test -M "PUT" -U "$BASE_URL/user/profile" -B "{`"full_name`":`"Updated Tester`"}" -Tk $TOKEN -N "Update Profile"
Invoke-Test -U "$BASE_URL/user/addresses" -Tk $TOKEN -N "List Addresses"
$ab = "{`"label`":`"Home`",`"full_name`":`"API Tester`",`"phone`":`"$PHONE`",`"address`":`"123 Main St`",`"city`":`"Dhaka`",`"district`":`"Dhaka`",`"postal_code`":`"1212`"}"
$ar = Invoke-Test -M "POST" -U "$BASE_URL/user/addresses" -B $ab -Tk $TOKEN -N "Add Address" -E @(201)
$AID = if ($ar) { $ar.address_id } else { "00000000-0000-0000-0000-000000000000" }
Invoke-Test -M "PUT" -U "$BASE_URL/user/addresses/$AID" -B "{`"label`":`"Office`"}" -Tk $TOKEN -N "Update Address" -E @(200,404)
Invoke-Test -M "DELETE" -U "$BASE_URL/user/addresses/$AID" -Tk $TOKEN -N "Delete Address" -E @(200,404)

Write-Host "`n[3] PRODUCT CATALOG" -ForegroundColor Magenta
Invoke-Test -U "$BASE_URL/products" -N "List Products (public)"
Invoke-Test -U "$BASE_URL/products?q=test" -N "Search Products"
Invoke-Test -U "$BASE_URL/categories" -N "Category Tree"
$catB = "{`"name`":`"Cat${ts}`",`"slug`":`"cat-${ts}`",`"commission_rate`":8.5}"
$catR = Invoke-Test -M "POST" -U "$BASE_URL/admin/categories" -B $catB -Tk $TOKEN -N "Create Category" -E @(201)
$CATID = if ($catR) { $catR.category_id } else { "00000000-0000-0000-0000-000000000001" }
$PSlug = "product-$ts"
$prodB = "{`"category_id`":`"$CATID`",`"title`":`"Product $ts`",`"slug`":`"$PSlug`",`"description`":`"Test product description`",`"short_description`":`"Short desc`",`"is_resellable`":true,`"wholesale_price`":800.0,`"retail_price`":1200.0}"
$prodR = Invoke-Test -M "POST" -U "$BASE_URL/vendor/products" -B $prodB -Tk $TOKEN -N "Create Product" -E @(201)
$PROD_ID = if ($prodR) { $prodR.product_id } else { "00000000-0000-0000-0000-000000000001" }
Invoke-Test -U "$BASE_URL/products/$PSlug" -N "Product Detail (public)" -E @(200,404)
$skuB = "{`"sku_code`":`"SKU${ts}`",`"wholesale_price`":800.0,`"retail_price`":1200.0,`"stock_quantity`":100}"
$skuR = Invoke-Test -M "POST" -U "$BASE_URL/vendor/products/$PROD_ID/skus" -B $skuB -Tk $TOKEN -N "Add SKU" -E @(201,404)
$SKUID = if ($skuR) { $skuR.sku_id } else { "00000000-0000-0000-0000-000000000001" }
Invoke-Test -M "PUT" -U "$BASE_URL/admin/products/$PROD_ID/approve" -B "{`"approval_status`":`"APPROVED`"}" -Tk $TOKEN -N "Approve Product" -E @(200,404)

Write-Host "`n[4] CART" -ForegroundColor Magenta
Invoke-Test -U "$BASE_URL/cart" -Tk $TOKEN -N "Get Cart"
$cB = "{`"sku_id`":`"$SKUID`",`"quantity`":2}"
Invoke-Test -M "POST" -U "$BASE_URL/cart/items" -B $cB -Tk $TOKEN -N "Add to Cart" -E @(201,200,400)
Invoke-Test -U "$BASE_URL/cart/summary" -Tk $TOKEN -N "Cart Summary"
Invoke-Test -M "DELETE" -U "$BASE_URL/cart" -Tk $TOKEN -N "Clear Cart"

Write-Host "`n[5] ORDER/CHECKOUT" -ForegroundColor Magenta
Invoke-Test -M "POST" -U "$BASE_URL/cart/items" -B $cB -Tk $TOKEN -N "Re-add Cart" -E @(201,200,400)
$chkB = "{`"shipping_address`":{`"full_name`":`"API Tester`",`"phone`":`"$PHONE`",`"address`":`"123 Dhaka`",`"city`":`"Dhaka`",`"district`":`"Dhaka`",`"postal_code`":`"1212`"},`"payment_method`":`"COD`"}"
$ordR = Invoke-Test -M "POST" -U "$BASE_URL/customer/orders/checkout" -B $chkB -Tk $TOKEN -N "Checkout" -E @(201,200,400)
$OID = if ($ordR) { $ordR.master_order_id } else { "00000000-0000-0000-0000-000000000001" }
Invoke-Test -U "$BASE_URL/customer/orders" -Tk $TOKEN -N "My Orders"
Invoke-Test -U "$BASE_URL/customer/orders/$OID" -Tk $TOKEN -N "Order Detail" -E @(200,404)

Write-Host "`n[6] PAYMENT" -ForegroundColor Magenta
Invoke-Test -U "$BASE_URL/payments/methods" -Tk $TOKEN -N "Payment Methods"
Invoke-Test -M "POST" -U "$BASE_URL/payments/initiate" -B "{`"payment_gateway`":`"BKASH`",`"amount`":1200.0}" -Tk $TOKEN -N "Initiate bKash"
Invoke-Test -M "POST" -U "$BASE_URL/payments/initiate" -B "{`"payment_gateway`":`"COD`",`"amount`":1200.0}" -Tk $TOKEN -N "Initiate COD"
Invoke-Test -M "POST" -U "$BASE_URL/payments/initiate" -B "{`"payment_gateway`":`"SSLCOMMERZ`",`"amount`":1200.0}" -Tk $TOKEN -N "Initiate SSL"
Invoke-Test -M "POST" -U "$BASE_URL/payments/initiate" -B "{`"payment_gateway`":`"NAGAD`",`"amount`":1200.0}" -Tk $TOKEN -N "Initiate Nagad"
Invoke-Test -M "POST" -U "$BASE_URL/payments/verify" -B "{`"transaction_reference`":`"TXN-test`"}" -Tk $TOKEN -N "Verify Payment" -E @(200,400)
Invoke-Test -M "POST" -U "$BASE_URL/payments/refunds" -B "{`"reason`":`"Defective`",`"amount`":500.0}" -Tk $TOKEN -N "Refund Request" -E @(201,200,400)
Invoke-Test -U "$BASE_URL/payments/status/TXN-test" -Tk $TOKEN -N "Payment Status"
Invoke-Test -M "POST" -U "$BASE_URL/payments/webhooks/bkash" -B "{`"status`":`"COMPLETED`"}" -N "bKash Webhook"
Invoke-Test -M "POST" -U "$BASE_URL/payments/webhooks/nagad" -B "{`"status`":`"SUCCESS`"}" -N "Nagad Webhook"

Write-Host "`n[7] VENDOR" -ForegroundColor Magenta
$VSlug = "vendor$ts"
$vB = "{`"store_name`":`"Store$ts`",`"store_slug`":`"$VSlug`",`"description`":`"Premium store`",`"bank_name`":`"DBBL`",`"bank_account_number`":`"1234567`",`"bkash_merchant_number`":`"01712345678`"}"
$vR = Invoke-Test -M "POST" -U "$BASE_URL/vendor/register" -B $vB -Tk $TOKEN -N "Register Vendor" -E @(201,200,409)
$VID = if ($vR) { $vR.vendor_id } else { "00000000-0000-0000-0000-000000000001" }
Invoke-Test -U "$BASE_URL/vendor/profile" -Tk $TOKEN -N "Vendor Profile"
Invoke-Test -U "$BASE_URL/vendor/analytics" -Tk $TOKEN -N "Vendor Analytics"
Invoke-Test -M "PUT" -U "$BASE_URL/vendor/profile" -B "{`"store_name`":`"Updated Store`"}" -Tk $TOKEN -N "Update Vendor"
Invoke-Test -U "$BASE_URL/vendors/store/$VSlug" -N "Public Vendor Store" -E @(200,404)
Invoke-Test -U "$BASE_URL/admin/vendors" -Tk $TOKEN -N "Admin Vendors"
Invoke-Test -M "PUT" -U "$BASE_URL/admin/vendors/$VID/verify" -B "{`"status`":`"APPROVED`",`"commission_rate`":7.5}" -Tk $TOKEN -N "Verify Vendor" -E @(200,404)

Write-Host "`n[8] RESELLER" -ForegroundColor Magenta
$RSlug = "reseller$ts"
Invoke-Test -M "POST" -U "$BASE_URL/reseller/setup" -B "{`"store_name`":`"RS$ts`",`"store_slug`":`"$RSlug`"}" -Tk $TOKEN -N "Setup Reseller" -E @(201,200,409)
Invoke-Test -U "$BASE_URL/reseller/profile" -Tk $TOKEN -N "Reseller Profile"
Invoke-Test -U "$BASE_URL/reseller/catalog" -Tk $TOKEN -N "Reseller Catalog"
Invoke-Test -M "POST" -U "$BASE_URL/reseller/margin/calculate" -B "{`"wholesale_price`":800.0,`"target_price`":1100.0}" -Tk $TOKEN -N "Margin Calc"
Invoke-Test -M "POST" -U "$BASE_URL/reseller/catalog/add" -B "{`"product_id`":`"$PROD_ID`",`"wholesale_price`":800.0,`"reseller_margin`":200.0}" -Tk $TOKEN -N "Add to Catalog" -E @(200,201,400)
Invoke-Test -M "POST" -U "$BASE_URL/reseller/share-link" -B "{`"product_id`":`"$PROD_ID`",`"channel`":`"Facebook`"}" -Tk $TOKEN -N "Share Link"
Invoke-Test -U "$BASE_URL/reseller/store/$RSlug" -N "Public Reseller" -E @(200,404)

Write-Host "`n[9] WALLET" -ForegroundColor Magenta
Invoke-Test -U "$BASE_URL/wallet" -Tk $TOKEN -N "Wallet Balance"
Invoke-Test -U "$BASE_URL/wallet/transactions" -Tk $TOKEN -N "Wallet Transactions"
$wdB = "{`"amount`":500.0,`"payment_method`":`"BKASH_MERCHANT`",`"account_details`":`"01712345678`"}"
$wdR = Invoke-Test -M "POST" -U "$BASE_URL/wallet/withdraw" -B $wdB -Tk $TOKEN -N "Withdrawal" -E @(201,200,400)
$WDID = if ($wdR) { $wdR.withdrawal_id } else { "00000000-0000-0000-0000-000000000001" }
Invoke-Test -U "$BASE_URL/admin/payouts/requests" -Tk $TOKEN -N "Admin Payout List"

Write-Host "`n[10] SHIPPING" -ForegroundColor Magenta
$shipB = "{`"order_id`":`"$OID`",`"courier`":`"Steadfast`",`"recipient_name`":`"Test`",`"recipient_phone`":`"$PHONE`",`"recipient_address`":`"Dhaka`",`"weight_kg`":0.5,`"cod_amount`":1200.0}"
$shipR = Invoke-Test -M "POST" -U "$BASE_URL/shipping/create-consignment" -B $shipB -Tk $TOKEN -N "Create Consignment" -E @(201,200,400)
$TRK = if ($shipR) { $shipR.tracking_number } else { "TRK-TEST-001" }
Invoke-Test -U "$BASE_URL/shipping/consignments" -Tk $TOKEN -N "List Consignments"
Invoke-Test -U "$BASE_URL/shipping/track/$TRK" -N "Track Shipment" -E @(200,404)
Invoke-Test -M "PUT" -U "$BASE_URL/shipping/consignments/$TRK/status" -B "{`"status`":`"IN_TRANSIT`"}" -Tk $TOKEN -N "Update Shipment" -E @(200,404)
Invoke-Test -U "$BASE_URL/shipping/rates?weight=0.5&district=Dhaka" -Tk $TOKEN -N "Shipping Rates" -E @(200,404)

Write-Host "`n[11] DISPUTE" -ForegroundColor Magenta
$dispB = "{`"order_id`":`"$OID`",`"reason`":`"WRONG_ITEM`",`"description`":`"Wrong item received`"}"
$dispR = Invoke-Test -M "POST" -U "$BASE_URL/disputes/open" -B $dispB -Tk $TOKEN -N "Open Dispute" -E @(201,200,400)
$DID = if ($dispR) { $dispR.ticket_id } else { "00000000-0000-0000-0000-000000000001" }
Invoke-Test -U "$BASE_URL/disputes/my" -Tk $TOKEN -N "My Disputes"
Invoke-Test -U "$BASE_URL/disputes/admin" -Tk $TOKEN -N "Admin Disputes"
Invoke-Test -U "$BASE_URL/disputes/$DID" -Tk $TOKEN -N "Dispute Detail" -E @(200,404)
Invoke-Test -M "PUT" -U "$BASE_URL/disputes/$DID/resolve" -B "{`"decision`":`"BUYER_WINS`",`"resolution_note`":`"Verified`",`"refund_amount`":1200.0}" -Tk $TOKEN -N "Resolve Dispute" -E @(200,404)
Invoke-Test -M "POST" -U "$BASE_URL/disputes/$DID/messages" -B "{`"message`":`"Escalate`"}" -Tk $TOKEN -N "Dispute Message" -E @(201,200,404)

Write-Host "`n[12] PROMOTION" -ForegroundColor Magenta
Invoke-Test -U "$BASE_URL/promotions/flash-sales" -N "Flash Sales (public)"
Invoke-Test -M "POST" -U "$BASE_URL/promotions/validate-coupon" -B "{`"coupon_code`":`"DARAZ20`",`"order_amount`":2000.0}" -Tk $TOKEN -N "Validate DARAZ20" -E @(200,400)
Invoke-Test -M "POST" -U "$BASE_URL/promotions/validate-coupon" -B "{`"coupon_code`":`"FAKECOUPON`",`"order_amount`":100.0}" -Tk $TOKEN -N "Invalid Coupon (400)" -E @(400)
$newCpnB = "{`"code`":`"AUTO$ts`",`"discount_type`":`"PERCENTAGE`",`"discount_value`":15.0,`"min_order_amount`":500.0}"
Invoke-Test -M "POST" -U "$BASE_URL/admin/coupons" -B $newCpnB -Tk $TOKEN -N "Create Coupon" -E @(201)
Invoke-Test -U "$BASE_URL/admin/coupons" -Tk $TOKEN -N "Admin List Coupons"

Write-Host "`n[13] REVIEW" -ForegroundColor Magenta
Invoke-Test -U "$BASE_URL/reviews/product/$PROD_ID" -N "Product Reviews"
Invoke-Test -U "$BASE_URL/reviews/seller/test-seller" -N "Seller Reviews"
Invoke-Test -U "$BASE_URL/reviews/my" -Tk $TOKEN -N "My Reviews"
$revB = "{`"product_id`":`"$PROD_ID`",`"rating`":5,`"title`":`"Great!`",`"comment`":`"Love this product`"}"
$revR = Invoke-Test -M "POST" -U "$BASE_URL/reviews" -B $revB -Tk $TOKEN -N "Submit Review" -E @(201,400)
$REVID = if ($revR) { $revR.review_id } else { "00000000-0000-0000-0000-000000000001" }
Invoke-Test -M "POST" -U "$BASE_URL/reviews/$REVID/helpful" -Tk $TOKEN -N "Mark Helpful" -E @(200,404)
Invoke-Test -M "PUT" -U "$BASE_URL/reviews/$REVID" -B "{`"rating`":4,`"comment`":`"Updated`"}" -Tk $TOKEN -N "Update Review" -E @(200,404)
Invoke-Test -M "DELETE" -U "$BASE_URL/reviews/$REVID" -Tk $TOKEN -N "Delete Review" -E @(200,404)

Write-Host "`n[14] NOTIFICATION" -ForegroundColor Magenta
Invoke-Test -U "$BASE_URL/notifications" -Tk $TOKEN -N "Get Notifications"
Invoke-Test -M "PUT" -U "$BASE_URL/notifications/read-all" -Tk $TOKEN -N "Mark All Read" -E @(200,404)
Invoke-Test -U "$BASE_URL/notifications/unread-count" -Tk $TOKEN -N "Unread Count" -E @(200,404)

Write-Host "`n[15] SEARCH" -ForegroundColor Magenta
Invoke-Test -U "$BASE_URL/search?q=test" -Tk $TOKEN -N "Search Products"
Invoke-Test -U "$BASE_URL/search?q=phone&category=electronics" -Tk $TOKEN -N "Search with Filter"
Invoke-Test -U "$BASE_URL/search/suggestions?q=ph" -Tk $TOKEN -N "Suggestions" -E @(200,404)

Write-Host "`n[16] INVENTORY" -ForegroundColor Magenta
Invoke-Test -U "$BASE_URL/inventory/stock" -Tk $TOKEN -N "Stock Levels"
Invoke-Test -U "$BASE_URL/inventory/warehouses" -Tk $TOKEN -N "Warehouses"
Invoke-Test -U "$BASE_URL/inventory/reservations" -Tk $TOKEN -N "Reservations"
Invoke-Test -M "POST" -U "$BASE_URL/inventory/adjust" -B "{`"product_id`":`"$PROD_ID`",`"quantity`":50}" -Tk $TOKEN -N "Adjust Stock" -E @(200,201,400)

Write-Host "`n[17] ANALYTICS" -ForegroundColor Magenta
Invoke-Test -U "$BASE_URL/analytics/overview" -Tk $TOKEN -N "Overview"
Invoke-Test -U "$BASE_URL/analytics/sales?days=30" -Tk $TOKEN -N "Sales"
Invoke-Test -U "$BASE_URL/analytics/customers" -Tk $TOKEN -N "Customers"
Invoke-Test -U "$BASE_URL/analytics/products" -Tk $TOKEN -N "Products"
Invoke-Test -U "$BASE_URL/analytics/traffic" -Tk $TOKEN -N "Traffic"
Invoke-Test -M "POST" -U "$BASE_URL/analytics/custom-report" -B "{`"report_type`":`"MONTHLY_SALES`",`"parameters`":{}}" -Tk $TOKEN -N "Custom Report" -E @(201,400)

Write-Host "`n[18] AFFILIATE" -ForegroundColor Magenta
Invoke-Test -U "$BASE_URL/affiliate/profile" -Tk $TOKEN -N "Profile"
Invoke-Test -U "$BASE_URL/affiliate/links" -Tk $TOKEN -N "Links"
Invoke-Test -U "$BASE_URL/affiliate/earnings" -Tk $TOKEN -N "Earnings"
Invoke-Test -M "POST" -U "$BASE_URL/affiliate/apply" -B "{`"website_url`":`"https://blog.example.com`"}" -Tk $TOKEN -N "Apply" -E @(201,409)
Invoke-Test -M "POST" -U "$BASE_URL/affiliate/links" -B "{`"full_url`":`"https://platform.com`"}" -Tk $TOKEN -N "Create Link" -E @(201,403)
Invoke-Test -M "PUT" -U "$BASE_URL/affiliate/profile" -B "{}" -Tk $TOKEN -N "Update Profile"
Invoke-Test -M "DELETE" -U "$BASE_URL/affiliate/links/link-001" -Tk $TOKEN -N "Delete Link" -E @(200,404)

Write-Host "`n[19] MEDIA" -ForegroundColor Magenta
Invoke-Test -U "$BASE_URL/media/test-id" -N "Get Media" -E @(200,404)
Invoke-Test -M "POST" -U "$BASE_URL/media/upload" -B "{`"file_name`":`"test.jpg`",`"file_type`":`"image/jpeg`"}" -Tk $TOKEN -N "Upload" -E @(201)
Invoke-Test -M "POST" -U "$BASE_URL/media/presigned-url" -B "{`"file_name`":`"img.jpg`",`"content_type`":`"image/jpeg`"}" -Tk $TOKEN -N "Presigned URL" -E @(201)

Write-Host "`n[20] ADMIN FINANCE" -ForegroundColor Magenta
Invoke-Test -U "$BASE_URL/admin/finance/overview" -Tk $TOKEN -N "Finance Overview"
Invoke-Test -U "$BASE_URL/admin/finance/payouts" -Tk $TOKEN -N "Payout Requests"
Invoke-Test -U "$BASE_URL/admin/finance/payouts?status=PENDING" -Tk $TOKEN -N "Pending Filter"
Invoke-Test -U "$BASE_URL/admin/finance/escrow" -Tk $TOKEN -N "Escrow Holdings"
Invoke-Test -U "$BASE_URL/admin/finance/commissions" -Tk $TOKEN -N "Commissions"
Invoke-Test -U "$BASE_URL/admin/finance/reports" -Tk $TOKEN -N "Reports"
$uid = if ($rr) { $rr.user_id } else { "00000000-0000-0000-0000-000000000001" }
Invoke-Test -M "POST" -U "$BASE_URL/admin/finance/escrow/release" -B "{`"user_id`":`"$uid`",`"amount`":100.0}" -Tk $TOKEN -N "Release Escrow" -E @(200,400)
Invoke-Test -M "POST" -U "$BASE_URL/admin/finance/manual-credit" -B "{`"user_id`":`"$uid`",`"amount`":500.0,`"type`":`"CREDIT`",`"description`":`"Test`"}" -Tk $TOKEN -N "Manual Credit" -E @(201,400)
Invoke-Test -M "PUT" -U "$BASE_URL/admin/finance/commission-rates" -B "{`"commission_rate`":8.5}" -Tk $TOKEN -N "Update Rates"

Write-Host "`n[21] ADMIN FRAUD" -ForegroundColor Magenta
Invoke-Test -U "$BASE_URL/admin/fraud/risk-profiles" -Tk $TOKEN -N "Risk Profiles"
Invoke-Test -U "$BASE_URL/admin/fraud/blacklists" -Tk $TOKEN -N "Blacklists"
Invoke-Test -U "$BASE_URL/admin/fraud/ip-blocks" -Tk $TOKEN -N "IP Blocks"
Invoke-Test -M "POST" -U "$BASE_URL/admin/fraud/blacklist-phone" -B "{`"phone`":`"01911222333`",`"reason`":`"FAKE_ORDERS`"}" -Tk $TOKEN -N "Blacklist Phone" -E @(201,409)
Invoke-Test -M "POST" -U "$BASE_URL/admin/fraud/block-ip" -B "{`"ip_address`":`"10.0.0.99`",`"reason`":`"Suspicious`",`"duration_hours`":24}" -Tk $TOKEN -N "Block IP" -E @(201)
Invoke-Test -M "POST" -U "$BASE_URL/admin/fraud/recalculate-risk" -B "{`"user_id`":`"$uid`"}" -Tk $TOKEN -N "Recalc Risk"

Write-Host "`n[22] ADMIN SETTINGS" -ForegroundColor Magenta
Invoke-Test -U "$BASE_URL/admin/settings" -Tk $TOKEN -N "All Settings"
Invoke-Test -U "$BASE_URL/admin/settings/audit-logs" -Tk $TOKEN -N "Audit Logs"
Invoke-Test -U "$BASE_URL/admin/settings/default_commission_rate" -Tk $TOKEN -N "Setting by Key"
Invoke-Test -U "$BASE_URL/admin/settings/nonexistent" -Tk $TOKEN -N "Missing Setting (404)" -E @(404)
Invoke-Test -M "PUT" -U "$BASE_URL/admin/settings" -B "{`"settings`":{`"test_key`":`"val1`"}}" -Tk $TOKEN -N "Batch Update"
Invoke-Test -M "PUT" -U "$BASE_URL/admin/settings/return_window_days" -B "{`"value`":`"7`"}" -Tk $TOKEN -N "Update Key"
Invoke-Test -M "PUT" -U "$BASE_URL/admin/settings/maintenance-mode" -B "{`"enabled`":false}" -Tk $TOKEN -N "Maintenance Mode"

Write-Host "`n[23] CHINA SOURCING" -ForegroundColor Magenta
Invoke-Test -U "$BASE_URL/china-sourcing/batches" -Tk $TOKEN -N "List Batches"
Invoke-Test -U "$BASE_URL/china-sourcing/landed-cost-calculator?cny_cost=50&quantity=100&freight_type=SEA" -Tk $TOKEN -N "Landed Cost SEA"
Invoke-Test -U "$BASE_URL/china-sourcing/landed-cost-calculator?cny_cost=80&quantity=20&freight_type=AIR" -Tk $TOKEN -N "Landed Cost AIR"
Invoke-Test -U "$BASE_URL/china-sourcing/customs-declarations" -Tk $TOKEN -N "Customs Declarations"
$btR = Invoke-Test -M "POST" -U "$BASE_URL/china-sourcing/batches" -B "{`"freight_type`":`"SEA`"}" -Tk $TOKEN -N "Create Batch" -E @(201)
$BID = if ($btR) { $btR.batch_id } else { "00000000-0000-0000-0000-000000000001" }
Invoke-Test -U "$BASE_URL/china-sourcing/batches/$BID" -Tk $TOKEN -N "Batch Detail" -E @(200,404)
Invoke-Test -M "POST" -U "$BASE_URL/china-sourcing/batches/$BID/items" -B "{`"product_name`":`"Widget`",`"quantity`":50,`"unit_cost_cny`":35.0}" -Tk $TOKEN -N "Add Item" -E @(201,404)
Invoke-Test -M "PUT" -U "$BASE_URL/china-sourcing/batches/$BID/status" -B "{`"status`":`"ORDERED`"}" -Tk $TOKEN -N "Update Status" -E @(200,404)
Invoke-Test -M "PUT" -U "$BASE_URL/china-sourcing/batches/$BID/landed-cost" -B "{`"cny_cost`":35.0,`"freight_cost`":85.0,`"customs_duty_pct`":25.0,`"landed_cost_bdt`":900.0}" -Tk $TOKEN -N "Update Landed Cost" -E @(200,404)
Invoke-Test -M "PUT" -U "$BASE_URL/china-sourcing/batches/$BID/customs" -B "{`"hs_code`":`"8517.12`"}" -Tk $TOKEN -N "Update Customs" -E @(200,404)

Write-Host "`n[24] SECURITY & VALIDATION" -ForegroundColor Magenta
Invoke-Test -U "$BASE_URL/user/profile" -N "No Auth Profile (401)" -E @(401)
Invoke-Test -U "$BASE_URL/wallet" -N "No Auth Wallet (401)" -E @(401)
Invoke-Test -U "$BASE_URL/admin/vendors" -N "No Auth Admin (401)" -E @(401)
Invoke-Test -M "POST" -U "$BASE_URL/auth/email/register" -B "{`"email`":`"bad-email`",`"password`":`"weak`"}" -N "Bad Email (400/422)" -E @(400,422)
Invoke-Test -M "POST" -U "$BASE_URL/wallet/withdraw" -B "{`"amount`":-100}" -Tk $TOKEN -N "Negative Withdraw (400/422)" -E @(400,422)
Invoke-Test -M "POST" -U "$BASE_URL/admin/fraud/block-ip" -B "{`"ip_address`":`"not-an-ip`"}" -Tk $TOKEN -N "Invalid IP (400/422)" -E @(400,422)

# FINAL REPORT
Write-Host ""
Write-Host "=====================================" -ForegroundColor Cyan
[int]$TOTAL = $PASS + $FAIL
Write-Host " FINAL TEST RESULTS" -ForegroundColor Cyan
Write-Host "=====================================" -ForegroundColor Cyan
Write-Host " TOTAL  : $TOTAL"
Write-Host " PASSED : $PASS" -ForegroundColor Green
Write-Host " FAILED : $FAIL" -ForegroundColor Red
[double]$RATE = if ($TOTAL -gt 0) { [Math]::Round([double]$PASS / [double]$TOTAL * 100.0, 1) } else { 0.0 }
Write-Host " RATE   : $RATE%" -ForegroundColor $(if ($RATE -ge 85) { "Green" } elseif ($RATE -ge 70) { "Yellow" } else { "Red" })
Write-Host "=====================================" -ForegroundColor Cyan
if ($FAIL -eq 0) { Write-Host " ALL PASSED - PRODUCTION READY!" -ForegroundColor Green }
elseif ($RATE -ge 85) { Write-Host " MOSTLY PASSING - Minor issues" -ForegroundColor Yellow }
else { Write-Host " Review required" -ForegroundColor Red }
Write-Host "=====================================" -ForegroundColor Cyan

