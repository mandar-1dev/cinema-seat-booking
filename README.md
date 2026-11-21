# Cinema — Movie Theatre Seat Booking

A seat booking backend built around a **hold → confirm → release** flow,
backed by Redis for atomic, race-free seat claims. Go standard library
`net/http` (Go 1.22+ routing), no framework.

## Why hold/confirm/release

Real booking systems don't book a seat in one step — they reserve it
temporarily while the user enters payment details, then finalize or cancel.
This project models that:

1. **Hold** — `POST /movies/{movieID}/seats/{seatID}/hold` atomically
   claims a seat for 2 minutes (`SETNX` in Redis — only one caller can ever
   win a given seat, even under heavy concurrent load).
2. **Confirm** — `PUT /sessions/{sessionID}/confirm` finalizes the hold
   into a permanent booking (removes its TTL).
3. **Release** — `DELETE /sessions/{sessionID}` cancels a hold immediately.

If nobody confirms within 2 minutes, Redis expires the key automatically —
no cleanup job needed.

## Project layout

```
cinema/
├── main.go                        # route wiring, movie catalog
├── static/index.html              # frontend (fetch-based, no build step)
├── docker-compose.yaml            # redis + redis-commander (GUI at :8081)
└── internal/
    ├── adapters/redis/client.go   # go-redis client, isolated behind this package
    ├── utils/json.go              # WriteJSON / WriteError helpers
    └── booking/
        ├── domain.go              # Booking type, BookingStore interface, errors
        ├── redis_store.go         # production store (Redis, atomic, TTL-based)
        ├── memory_store.go        # single-threaded store, for simple unit tests
        ├── concurrent_store.go    # mutex-guarded store, for concurrency tests without Redis
        ├── service.go             # thin layer between handlers and the store
        ├── handler.go             # HTTP handlers
        └── service_test.go        # proves exactly one booking wins under 100k concurrent holds
```

## Prerequisites

- Go 1.22+ (uses `r.PathValue` / method-based `mux.HandleFunc` routing)
- Docker + Docker Compose (for Redis)

## Setup

Copy `.env.example` to `.env` and configure it for your environment. MongoDB
Atlas and Razorpay credentials are optional during local infrastructure setup,
but all persistent and payment features require them.

```bash
# 1. Start Redis (and Redis Commander GUI at http://localhost:8081)
docker compose up -d

# 2. Download dependencies and verify go.sum
go mod tidy

# 3. Run the server
go run .
```

Open **http://localhost:8080** for the UI.

For the React development UI:

```bash
cd frontend
npm install
npm run dev
```

The Vite server runs at `http://localhost:5173` and proxies Go API routes to
`http://localhost:8080`.

## Running tests

```bash
# Requires Redis running (docker compose up -d) — this is an integration
# test that hits real Redis to prove atomic booking behavior.
go test ./... -run TestConcurrentBooking -v
```

## API

| Method | Path                                       | Body                  | Description               |
|--------|---------------------------------------------|-----------------------|----------------------------|
| GET    | `/movies`                                  | —                      | List available movies      |
| GET    | `/movies/{movieID}/seats`                  | —                      | List held/booked seats     |
| POST   | `/movies/{movieID}/seats/{seatID}/hold`    | `{"user_id": "..."}`  | Hold a seat (2 min TTL)    |
| PUT    | `/sessions/{sessionID}/confirm`            | `{"user_id": "..."}`  | Confirm a held seat        |
| DELETE | `/sessions/{sessionID}`                    | `{"user_id": "..."}`  | Release a held seat        |

When MongoDB and JWT are configured:

| Method | Path | Body | Description |
|--------|------|------|-------------|
| POST | `/auth/register` | Registration JSON | Create a customer account |
| POST | `/auth/login` | Email/password JSON | Issue access and refresh JWTs |
| POST | `/auth/refresh` | `{"refresh_token":"..."}` | Refresh an access token |
| GET | `/auth/me` | — | Get the authenticated user profile |
| PUT | `/auth/me` | Profile JSON | Update the authenticated profile |
| GET | `/catalog/movies` | — | List persisted movies |
| GET | `/catalog/theatres?city=` | — | List theatres, optionally filtered by city |
| GET | `/catalog/shows?movie_id=&theatre_id=&date=` | — | List persisted shows |

Payment routes, when Razorpay is configured:

| Method | Path | Authentication | Description |
|--------|------|----------------|-------------|
| POST | `/payments/orders` | Bearer access token | Create a real Razorpay test order |
| POST | `/payments/verify` | Bearer access token | Verify the Razorpay payment signature |
| POST | `/payments/webhooks/razorpay` | Razorpay signature | Receive provider webhook events |

The checkout only confirms a booking after Razorpay signature verification; it
does not report an unavailable or failed payment as successful. After successful
verification, customers can download a compact PDF ticket containing the
ticket QR and booking details. The current demo UI's showtime and cinema are
static placeholders until a real show is selected from the persisted catalog.

For Razorpay sandbox checkout, create a Test Mode key pair in the Razorpay
Dashboard and set `RAZORPAY_KEY_ID` and `RAZORPAY_KEY_SECRET` in the local
`.env` file. Use the `rzp_test_` key ID and restart the Go server. Do not paste
the secret into source files or commit `.env`. A valid entry QR is issued only
after the server verifies the payment signature and confirms the captured
payment with Razorpay.

### Demo checkout

Select **Demo mode** beside Sign in, enter a billing name, choose a seat, and
complete the clearly labeled simulated checkout. It makes no charge and does
not require Razorpay. Demo payment records and passes are stored separately in
MongoDB's `demo_payments` and `demo_passes` collections. Prices are calculated
by the server from the demo menu.

Demo QR codes are signed and can be checked with `POST /demo/tickets/verify`
using `{"ticket_id":"..."}`. The response explicitly marks them as demo-only
and `valid_for_entry: false`. They are isolated from the real `/tickets/verify`
and `/tickets/use` endpoints, and the downloadable PDF is marked **NOT VALID
FOR ENTRY**. This mode is for exercising the checkout UI only; it does not
create a paid or real admission ticket.

Real-time seat updates are available through:
`ws://localhost:8080/ws/seats/{movieID}`.

`user_id` must match the user who created the session, or `Confirm`/`Release`
return `403 Forbidden` — this is what stops someone else from cancelling or
confirming your booking if they learn your session ID.

## Design notes

- **Why TTL instead of an explicit status?** A held seat's Redis key has a
  TTL; a confirmed one doesn't (`PERSIST`); an available seat's key simply
  doesn't exist. Letting Redis's own expiry mechanism represent "hold
  timed out" means abandoned holds clean up for free.
- **Why is `BookingStore` an interface?** So `RedisStore` can be swapped
  for `MemoryStore`/`ConcurentStore` in tests without touching `Service`
  or the handlers.
