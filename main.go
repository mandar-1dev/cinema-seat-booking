package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sikozonpc/cinema/internal/adapters/mongodb"
	"github.com/sikozonpc/cinema/internal/adapters/redis"
	"github.com/sikozonpc/cinema/internal/auth"
	"github.com/sikozonpc/cinema/internal/booking"
	"github.com/sikozonpc/cinema/internal/catalog"
	"github.com/sikozonpc/cinema/internal/config"
	"github.com/sikozonpc/cinema/internal/demo"
	"github.com/sikozonpc/cinema/internal/payment"
	"github.com/sikozonpc/cinema/internal/realtime"
	"github.com/sikozonpc/cinema/internal/ticket"
	"github.com/sikozonpc/cinema/internal/utils"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()

	redisClient := redis.NewClient(cfg.RedisURL)
	store := booking.NewRedisStore(redisClient)
	svc := booking.NewService(store)
	h := booking.NewHandler(svc)
	realtimeHub := realtime.NewHub()
	h.SetRealtimeHub(realtimeHub)
	mux.HandleFunc("GET /ws/seats/{movieID}", realtimeHub.ServeWS)

	if _, err := os.Stat("frontend/dist"); err == nil {
		mux.Handle("GET /", http.FileServer(http.Dir("frontend/dist")))
	} else {
		mux.Handle("GET /", http.FileServer(http.Dir("static")))
	}

	if cfg.MongoDBURI != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		mongoClient, mongoErr := mongodb.Connect(ctx, cfg.MongoDBURI, cfg.MongoDBDatabase)
		cancel()
		if mongoErr != nil {
			log.Printf("MongoDB unavailable: %v", mongoErr)
		} else {
			defer mongoClient.Close(context.Background())
			log.Printf("MongoDB connected to database %q", cfg.MongoDBDatabase)

			bookingRepo := mongodb.NewBookingRepository(mongoClient.Database)
			indexCtx, indexCancel := context.WithTimeout(context.Background(), 10*time.Second)
			if err := bookingRepo.EnsureIndexes(indexCtx); err != nil {
				log.Printf("could not ensure booking indexes: %v", err)
			}
			indexCancel()
			h = booking.NewHandler(svc, bookingRepo)
			h.SetRealtimeHub(realtimeHub)

			if cfg.JWTSecret == "" {
				log.Printf("JWT authentication disabled; set JWT_SECRET")
			} else {
				userRepo := auth.NewMongoUserRepository(mongoClient.Database)
				indexCtx, indexCancel := context.WithTimeout(context.Background(), 10*time.Second)
				if err := userRepo.EnsureIndexes(indexCtx); err != nil {
					log.Printf("could not ensure user indexes: %v", err)
				}
				indexCancel()

				authService := auth.NewService(userRepo, cfg.JWTSecret)
				authHandler := auth.NewHandler(authService)
				requireAuth := auth.RequireAuth(authService)
				mux.HandleFunc("POST /auth/register", authHandler.Register)
				mux.HandleFunc("POST /auth/login", authHandler.Login)
				mux.HandleFunc("POST /auth/refresh", authHandler.Refresh)
				mux.Handle("GET /auth/me", requireAuth(http.HandlerFunc(authHandler.Me)))
				mux.Handle("PUT /auth/me", requireAuth(http.HandlerFunc(authHandler.UpdateProfile)))
				mux.Handle("GET /bookings", requireAuth(http.HandlerFunc(h.ListMyBookings)))

				paymentRepo := payment.NewMongoRepository(mongoClient.Database)
				paymentIndexCtx, paymentIndexCancel := context.WithTimeout(context.Background(), 10*time.Second)
				if err := paymentRepo.EnsureIndexes(paymentIndexCtx); err != nil {
					log.Printf("could not ensure payment indexes: %v", err)
				}
				paymentIndexCancel()
				paymentHandler := payment.NewHandler(cfg.RazorpayKeyID, cfg.RazorpayKeySecret, paymentRepo)
				ticketRepo := ticket.NewMongoRepository(mongoClient.Database)
				ticketIndexCtx, ticketIndexCancel := context.WithTimeout(context.Background(), 10*time.Second)
				if err := ticketRepo.EnsureIndexes(ticketIndexCtx); err != nil {
					log.Printf("could not ensure ticket indexes: %v", err)
				}
				ticketIndexCancel()
				ticketService := ticket.NewService(ticketRepo, cfg.JWTSecret)
				ticketHandler := ticket.NewHandler(ticketService)
				demoRepo := demo.NewMongoRepository(mongoClient.Database)
				demoIndexCtx, demoIndexCancel := context.WithTimeout(context.Background(), 10*time.Second)
				if err := demoRepo.EnsureIndexes(demoIndexCtx); err != nil {
					log.Printf("could not ensure demo checkout indexes: %v", err)
				}
				demoIndexCancel()
				demoService := demo.NewService(svc, bookingRepo, demoRepo, cfg.JWTSecret)
				demoService.SetConfirmedCallback(func(confirmed booking.Booking) {
					realtimeHub.Broadcast(realtime.Event{
						Type: "seat_confirmed", MovieID: confirmed.MovieID, SeatID: confirmed.SeatID, Status: "booked",
					})
				})
				demoHandler := demo.NewHandler(demoService)

				paymentHandler.SetCapturedCallback(func(ctx context.Context, req payment.VerificationRequest) (string, error) {
					if req.SessionID == "" {
						return "", booking.ErrSessionNotFound
					}
					claims := auth.ClaimsFromContext(ctx)
					if claims == nil {
						return "", auth.ErrInvalidToken
					}
					confirmed, err := svc.ConfirmSeat(ctx, req.SessionID, claims.UserID)
					if err != nil {
						return "", err
					}
					if err := bookingRepo.Save(ctx, confirmed); err != nil {
						return "", err
					}
					issued, err := ticketService.Issue(ctx, confirmed.ID, confirmed.UserID, confirmed.MovieID, confirmed.SeatID)
					if err != nil {
						return "", err
					}
					realtimeHub.Broadcast(realtime.Event{
						Type: "seat_confirmed", MovieID: confirmed.MovieID, SeatID: confirmed.SeatID, Status: "booked",
					})
					return issued.ID, nil
				})
				mux.Handle("POST /payments/orders", requireAuth(http.HandlerFunc(paymentHandler.CreateOrder)))
				mux.Handle("POST /payments/verify", requireAuth(http.HandlerFunc(paymentHandler.Verify)))
				mux.HandleFunc("POST /payments/webhooks/razorpay", paymentHandler.RazorpayWebhook)
				mux.Handle("GET /tickets/{ticketID}/qr", requireAuth(http.HandlerFunc(ticketHandler.DownloadQR)))
				mux.Handle("POST /tickets/verify", auth.RequireRole(authService, "admin")(http.HandlerFunc(ticketHandler.Verify)))
				mux.Handle("POST /tickets/use", auth.RequireRole(authService, "admin")(http.HandlerFunc(ticketHandler.Use)))
				mux.HandleFunc("POST /demo/checkout", demoHandler.Checkout)
				mux.HandleFunc("GET /demo/tickets/{ticketID}/qr", demoHandler.DownloadQR)
				mux.HandleFunc("POST /demo/tickets/verify", demoHandler.Verify)
			}

			catalogRepo := catalog.NewMongoRepository(mongoClient.Database)
			indexCtx, indexCancel = context.WithTimeout(context.Background(), 10*time.Second)
			if err := catalogRepo.EnsureIndexes(indexCtx); err != nil {
				log.Printf("could not ensure catalog indexes: %v", err)
			}
			indexCancel()
			catalogHandler := catalog.NewHandler(catalogRepo)
			mux.HandleFunc("GET /catalog/movies", catalogHandler.ListMovies)
			mux.HandleFunc("GET /catalog/theatres", catalogHandler.ListTheatres)
			mux.HandleFunc("GET /catalog/shows", catalogHandler.ListShows)
		}
	} else {
		log.Printf("MongoDB not configured; set MONGODB_URI and MONGODB_DATABASE")
	}

	mux.HandleFunc("GET /movies", listMovies)
	mux.HandleFunc("GET /movies/{movieID}/seats", h.ListSeats)
	mux.HandleFunc("POST /movies/{movieID}/seats/{seatID}/hold", h.HoldSeat)
	mux.HandleFunc("PUT /sessions/{sessionID}/confirm", h.ConfirmSession)
	mux.HandleFunc("DELETE /sessions/{sessionID}", h.ReleaseSession)

	server := &http.Server{
		Addr:    ":" + cfg.HTTPPort,
		Handler: mux,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("server shutdown: %v", err)
		}
	}()

	log.Printf("server running at http://localhost:%s", cfg.HTTPPort)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

var movies = []movieResponse{
	{ID: "inception", Title: "Inception", Rows: 5, SeatsPerRow: 8},
	{ID: "dune", Title: "Dune: Part Two", Rows: 4, SeatsPerRow: 6},
}

func listMovies(w http.ResponseWriter, r *http.Request) {
	utils.WriteJSON(w, http.StatusOK, movies)
}

type movieResponse struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Rows        int    `json:"rows"`
	SeatsPerRow int    `json:"seats_per_row"`
}
