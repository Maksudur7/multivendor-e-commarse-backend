# Deep Multi-Angle Automated Test Suite
$ErrorActionPreference = "Continue"
$baseUrl = "http://localhost:8080/api/v1"

$passed = 0
$failed = 0
$total = 0

function Test-Endpoint {
    param(
        [string]$Category,
        [string]$Name,
        [string]$Method,
        [string]$Path,
        [hashtable]$Headers,
        [string]$Body,
        [int]$ExpectedStatus
    )
    $script:total++
    $url = "$baseUrl$Path"
    try {
        $params = @{
            Uri = $url
            Method = $Method
            ContentType = "application/json"
            Headers = $Headers
            UseBasicParsing = $true
        }
        if ($Body) { $params.Body = $Body }
        
        $res = Invoke-WebRequest @params
        $status = [int]$res.StatusCode
    } catch {
        if ($_.Exception.Response) {
            $status = [int]$_.Exception.Response.StatusCode
        } else {
            $status = 0
        }
    }

    if ($status -eq $ExpectedStatus) {
        $script:passed++
        Write-Host "  [PASS] [$Category] $Name -> HTTP $status (Expected $ExpectedStatus)" -ForegroundColor Green
    } else {
        $script:failed++
        Write-Host "  [FAIL] [$Category] $Name -> HTTP $status (Expected $ExpectedStatus)" -ForegroundColor Red
    }
}

Write-Host "============================================================" -ForegroundColor Cyan
Write-Host "DEEP MULTI-ANGLE API AUDIT AND STRESS VERIFICATION" -ForegroundColor Cyan
Write-Host "============================================================" -ForegroundColor Cyan

# 1. AUTH AND USER DEEP TESTS
Write-Host "`n--- Angle 1: Authentication and User Validation ---" -ForegroundColor Yellow
$regBody = '{"email":"audit_user_99@test.com","password":"Password123!","full_name":"Audit User","phone":"+8801700998877"}'
Test-Endpoint -Category "AUTH" -Name "Valid Email Register" -Method "POST" -Path "/auth/email/register" -Headers @{} -Body $regBody -ExpectedStatus 201

$loginBody = '{"email":"audit_user_99@test.com","password":"Password123!"}'
$loginRes = Invoke-RestMethod -Uri "$baseUrl/auth/email/login" -Method Post -Body $loginBody -ContentType "application/json"
$token = $loginRes.data.access_token
$authHeaders = @{ Authorization = "Bearer $token" }

Test-Endpoint -Category "AUTH" -Name "Duplicate Register Error (409)" -Method "POST" -Path "/auth/email/register" -Headers @{} -Body $regBody -ExpectedStatus 409
Test-Endpoint -Category "AUTH" -Name "Invalid Email Format (400/422)" -Method "POST" -Path "/auth/email/register" -Headers @{} -Body '{"email":"bad-email","password":"123"}' -ExpectedStatus 422
Test-Endpoint -Category "AUTH" -Name "Wrong Password Login (401)" -Method "POST" -Path "/auth/email/login" -Headers @{} -Body '{"email":"audit_user_99@test.com","password":"WrongPassword!"}' -ExpectedStatus 401
Test-Endpoint -Category "AUTH" -Name "Unauthenticated Profile Access (401)" -Method "GET" -Path "/user/profile" -Headers @{} -Body $null -ExpectedStatus 401
Test-Endpoint -Category "AUTH" -Name "Authenticated Profile Access (200)" -Method "GET" -Path "/user/profile" -Headers $authHeaders -Body $null -ExpectedStatus 200

# 2. VENDOR AND RESELLER EDGE CASES
Write-Host "`n--- Angle 2: Vendor and Reseller Input Boundaries ---" -ForegroundColor Yellow
Test-Endpoint -Category "VENDOR" -Name "Register Store Missing Name (422)" -Method "POST" -Path "/vendor/register" -Headers $authHeaders -Body '{"description":"test"}' -ExpectedStatus 422
Test-Endpoint -Category "VENDOR" -Name "Valid Store Registration (201)" -Method "POST" -Path "/vendor/register" -Headers $authHeaders -Body '{"store_name":"Audit Store","store_slug":"audit-store-99"}' -ExpectedStatus 201
Test-Endpoint -Category "RESELLER" -Name "Calculate Margin (Zero Wholesale)" -Method "POST" -Path "/reseller/margin/calculate" -Headers $authHeaders -Body '{"wholesale_price":0,"target_price":500}' -ExpectedStatus 200
Test-Endpoint -Category "RESELLER" -Name "Calculate Margin (Valid)" -Method "POST" -Path "/reseller/margin/calculate" -Headers $authHeaders -Body '{"wholesale_price":1000,"target_price":1350}' -ExpectedStatus 200

# 3. PROMOTIONS AND DISCOUNTS
Write-Host "`n--- Angle 3: Coupon and Promotion Validation ---" -ForegroundColor Yellow
Test-Endpoint -Category "PROMOTION" -Name "Valid Fixed Coupon (DARAZ20)" -Method "POST" -Path "/promotions/validate-coupon" -Headers $authHeaders -Body '{"coupon_code":"DARAZ20","order_amount":500}' -ExpectedStatus 200
Test-Endpoint -Category "PROMOTION" -Name "Invalid Coupon Code (400)" -Method "POST" -Path "/promotions/validate-coupon" -Headers $authHeaders -Body '{"coupon_code":"NONEXISTENT999","order_amount":500}' -ExpectedStatus 400
Test-Endpoint -Category "PROMOTION" -Name "Empty Coupon Code (422)" -Method "POST" -Path "/promotions/validate-coupon" -Headers $authHeaders -Body '{"coupon_code":"","order_amount":500}' -ExpectedStatus 422

# 4. CART AND CHECKOUT SANITY
Write-Host "`n--- Angle 4: Cart and Multi-Vendor Checkout ---" -ForegroundColor Yellow
Test-Endpoint -Category "CART" -Name "Get Empty/Existing Cart" -Method "GET" -Path "/cart" -Headers $authHeaders -Body $null -ExpectedStatus 200
Test-Endpoint -Category "CART" -Name "Add Item to Cart" -Method "POST" -Path "/cart/items" -Headers $authHeaders -Body '{"product_id":"d0000000-0000-0000-0000-000000000001","quantity":2,"unit_price":1200}' -ExpectedStatus 201
Test-Endpoint -Category "CHECKOUT" -Name "Process Checkout" -Method "POST" -Path "/customer/orders/checkout" -Headers $authHeaders -Body '{"shipping_address":"House 12, Road 5, Dhanmondi, Dhaka","payment_method":"COD"}' -ExpectedStatus 201

# 5. ADMIN FRAUD AND RISK ENFORCEMENT
Write-Host "`n--- Angle 5: Admin Fraud and IP Blocking Boundaries ---" -ForegroundColor Yellow
Test-Endpoint -Category "FRAUD" -Name "Blacklist Phone Invalid Reason (422)" -Method "POST" -Path "/admin/fraud/blacklist-phone" -Headers $authHeaders -Body '{"phone":"01711000000","reason":"INVALID_REASON"}' -ExpectedStatus 422
Test-Endpoint -Category "FRAUD" -Name "Blacklist Phone Valid (201)" -Method "POST" -Path "/admin/fraud/blacklist-phone" -Headers $authHeaders -Body '{"phone":"01711000000","reason":"FRAUD","note":"Repeated COD fake order"}' -ExpectedStatus 201
Test-Endpoint -Category "FRAUD" -Name "Block Invalid IP Format (422)" -Method "POST" -Path "/admin/fraud/block-ip" -Headers $authHeaders -Body '{"ip_address":"999.999.999.999"}' -ExpectedStatus 422
Test-Endpoint -Category "FRAUD" -Name "Block Valid IP Address (201)" -Method "POST" -Path "/admin/fraud/block-ip" -Headers $authHeaders -Body '{"ip_address":"192.168.1.100","reason":"Bot brute force","duration_hours":48}' -ExpectedStatus 201

# 6. CHINA SOURCING LOGISTICS
Write-Host "`n--- Angle 6: China Import and Landed Cost Calculation ---" -ForegroundColor Yellow
Test-Endpoint -Category "CHINA" -Name "Landed Cost Missing CNY (422)" -Method "GET" -Path "/china-sourcing/landed-cost-calculator?cny_cost=0" -Headers $authHeaders -Body $null -ExpectedStatus 422
Test-Endpoint -Category "CHINA" -Name "Landed Cost Sea Freight" -Method "GET" -Path "/china-sourcing/landed-cost-calculator?cny_cost=50`&quantity=100`&freight_type=SEA" -Headers $authHeaders -Body $null -ExpectedStatus 200
Test-Endpoint -Category "CHINA" -Name "Landed Cost Air Freight" -Method "GET" -Path "/china-sourcing/landed-cost-calculator?cny_cost=50`&quantity=100`&freight_type=AIR" -Headers $authHeaders -Body $null -ExpectedStatus 200

# SUMMARY
Write-Host "`n============================================================" -ForegroundColor Cyan
Write-Host "MULTI-ANGLE AUDIT SUMMARY" -ForegroundColor Cyan
Write-Host "============================================================" -ForegroundColor Cyan
Write-Host " TOTAL TESTS EXECUTED : $total"
Write-Host " PASSED               : $passed" -ForegroundColor Green
Write-Host " FAILED               : $failed" -ForegroundColor Red
Write-Host " SUCCESS RATE         : $([math]::Round(($passed/$total)*100, 2))%"
