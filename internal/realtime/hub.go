package realtime

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

type Event struct {
	Type    string `json:"type"`
	MovieID string `json:"movie_id"`
	SeatID  string `json:"seat_id,omitempty"`
	Status  string `json:"status,omitempty"`
}

type client struct {
	movieID string
	conn    *websocket.Conn
}

type Hub struct {
	mu       sync.RWMutex
	clients  map[*client]struct{}
	upgrader websocket.Upgrader
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[*client]struct{}),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	movieID := r.PathValue("movieID")
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c := &client{movieID: movieID, conn: conn}
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			h.mu.Lock()
			delete(h.clients, c)
			h.mu.Unlock()
			_ = conn.Close()
			return
		}
	}
}

func (h *Hub) Broadcast(event Event) {
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		if event.MovieID != "" && c.movieID != event.MovieID {
			continue
		}
		if err := c.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
			_ = c.conn.Close()
		}
	}
}
