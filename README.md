# ShopGo

A small online store with **two screens**: a shop for customers, and an admin panel for the person who runs the store. Both talk to the same Go API and the same PostgreSQL database.

| Screen | Who | URL | What it is for |
| --- | --- | --- | --- |
| Store | Anyone, then a logged-in customer | http://localhost:5173 | Browse, cart, checkout, pay, track orders |
| Admin panel | The seeded admin account only | http://localhost:5173/admin | Catalog, stock, revenue, and fulfillment |

Customers cannot open `/admin`. The admin can still use the store (there is an **Open store** link in the admin bar) and also sees an **Admin panel** button in the shop nav.

## Store (customer)

Open http://localhost:5173. No account is required to look around.

- Home: search, category chips, sort, in-stock filter, product cards
- Product page: details, stock, add to cart, buy now
- Sign up / login
- Cart and checkout with a shipping address
- Pay (demo button, or Razorpay when keys are set)
- **My orders**: status tracker from awaiting payment → paid → shipped → delivered, plus cancel

A new signup is always a customer. There is no public way to become an admin.

## Admin panel

Log in as the seeded admin, then open **Admin panel**. The bar turns dark and the page gets its own sidebar so it does not look like the shop.

| Sidebar | What you can do |
| --- | --- |
| Dashboard | Revenue from paid orders, order counts by status, customer count, products with 5 or fewer left |
| Catalog | Add categories, create / edit / delete products, set price and stock, upload a photo (jpg, png, gif, webp, max 5MB) |
| Orders | Every customer's order and address. Move status: pending → paid or cancelled, paid → shipped or cancelled, shipped → delivered |

Prices in the database are stored in **paise** (₹4499.00 is `449900`). The admin form takes rupees and converts them.

## Run

You need **Go**, **Node.js**, and **PostgreSQL** listening on `localhost:5432`.

```powershell
# copy once, then edit the database password if it is not "postgres"
copy backend\.env.example backend\.env
```

`backend\.env` is already read on startup. Default admin is created the first time the API starts.

```powershell
# terminal 1 — API on :8080
cd backend
go mod tidy
go run .

# terminal 2 — website on :5173
cd frontend
npm install
npm run dev
```

| Account | Email | Password |
| --- | --- | --- |
| Admin | `admin@shopgo.local` | `admin12345` |
| Customer | whatever you register | the password you choose |

Change `ADMIN_EMAIL`, `ADMIN_PASSWORD`, and `JWT_SECRET` in `backend\.env` before you share the app. The admin row is only inserted if that email does not exist yet.

The first start runs AutoMigrate and, if the catalog is empty, seeds four categories and nine sample products (one of them is out of stock so the store filter is easy to see).

## Payments

Leave the Razorpay keys empty and checkout uses a free **Pay now** demo button. The log line is `Payments: DEMO mode`.

To use Razorpay test checkout:

1. Create an account at razorpay.com and switch the dashboard to **Test Mode**.
2. Account & Settings → **API Keys** → generate a test key.
3. Put the keys in `backend\.env` and restart the API:

```
RAZORPAY_KEY_ID=rzp_test_xxxxxxxx
RAZORPAY_KEY_SECRET=xxxxxxxxxxxxxxxx
```

The log should say `Payments: Razorpay is ON`. The demo `/pay` shortcut is then turned off. The server creates the Razorpay order from the amount stored in the database, then checks Razorpay's HMAC signature before it marks the order paid. The secret never goes to the browser.

Test card `4111 1111 1111 1111` (any future expiry and CVV) or test UPI `success@razorpay`. Confirm the current list in Razorpay's test-mode docs.

Optional webhook, so an order is still marked paid if the customer closes the tab: Dashboard → Webhooks → `https://<public-url>/api/webhooks/razorpay`, event `payment.captured`, then set `RAZORPAY_WEBHOOK_SECRET`. Locally that needs a tunnel such as ngrok.

## Layout

```
backend/     Gin API, GORM, JWT, Razorpay
  main.go            routes
  handlers/          auth, cart, catalog, orders, payments, stats
  middleware/        JWT auth and login rate limit
frontend/    React + Vite
  src/pages/Shop.jsx Product.jsx Cart.jsx Orders.jsx   customer screens
  src/pages/Admin.jsx                                   admin screen
  src/pages/AuthForm.jsx
```

## API

Public:

| Method | Path | |
| --- | --- | --- |
| GET | `/api/health` | liveness |
| POST | `/api/auth/register` | create a customer |
| POST | `/api/auth/login` | returns a JWT |
| GET | `/api/products` | search, category, sort, stock, page |
| GET | `/api/products/:id` | |
| GET | `/api/categories` | |
| POST | `/api/webhooks/razorpay` | signature-checked |

Customer (header `Authorization: Bearer <token>`):

| Method | Path |
| --- | --- |
| GET | `/api/auth/me` |
| GET, POST, PUT, DELETE | `/api/cart` and `/api/cart/items` |
| POST | `/api/orders` checkout |
| GET | `/api/orders`, `/api/orders/:id` |
| POST | `/api/orders/:id/pay` demo payment only |
| POST | `/api/orders/:id/payment` and `/payment/verify` |
| POST | `/api/orders/:id/cancel` |

Admin (same header, and the user role must be `admin`):

| Method | Path |
| --- | --- |
| GET | `/api/admin/stats` |
| POST, PUT, DELETE | `/api/admin/products` and `/api/admin/categories` |
| POST | `/api/admin/products/:id/image` |
| GET | `/api/admin/orders` |
| PUT | `/api/admin/orders/:id/status` |

Login and register are limited to 10 requests per minute per IP.

## Where to add AI

The useful places are the two screens you already have. Keep the model behind the Go API so the browser never holds an API key, and keep money, stock, and order status in the existing handlers. The model should suggest; those handlers should still decide.

**Store**

1. **Search box on the home page.** People type "headphones under 2000" instead of using the category chip and the sort menu. A small model (or even keyword extraction) can turn that sentence into the query params `/api/products` already accepts: `q`, `category_id`, and a price range if you add one. This is the highest-value, lowest-risk addition.
2. **Product page, "similar items".** Embed product name + description + category, store the vectors in Postgres (`pgvector`) or a small side table, and show 4 neighbors under the product. No checkout logic changes.
3. **Order helper on My orders.** A chat panel that can only call the signed-in user's `/api/orders`. It answers "where is order 12?" from the status you already store. Do not let it cancel or pay by itself.

**Admin panel**

4. **Catalog form, "Write description".** Next to the description field, a button that sends the product name and category to a model and fills the textarea. The admin still presses Create. Same idea for a short SEO blurb later.
5. **Photo upload.** After the image is saved, ask a vision model for a category suggestion and alt text. Show it as a hint on the product form. Do not auto-move the product into a category.
6. **Dashboard summary.** `/api/admin/stats` already returns revenue, status counts, and low stock. A paragraph at the top ("3 products are at 0, revenue is ₹…") is a single prompt over that JSON. Good first admin feature because the data is already there.

Skip AI for price, stock updates, payment verification, and status changes. Those stay in the current handlers.

A practical order: (6) dashboard summary, then (4) description helper, then (1) natural-language search.

## Test

```powershell
cd backend
go test ./...
```

That checks the Razorpay signature code.
