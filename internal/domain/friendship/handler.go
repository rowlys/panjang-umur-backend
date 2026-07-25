package friendship

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rowlys/panjang-umur-backend/internal/httputil"
)

type Handler struct {
	service Service
}

type SendRequestRequest struct {
	AddresseeID uuid.UUID `json:"addresseeId" binding:"required"`
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("", h.ListFriends)
	rg.GET("/requests", h.ListRequests)
	rg.POST("/requests", h.SendRequest)
	rg.POST("/requests/:id/accept", h.Accept)
	rg.POST("/requests/:id/decline", h.Decline)
	rg.DELETE("/:userId", h.Unfriend)
}

// SendRequest godoc
// @Summary      Send a friend request
// @Tags         Friends
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      SendRequestRequest  true  "Addressee"
// @Success      201   {object}  models.Friendship
// @Failure      400   {object}  map[string]string
// @Failure      409   {object}  map[string]string
// @Router       /friends/requests [post]
func (h *Handler) SendRequest(c *gin.Context) {
	var input SendRequestRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	callerID, ok := httputil.ParseUserID(c)
	if !ok {
		return
	}

	f, err := h.service.SendRequest(c.Request.Context(), callerID, input.AddresseeID)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusCreated, f)
}

// Accept godoc
// @Summary      Accept a friend request
// @Tags         Friends
// @Produce      json
// @Security     BearerAuth
// @Param        id  path      string  true  "Friend request ID (UUID)"
// @Success      200 {object}  models.Friendship
// @Failure      400 {object}  map[string]string
// @Failure      403 {object}  map[string]string
// @Failure      404 {object}  map[string]string
// @Router       /friends/requests/{id}/accept [post]
func (h *Handler) Accept(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request ID"})
		return
	}

	callerID, ok := httputil.ParseUserID(c)
	if !ok {
		return
	}

	f, err := h.service.Accept(c.Request.Context(), id, callerID)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, f)
}

// Decline godoc
// @Summary      Decline or cancel a friend request
// @Tags         Friends
// @Produce      json
// @Security     BearerAuth
// @Param        id  path      string  true  "Friend request ID (UUID)"
// @Success      200 {object}  map[string]string
// @Failure      400 {object}  map[string]string
// @Failure      403 {object}  map[string]string
// @Failure      404 {object}  map[string]string
// @Router       /friends/requests/{id}/decline [post]
func (h *Handler) Decline(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request ID"})
		return
	}

	callerID, ok := httputil.ParseUserID(c)
	if !ok {
		return
	}

	if err := h.service.Decline(c.Request.Context(), id, callerID); err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Friend request declined"})
}

// Unfriend godoc
// @Summary      Remove a friend
// @Tags         Friends
// @Produce      json
// @Security     BearerAuth
// @Param        userId  path      string  true  "Friend's user ID (UUID)"
// @Success      200     {object}  map[string]string
// @Failure      400     {object}  map[string]string
// @Failure      404     {object}  map[string]string
// @Router       /friends/{userId} [delete]
func (h *Handler) Unfriend(c *gin.Context) {
	otherID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	callerID, ok := httputil.ParseUserID(c)
	if !ok {
		return
	}

	if err := h.service.Unfriend(c.Request.Context(), callerID, otherID); err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Friend removed"})
}

// ListFriends godoc
// @Summary      List my friends
// @Tags         Friends
// @Produce      json
// @Security     BearerAuth
// @Success      200  {array}   models.User
// @Failure      500  {object}  map[string]string
// @Router       /friends [get]
func (h *Handler) ListFriends(c *gin.Context) {
	callerID, ok := httputil.ParseUserID(c)
	if !ok {
		return
	}

	friends, err := h.service.ListFriends(callerID)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, friends)
}

// ListRequests godoc
// @Summary      List my incoming and outgoing friend requests
// @Tags         Friends
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  map[string]interface{}
// @Failure      500  {object}  map[string]string
// @Router       /friends/requests [get]
func (h *Handler) ListRequests(c *gin.Context) {
	callerID, ok := httputil.ParseUserID(c)
	if !ok {
		return
	}

	incoming, err := h.service.ListIncoming(callerID)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	outgoing, err := h.service.ListOutgoing(callerID)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, gin.H{"incoming": incoming, "outgoing": outgoing})
}
