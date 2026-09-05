package chat

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rowlys/panjang-umur-backend/internal/models"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
)

type connection struct {
	conn *websocket.Conn
	send chan []byte
}

func newConnection(conn *websocket.Conn) *connection {
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	return &connection{
		conn: conn,
		send: make(chan []byte, 256),
	}
}

type Hub struct {
	mu          sync.RWMutex
	connections map[uuid.UUID][]*connection
}

func NewHub() *Hub {
	return &Hub{
		connections: make(map[uuid.UUID][]*connection),
	}
}

type wsEvent struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

type readReceiptPayload struct {
	FriendID uuid.UUID `json:"friendId"`
	ReadAt   time.Time `json:"readAt"`
}

func (h *Hub) Register(userID uuid.UUID, conn *connection) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.connections[userID] = append(h.connections[userID], conn)
}

func (h *Hub) Unregister(userID uuid.UUID, conn *connection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	conns := h.connections[userID]
	for i, existing := range conns {
		if existing == conn {
			conns = append(conns[:i], conns[i+1:]...)
			break
		}
	}

	if len(conns) == 0 {
		delete(h.connections, userID)
	} else {
		h.connections[userID] = conns
	}

	close(conn.send)
}

func (h *Hub) PushMessage(userID uuid.UUID, message *models.Message) {
	h.push(userID, wsEvent{Type: "message", Data: message})
}

func (h *Hub) PushReadReceipt(userID, byUserID uuid.UUID, readAt time.Time) {
	h.push(userID, wsEvent{
		Type: "read",
		Data: readReceiptPayload{FriendID: byUserID, ReadAt: readAt},
	})
}

func (h *Hub) push(userID uuid.UUID, event wsEvent) {
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, conn := range h.connections[userID] {
		select {
		case conn.send <- payload:
		default:
		}
	}
}

func (h *Hub) writePump(conn *connection) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		conn.conn.Close()
	}()

	for {
		select {
		case payload, ok := <-conn.send:
			conn.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				conn.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := conn.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				return
			}

		case <-ticker.C:
			conn.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
