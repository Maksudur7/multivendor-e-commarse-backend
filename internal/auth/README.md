# 🔐 Authentication Module — Next.js App Router Integration Guide & Complete API Reference

> **ফাইল লোকেশন:** `internal/auth/README.md`  
> **লাইভ Vercel Base URL:** `https://e-commarse-three.vercel.app/api/v1/auth`  
> **লোকাল Base URL:** `http://localhost:8080/api/v1/auth`  
> **টার্গেট ফ্রন্টএন্ড স্ট্যাক:** **Next.js (App Router 13/14/15) + TypeScript + Axios / Fetch**

---

## 📑 সুচিপত্র (Table of Contents)
1. [আর্কিটেকচার ও Next.js সিকিউরিটি](#1-আর্কিটেকচার-ও-nextjs-সিকিউরিটি)
2. [Next.js API Client Setup (Auto Token Refresh)](#2-nextjs-api-client-setup-auto-token-refresh)
3. [Next.js Middleware (Protected Routes & Auth Guard)](#3-nextjs-middleware-protected-routes--auth-guard)
4. [Next.js Email Register & Verification Flow](#4-nextjs-email-register--verification-flow)
5. [Next.js Phone & WhatsApp OTP Login Component](#5-nextjs-phone--whatsapp-otp-login-component)
6. [Next.js Google OAuth 2.0 Integration](#6-nextjs-google-oauth-20-integration)
7. [Next.js Password Reset Flow](#7-nextjs-password-reset-flow)
8. [সকল ১৬টি API এন্ডপয়েন্টের পুঙ্খানুপুঙ্খ নির্দেশিকা (Complete 16 APIs Reference)](#8-সকল-১৬টি-api-এন্ডপয়েন্টের-পুঙ্খানুপুঙ্খ-নির্দেশিকা-complete-16-apis-reference)
   - [8.1 POST /auth/email/register](#81-post-authemailregister)
   - [8.2 POST /auth/email/login](#82-post-authemaillogin)
   - [8.3 POST /auth/otp/send](#83-post-authotpsend)
   - [8.4 POST /auth/phone/send-otp](#84-post-authphonesend-otp)
   - [8.5 POST /auth/otp/verify](#85-post-authotpverify)
   - [8.6 POST /auth/refresh](#86-post-authrefresh)
   - [8.7 GET /auth/email/verify](#87-get-authemailverify)
   - [8.8 POST /auth/email/resend-verification](#88-post-authemailresend-verification)
   - [8.9 POST /auth/password/reset-request](#89-post-authpasswordreset-request)
   - [8.10 PUT /auth/password/reset](#810-put-authpasswordreset)
   - [8.11 GET /auth/google](#811-get-authgoogle)
   - [8.12 GET /auth/google/callback](#812-get-authgooglecallback)
   - [8.13 POST /auth/logout](#813-post-authlogout)
   - [8.14 GET /auth/me](#814-get-authme)
   - [8.15 GET /auth/sessions](#815-get-authsessions)
   - [8.16 DELETE /auth/sessions/:sessionId](#816-delete-authsessionssessionid)

---

# 1. আর্কিটেকচার ও Next.js সিকিউরিটি

এই ব্যাকএন্ডে **JWT Access Token (15m)** এবং **Refresh Token Rotation (7d)** ব্যবহৃত হয়েছে। Next.js App Router-এ ক্লায়েন্ট ও সার্ভার দুই জায়গাতেই টোকেন সিকিউর রাখতে নিচের প্র্যাকটিস ফলো করুন:

| টোকেন | মেয়াদ | Next.js স্টোরেজ অবস্থান | ব্যবহার |
|---|---|---|---|
| **Access Token** | ১৫ মিনিট | `Cookies` (js-cookie) / Zustand Store | API Header: `Authorization: Bearer <access_token>` |
| **Refresh Token** | ৭ দিন | `Cookies` (`httpOnly` secure) | `/auth/refresh` কল করার জন্য |

---

# 2. Next.js API Client Setup (Auto Token Refresh)

আপনার Next.js প্রজেক্টের `lib/apiClient.ts` ফাইলে এই ক্লায়েন্টটি তৈরি করুন। এটি Access Token এর মেয়াদ শেষ হলে ব্যাকগ্রাউন্ডে স্বয়ংক্রিয়ভাবে রিফ্রেশ টোকেন দিয়ে নতুন টোকেন সংগ্রহ করবে:

```typescript
// lib/apiClient.ts
import axios from "axios";
import Cookies from "js-cookie";

const API_BASE = "https://e-commarse-three.vercel.app/api/v1";

export const apiClient = axios.create({
  baseURL: API_BASE,
  headers: {
    "Content-Type": "application/json",
  },
});

// 1. Attach Bearer Access Token in requests
apiClient.interceptors.request.use((config) => {
  const token = Cookies.get("access_token");
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// 2. Auto Refresh Token on 401 Unauthorized
apiClient.interceptors.response.use(
  (response) => response,
  async (error) => {
    const originalRequest = error.config;

    if (error.response?.status === 401 && !originalRequest._retry) {
      originalRequest._retry = true;
      try {
        const refreshToken = Cookies.get("refresh_token");
        if (!refreshToken) throw new Error("No refresh token");

        const { data } = await axios.post(`${API_BASE}/auth/refresh`, {
          refresh_token: refreshToken,
        });

        const newAccessToken = data.data.access_token;
        const newRefreshToken = data.data.refresh_token;

        // Save rotated tokens in Cookies
        Cookies.set("access_token", newAccessToken, { expires: 1 / 96 }); // 15 mins
        Cookies.set("refresh_token", newRefreshToken, { expires: 7 }); // 7 days

        originalRequest.headers.Authorization = `Bearer ${newAccessToken}`;
        return apiClient(originalRequest);
      } catch (refreshErr) {
        Cookies.remove("access_token");
        Cookies.remove("refresh_token");
        if (typeof window !== "undefined") {
          window.location.href = "/login";
        }
        return Promise.reject(refreshErr);
      }
    }
    return Promise.reject(error);
  }
);
```

---

# 3. Next.js Middleware (Protected Routes & Auth Guard)

Next.js App Router-এর মূল ফোল্ডারে `middleware.ts` তৈরি করুন। এটি লগইন ছাড়া ইউজারদের প্রোফাইল, ড্যাশবোর্ড বা চেকআউট পেজে যেতে বাধা দেবে:

```typescript
// middleware.ts
import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

const protectedRoutes = ["/dashboard", "/profile", "/checkout", "/orders"];
const authRoutes = ["/login", "/register", "/verify-email"];

export function middleware(request: NextRequest) {
  const token = request.cookies.get("access_token")?.value;
  const { pathname } = request.nextUrl;

  if (!token && protectedRoutes.some((route) => pathname.startsWith(route))) {
    const loginUrl = new URL("/login", request.url);
    loginUrl.searchParams.set("from", pathname);
    return NextResponse.redirect(loginUrl);
  }

  if (token && authRoutes.some((route) => pathname.startsWith(route))) {
    return NextResponse.redirect(new URL("/dashboard", request.url));
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/dashboard/:path*", "/profile/:path*", "/checkout/:path*", "/login", "/register"],
};
```

---

# 4. Next.js Email Register & Verification Flow

#### ১. রেজিস্ট্রেশন ফর্ম (`app/register/page.tsx`)
```tsx
"use client";
import { useState } from "react";
import { apiClient } from "@/lib/apiClient";
import Cookies from "js-cookie";
import { useRouter } from "next/navigation";

export default function RegisterPage() {
  const [form, setForm] = useState({ name: "", email: "", phone: "", password: "" });
  const [loading, setLoading] = useState(false);
  const router = useRouter();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      const { data } = await apiClient.post("/auth/email/register", form);
      const { access_token, refresh_token } = data.data;

      Cookies.set("access_token", access_token, { expires: 1 / 96 });
      Cookies.set("refresh_token", refresh_token, { expires: 7 });

      alert("অ্যাকাউন্ট তৈরি সফল হয়েছে! আপনার ইমেইলে ভেরিফিকেশন লিংক পাঠানো হয়েছে।");
      router.push("/dashboard");
    } catch (err: any) {
      alert(err.response?.data?.error?.message || "রেজিস্ট্রেশনে সমস্যা হয়েছে");
    } finally {
      setLoading(false);
    }
  };

  return (
    <form onSubmit={handleSubmit} className="max-w-md mx-auto p-6 space-y-4">
      <h1 className="text-2xl font-bold">Register Account</h1>
      <input className="w-full border p-2 rounded" placeholder="Full Name" onChange={(e) => setForm({...form, name: e.target.value})} required />
      <input className="w-full border p-2 rounded" type="email" placeholder="Email Address" onChange={(e) => setForm({...form, email: e.target.value})} required />
      <input className="w-full border p-2 rounded" placeholder="Phone (e.g. 01880829496)" onChange={(e) => setForm({...form, phone: e.target.value})} required />
      <input className="w-full border p-2 rounded" type="password" placeholder="Password" onChange={(e) => setForm({...form, password: e.target.value})} required />
      <button disabled={loading} className="w-full bg-blue-600 text-white py-2 rounded font-bold">
        {loading ? "Registering..." : "Create Account"}
      </button>
    </form>
  );
}
```

#### ২. ইমেইল ভেরিফিকেশন হ্যান্ডলার (`app/verify-email/page.tsx`)
```tsx
"use client";
import { useEffect, useState } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import { apiClient } from "@/lib/apiClient";

export default function VerifyEmailPage() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const [status, setStatus] = useState("ইমেইল ভেরিফাই করা হচ্ছে...");

  useEffect(() => {
    const token = searchParams.get("token");
    const uid = searchParams.get("uid");

    if (token && uid) {
      apiClient
        .get(`/auth/email/verify?token=${encodeURIComponent(token)}&uid=${encodeURIComponent(uid)}`)
        .then(() => {
          setStatus("✅ ইমেইল ভেরিফিকেশন সফল হয়েছে!");
          setTimeout(() => router.push("/dashboard"), 2000);
        })
        .catch((err) => {
          setStatus("❌ " + (err.response?.data?.error?.message || "ভেরিফিকেশন লিংকটির মেয়াদ শেষ।"));
        });
    }
  }, [searchParams, router]);

  return (
    <div className="flex flex-col items-center justify-center min-h-[60vh]">
      <h1 className="text-2xl font-bold">{status}</h1>
    </div>
  );
}
```

---

# 5. Next.js Phone & WhatsApp OTP Login Component

```tsx
"use client";
import { useState } from "react";
import { apiClient } from "@/lib/apiClient";
import Cookies from "js-cookie";
import { useRouter } from "next/navigation";

export default function OTPLoginPage() {
  const [phone, setPhone] = useState("");
  const [otp, setOtp] = useState("");
  const [step, setStep] = useState<"SEND" | "VERIFY">("SEND");
  const router = useRouter();

  const handleSendOTP = async () => {
    try {
      await apiClient.post("/auth/otp/send", { phone, purpose: "LOGIN" });
      setStep("VERIFY");
      alert("আপনার হোয়াটসঅ্যাপ / ফোনে ৬ ডিজিটের OTP পাঠানো হয়েছে।");
    } catch (err: any) {
      alert(err.response?.data?.error?.message || "OTP পাঠাতে ব্যর্থ হয়েছে");
    }
  };

  const handleVerifyOTP = async () => {
    try {
      const { data } = await apiClient.post("/auth/otp/verify", { phone, otp, purpose: "LOGIN" });
      const { access_token, refresh_token } = data.data;

      Cookies.set("access_token", access_token, { expires: 1 / 96 });
      Cookies.set("refresh_token", refresh_token, { expires: 7 });

      router.push("/dashboard");
    } catch (err: any) {
      alert(err.response?.data?.error?.message || "ভুল OTP প্রদান করা হয়েছে");
    }
  };

  return (
    <div className="max-w-md mx-auto p-6 space-y-4">
      <h2 className="text-xl font-bold">WhatsApp / Phone OTP Login</h2>
      {step === "SEND" ? (
        <>
          <input className="w-full border p-2 rounded" placeholder="Phone (e.g. 01880829496)" value={phone} onChange={(e) => setPhone(e.target.value)} />
          <button onClick={handleSendOTP} className="w-full bg-green-600 text-white p-2 rounded font-bold">
            Send OTP Code
          </button>
        </>
      ) : (
        <>
          <input className="w-full border p-2 rounded" placeholder="Enter 6-digit OTP" value={otp} onChange={(e) => setOtp(e.target.value)} maxLength={6} />
          <button onClick={handleVerifyOTP} className="w-full bg-blue-600 text-white p-2 rounded font-bold">
            Verify & Login
          </button>
        </>
      )}
    </div>
  );
}
```

---

# 6. Next.js Google OAuth 2.0 Integration

```tsx
"use client";

export function GoogleLoginButton() {
  const handleGoogleLogin = () => {
    window.location.href = "https://e-commarse-three.vercel.app/api/v1/auth/google";
  };

  return (
    <button onClick={handleGoogleLogin} className="w-full flex items-center justify-center gap-2 border p-2 rounded hover:bg-gray-50">
      <span>🌐 Sign in with Google</span>
    </button>
  );
}
```

---

# 7. Next.js Password Reset Flow

```typescript
// 1. Password Reset Request
async function requestPasswordReset(email: string) {
  await apiClient.post("/auth/password/reset-request", { email });
  alert("পাসওয়ার্ড রিসেট OTP আপনার ইমেইলে পাঠানো হয়েছে।");
}

// 2. Submit New Password with OTP
async function submitPasswordReset(email: string, otp: string, newPass: string) {
  await apiClient.put("/auth/password/reset", {
    target: email,
    otp: otp,
    new_password: newPass,
  });
  alert("পাসওয়ার্ড সফলভাবে পরিবর্তিত হয়েছে! নতুন পাসওয়ার্ড দিয়ে লগইন করুন।");
}
```

---

# 8. সকল ১৬টি API এন্ডপয়েন্টের পুঙ্খানুপুঙ্খ নির্দেশিকা (Complete 16 APIs Reference)

---

### 8.1 `POST /auth/email/register`
- **Method:** `POST`
- **Access Level:** Public
- **বিবরণ:** ইমেইল, মোবাইল নাম্বার এবং পাসওয়ার্ড দিয়ে নতুন ইউজার অ্যাকাউন্ট তৈরি করে।

#### Request Body:
```json
{
  "name": "Maksudur Rahman",
  "email": "maksudurr538@gmail.com",
  "phone": "01880829496",
  "password": "Password123!"
}
```

#### Response (`200 OK`):
```json
{
  "success": true,
  "message": "Account created successfully. Please verify your email.",
  "data": {
    "user_id": "f48dc608-8585-46f0-9fb5-dd20c8201af4",
    "email": "maksudurr538@gmail.com",
    "phone": "01880829496",
    "full_name": "Maksudur Rahman",
    "role": "CUSTOMER",
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refresh_token": "7c23ee77af08fbedd08eba6f64dbb810ebc969221a7122c444fcbfc5dc07e04a"
  }
}
```

---

### 8.2 `POST /auth/email/login`
- **Method:** `POST`
- **Access Level:** Public (Login Rate Limited)
- **বিবরণ:** ইমেইল ও পাসওয়ার্ড যাচাই করে লগইন করায় এবং JWT টোকেন জোড়া প্রদান করে।

#### Request Body:
```json
{
  "email": "maksudurr538@gmail.com",
  "password": "Password123!"
}
```

#### Response (`200 OK`):
```json
{
  "success": true,
  "message": "Login successful",
  "data": {
    "user_id": "f48dc608-8585-46f0-9fb5-dd20c8201af4",
    "email": "maksudurr538@gmail.com",
    "role": "CUSTOMER",
    "access_token": "eyJhbGciOiJIUzI1...",
    "refresh_token": "7c23ee77af08..."
  }
}
```

---

### 8.3 `POST /auth/otp/send`
- **Method:** `POST`
- **Access Level:** Public (OTP Rate Limited)
- **বিবরণ:** যেকোনো মোবাইল নাম্বারে WhatsApp/SMS অথবা ইমেইলে ৫ মিনিট মেয়াদী ৬-ডিজিটের OTP পাঠায়।

#### Request Body:
```json
{
  "target": "01880829496",
  "purpose": "LOGIN",
  "channel": "AUTO"
}
```
*(purpose options: `LOGIN`, `REGISTER`, `PASSWORD_RESET` | channel options: `WHATSAPP`, `SMS`, `AUTO`)*

#### Response (`200 OK`):
```json
{
  "success": true,
  "message": "OTP sent successfully",
  "data": {
    "target": "01880829496",
    "expires_in": "300s"
  }
}
```

---

### 8.4 `POST /auth/phone/send-otp`
- **Method:** `POST`
- **Access Level:** Public (OTP Rate Limited)
- **বিবরণ:** `/auth/otp/send`-এর এলিয়াস এন্ডপয়েন্ট। বিশেষত মোবাইল নাম্বারে SMS/WhatsApp OTP পাঠাতে ব্যবহৃত হয়।

#### Request Body:
```json
{
  "phone": "01880829496",
  "purpose": "LOGIN"
}
```

---

### 8.5 `POST /auth/otp/verify`
- **Method:** `POST`
- **Access Level:** Public (OTP Rate Limited)
- **বিবরণ:** ৬-ডিজিটের OTP কোড যাচাই করে লগইন করায়। **নিরাপত্তা:** টানা ৫ বার ভুল ওটিপি দিলে ৪২৯ এরর দিয়ে ওটিপি লক হয়ে যাবে।

#### Request Body:
```json
{
  "target": "01880829496",
  "otp": "482910",
  "purpose": "LOGIN"
}
```

#### Response (`200 OK`):
```json
{
  "success": true,
  "message": "OTP verified successfully",
  "data": {
    "user_id": "f48dc608-8585-46f0-9fb5-dd20c8201af4",
    "phone": "01880829496",
    "role": "CUSTOMER",
    "access_token": "eyJhbGciOiJIUzI1...",
    "refresh_token": "7c23ee77af08..."
  }
}
```

---

### 8.6 `POST /auth/refresh`
- **Method:** `POST`
- **Access Level:** Public
- **বিবরণ:** Access Token এর মেয়াদ শেষ হয়ে গেলে মেয়াদী Refresh Token প্রদান করে নতুন Access Token ও রোটেশনাল Refresh Token গ্রহণ করুন।

#### Request Body:
```json
{
  "refresh_token": "7c23ee77af08fbedd08eba6f64dbb810ebc969221a7122c444fcbfc5dc07e04a"
}
```

#### Response (`200 OK`):
```json
{
  "success": true,
  "message": "Access token refreshed successfully",
  "data": {
    "access_token": "eyJhbGciOiJIUzI1...",
    "refresh_token": "new_rotated_refresh_token_string"
  }
}
```

---

### 8.7 `GET /auth/email/verify`
- **Method:** `GET`
- **Access Level:** Public
- **Query Params:** `?token=<token_hash>&uid=<user_id>`
- **বিবরণ:** ইমেইলে প্রাপ্ত ভেরিফিকেশন লিংকে ক্লিক করলে ডাটাবেজে ইউজারের `email_verified` স্ট্যাটাস `true` করে।

#### Request Example:
`GET https://e-commarse-three.vercel.app/api/v1/auth/email/verify?token=abc123hash&uid=f48dc608-8585-46f0-9fb5-dd20c8201af4`

#### Response (`200 OK`):
```json
{
  "success": true,
  "message": "Email address verified successfully!"
}
```

---

### 8.8 `POST /auth/email/resend-verification`
- **Method:** `POST`
- **Access Level:** Protected (`Authorization: Bearer <access_token>`)
- **বিবরণ:** ইমেইল ভেরিফাইড না থাকলে পুনরায় নতুন ভেরিফিকেশন লিংক মেইলে পাঠায়।

#### Response (`200 OK`):
```json
{
  "success": true,
  "message": "Verification link sent to your email address."
}
```

---

### 8.9 `POST /auth/password/reset-request`
- **Method:** `POST`
- **Access Level:** Public
- **বিবরণ:** পাসওয়ার্ড ভুলে গেলে ইমেইলে ১৫ মিনিট মেয়াদী রিসেট ওটিপি পাঠায়।

#### Request Body:
```json
{
  "email": "maksudurr538@gmail.com"
}
```

#### Response (`200 OK`):
```json
{
  "success": true,
  "message": "If this email is registered, a password reset OTP has been sent.",
  "data": {
    "status": "DISPATCHED",
    "expires_in": "900s"
  }
}
```

---

### 8.10 `PUT /auth/password/reset`
- **Method:** `PUT`
- **Access Level:** Public
- **বিবরণ:** প্রাপ্ত রিসেট ওটিপি কোড দিয়ে নতুন পাসওয়ার্ড সেভ করে।

#### Request Body:
```json
{
  "target": "maksudurr538@gmail.com",
  "otp": "839201",
  "new_password": "NewSecurePassword123!"
}
```

#### Response (`200 OK`):
```json
{
  "success": true,
  "message": "Password reset successfully. Please log in with your new password."
}
```

---

### 8.11 `GET /auth/google`
- **Method:** `GET`
- **Access Level:** Public
- **বিবরণ:** ইউজারকে সরাসরি Google-এর সিকিউর সাইন-ইন পেজে ৩০২ রিডাইরেক্ট করে।

---

### 8.12 `GET /auth/google/callback`
- **Method:** `GET`
- **Access Level:** Public
- **Query Params:** `?code=...&state=...`
- **বিবরণ:** Google এর কলব্যাক প্রসেস করে নতুন অ্যাকাউন্ট খুলে অথবা ইউজারকে সরাসরি লগইন করিয়ে JWT টোকেন প্রদান করে।

---

### 8.13 `POST /auth/logout`
- **Method:** `POST`
- **Access Level:** Protected (`Authorization: Bearer <access_token>`)
- **বিবরণ:** ইউজারের বর্তমান সেশন বাতিল করে এবং Access Token কে মেমোরি/Redis ব্ল্যাকলিস্টে যুক্ত করে।

#### Response (`200 OK`):
```json
{
  "success": true,
  "message": "Logged out successfully. All sessions have been revoked."
}
```

---

### 8.14 `GET /auth/me`
- **Method:** `GET`
- **Access Level:** Protected (`Authorization: Bearer <access_token>`)
- **বিবরণ:** লগইন থাকা ইউজারের পূর্ণাঙ্গ প্রোফাইল ও ভেরিফিকেশন স্ট্যাটাস রিটার্ন করে।

#### Response (`200 OK`):
```json
{
  "success": true,
  "message": "Profile loaded successfully",
  "data": {
    "id": "f48dc608-8585-46f0-9fb5-dd20c8201af4",
    "email": "maksudurr538@gmail.com",
    "phone": "01880829496",
    "role": "CUSTOMER",
    "status": "ACTIVE",
    "email_verified": true,
    "phone_verified": true
  }
}
```

---

### 8.15 `GET /auth/sessions`
- **Method:** `GET`
- **Access Level:** Protected (`Authorization: Bearer <access_token>`)
- **বিবরণ:** ইউজারের অ্যাকাউন্টটি কোন কোন ডিভাইসে বা আইপি-তে লগইন আছে তার তালিকা দেখায়।

#### Response (`200 OK`):
```json
{
  "success": true,
  "message": "Active sessions retrieved",
  "data": {
    "user_id": "f48dc608-8585-46f0-9fb5-dd20c8201af4",
    "count": 1,
    "sessions": [
      {
        "id": "b6c29de4-c18b-41c5-bc32-dfba0e86e962",
        "ip_address": "127.0.0.1",
        "user_agent": "Mozilla/5.0",
        "created_at": "2026-09-24T10:04:11Z"
      }
    ]
  }
}
```

---

### 8.16 `DELETE /auth/sessions/:sessionId`
- **Method:** `DELETE`
- **Access Level:** Protected (`Authorization: Bearer <access_token>`)
- **বিবরণ:** সেশন আইডি প্রদান করে অন্য কোনো নির্দিষ্ট ডিভাইস থেকে ইউজারকে লগআউট করার সুবিধা।

#### Response (`200 OK`):
```json
{
  "success": true,
  "message": "Session revoked successfully."
}
```

---

### 💡 Next.js ডেভেলপারদের জন্য সিকিউরিটি টিপস:
1. **Protected Route Middleware:** ইউজারদের সুরক্ষিত পেজে (যেমন `/dashboard`) অটোমেটিক গার্ড দিতে `middleware.ts` ব্যবহার করুন।
2. **Brute-Force Lockout:** OTP ভেরিফিকেশনে পর পর ৫ বার ভুল ইনপুট দিলে ৪২৯ এরর আসবে।
3. **Auto Token Rotation:** `apiClient.ts` ব্যবহার করলে ইউজারের ১৫ মিনিটের টোকেন এক্সপায়ারি ক্লায়েন্টে টের পাওয়া যাবে না।
