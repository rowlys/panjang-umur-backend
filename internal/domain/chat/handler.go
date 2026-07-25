package chat

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/rowlys/panjang-umur-backend/internal/httputil"
)

type Handler struct {
	service  Service
	hub      *Hub
	upgrader websocket.Upgrader
}

type wsInboundMessage struct {
	RecipientID uuid.UUID `json:"recipientId"`
	Body        string    `json:"body"`
}

type wsErrorFrame struct {
	Error string `json:"error"`
}

func NewHandler(service Service, hub *Hub) *Handler {
	return &Handler{
		service: service,
		hub:     hub,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/:friendId/messages", h.GetMessages)
	rg.POST("/:friendId/read", h.MarkAsRead)
}

// GetMessages godoc
// @Summary      Get message history with a friend
// @Tags         Chat
// @Produce      json
// @Security     BearerAuth
// @Param        friendId  path      string  true   "Friend's user ID (UUID)"
// @Param        before    query     string  false  "Only return messages older than this RFC3339 timestamp (pagination cursor)"
// @Param        limit     query     int     false  "Max messages to return (default 50, capped at 100)"
// @Success      200       {array}   models.Message
// @Failure      400       {object}  map[string]string
// @Router       /chat/{friendId}/messages [get]
func (h *Handler) GetMessages(c *gin.Context) {
	friendID, err := uuid.Parse(c.Param("friendId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid friend ID"})
		return
	}

	callerID, ok := httputil.ParseUserID(c)
	if !ok {
		return
	}

	var before *time.Time
	if raw := c.Query("before"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid 'before' timestamp, expected RFC3339"})
			return
		}
		before = &parsed
	}

	limit := 0
	if raw := c.Query("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid 'limit', expected an integer"})
			return
		}
		limit = parsed
	}

	messages, err := h.service.GetConversation(c.Request.Context(), callerID, friendID, before, limit)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, messages)
}

// MarkAsRead godoc
// @Summary      Mark every message sent by a friend to the caller as read
// @Tags         Chat
// @Produce      json
// @Security     BearerAuth
// @Param        friendId  path      string  true  "Friend's user ID (UUID)"
// @Success      200       {object}  map[string]string
// @Failure      400       {object}  map[string]string
// @Router       /chat/{friendId}/read [post]
func (h *Handler) MarkAsRead(c *gin.Context) {
	friendID, err := uuid.Parse(c.Param("friendId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid friend ID"})
		return
	}

	callerID, ok := httputil.ParseUserID(c)
	if !ok {
		return
	}

	if err := h.service.MarkAsRead(c.Request.Context(), callerID, friendID); err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Messages marked as read"})
}

func (h *Handler) ServeWS(c *gin.Context) {
	userID, ok := httputil.ParseUserID(c)
	if !ok {
		return
	}

	wsConn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	conn := newConnection(wsConn)
	h.hub.Register(userID, conn)

	go h.hub.writePump(conn)

	h.readPump(c.Request.Context(), userID, conn)
}

func (h *Handler) readPump(ctx context.Context, userID uuid.UUID, conn *connection) {
	defer h.hub.Unregister(userID, conn)

	for {
		_, raw, err := conn.conn.ReadMessage()
		if err != nil {
			return
		}

		var in wsInboundMessage
		if err := json.Unmarshal(raw, &in); err != nil {
			h.sendError(conn, "Invalid message format, expected {\"recipientId\":\"...\",\"body\":\"...\"}")
			continue
		}

		if _, err := h.service.SendMessage(ctx, userID, in.RecipientID, in.Body); err != nil {
			_, msg := httputil.ResolveServiceError(err)
			h.sendError(conn, msg)
		}
	}
}

func (h *Handler) sendError(conn *connection, message string) {
	payload, err := json.Marshal(wsErrorFrame{Error: message})
	if err != nil {
		return
	}

	select {
	case conn.send <- payload:
	default:
	}
}
