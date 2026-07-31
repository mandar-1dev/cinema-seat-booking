package main

import (
	"log"
	"net/http"

	"github.com/sikozonpc/cinema/internal/adapters/redis"
	"github.com/sikozonpc/cinema/internal/booking"
	"github.com/sikozonpc/cinema/internal/utils"
)

func main() {
	mux := http.NewServeMux()

	redisClient := redis.NewClient("localhost:6379")
	store := booking.NewRedisStore(redisClient)
	svc := booking.NewService(store)
	h := booking.NewHandler(svc)

	mux.HandleFunc("GET /movies", listMovies)
	mux.HandleFunc("GET /movies/{movieID}/seats", h.ListSeats)
	mux.HandleFunc("POST /movies/{movieID}/seats/{seatID}/hold", h.HoldSeat)
	mux.HandleFunc("PUT /sessions/{sessionID}/confirm", h.ConfirmSession)
	mux.HandleFunc("DELETE /sessions/{sessionID}", h.ReleaseSession)

	mux.Handle("GET /", http.FileServer(http.Dir("static")))

	log.Println("server running at http://localhost:8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
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
