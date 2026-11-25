# ShopGo (Go + Gin + PostgreSQL + React)

## Run (Windows, two terminals)

```powershell
# terminal 1: API on :8080  (settings are read from backend\.env)
cd backend
go mod tidy
go run .

# terminal 2: website on :5173
cd frontend
npm install
npm run dev
```
Open http://localhost:5173. Admin: `admin@shopgo.local` / `admin12345`.
The first start adds the new database columns/tables automatically (AutoMigrate).

## Payments (Razorpay)

Without keys the shop runs in **DEMO payment mode** (free "Pay now" button).
With keys it uses the real Razorpay Checkout (UPI, cards, netbanking, wallets).

1. Create a free account at razorpay.com, switch the dashboard to **Test Mode**.
2. Account & Settings -> **API Keys** -> Generate Test Key.
3. Put them in `backend\.env`, then restart the backend:
   ```
   RAZORPAY_KEY_ID=rzp_test_xxxxxxxx
   RAZORPAY_KEY_SECRET=xxxxxxxxxxxxxxxx
   ```
   The log should say `Payments: Razorpay is ON`.
4. Place an order and pay with a Razorpay **test** method (test card 4111 1111 1111 1111 with any
   future expiry/CVV, or test UPI `success@razorpay`; see Razorpay's test-mode docs for the current list).

How it stays secure:
- The server creates the Razorpay order using the amount from the database (never from the browser).
- After payment the server re-computes Razorpay's HMAC signature with the secret key. Only a valid
  signature marks the order paid. The secret key never reaches the browser.
- With Razorpay on, the free `/pay` shortcut is disabled.

Optional webhook (marks orders paid even if the customer closes the tab right after paying):
Dashboard -> Webhooks -> URL `https://<public-url>/api/webhooks/razorpay`, event `payment.captured`,
set a secret and put it in `RAZORPAY_WEBHOOK_SECRET`. Locally you need a tunnel such as ngrok.

Going live: replace test keys with live keys (needs Razorpay KYC), use HTTPS, set a strong
`JWT_SECRET` and `ADMIN_PASSWORD`.

## What's new in this version
- Razorpay payments + payment records, webhook, signature verification (+ unit test)
- Shipping address at checkout (saved on the order, shown to the admin)
- New design: hero, category chips, product cards, skeleton loaders, toasts, order tracker
- Product detail page, "Buy now"
- Admin dashboard: revenue, orders by status, low-stock list
- Login/register rate limit (10 per minute per IP) and security headers

## Test
`cd backend && go test ./...`  (checks the payment signature code)

## Ideas for later
Email/SMS order updates, refunds via Razorpay API, coupons, product reviews & wishlist,
forgot-password flow, refresh tokens, GST invoices (PDF), search by multiple filters, deployment
(VPS or Render/Railway) with HTTPS.
