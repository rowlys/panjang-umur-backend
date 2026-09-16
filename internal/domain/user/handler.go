package user

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rowlys/panjang-umur-backend/internal/httputil"
)

type RegisterInput struct {
	Username string `json:"username" binding:"required"`
	Name     string `json:"name" binding:"required"`
	Password string `json:"password" binding:"required,min=6"`
}

type LoginInput struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type UserSearchDTO struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
	Name     string    `json:"name"`
	Status   int       `json:"status"` // Friendship status with the caller
}

type FriendshipService interface {
	GetBulkStatuses(callerID uuid.UUID, otherIDs []uuid.UUID) (map[uuid.UUID]int, error)
}

type Handler struct {
	service Service
	friends FriendshipService
}

func NewHandler(service Service, friends FriendshipService) *Handler {
	return &Handler{service: service, friends: friends}
}

func (h *Handler) RegisterAuthRoutes(rg *gin.RouterGroup) {
	rg.POST("/register", h.Register)
	rg.POST("/login", h.Login)
}

func (h *Handler) RegisterProtectedRoutes(rg *gin.RouterGroup) {
	// Static routes before wildcards
	rg.GET("/me", h.GetMe)
	rg.PATCH("/me", h.UpdateProfile)
	rg.PATCH("/me/password", h.ChangePassword)
	rg.GET("/username/:username", h.GetByUsername)
	rg.GET("/:userId", h.GetByID)

	rg.GET("/search", h.SearchByUsername)
}

// Register godoc
// @Summary      Register a new user
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        body body RegisterInput true "Registration data"
// @Success      201  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /auth/register [post]
func (h *Handler) Register(c *gin.Context) {
	var input RegisterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.service.Register(c.Request.Context(), input)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "User created successfully", "userId": user.ID})
}

// Login godoc
// @Summary      Login
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        body body LoginInput true "Login credentials"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]string
// @Failure      401  {object}  map[string]string
// @Router       /auth/login [post]
func (h *Handler) Login(c *gin.Context) {
	var input LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	loginResp, err := h.service.Login(c.Request.Context(), input)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, loginResp)
}

// GetMe godoc
// @Summary      Get the authenticated user's profile
// @Tags         Users
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  models.User
// @Failure      401  {object}  map[string]string
// @Router       /users/me [get]
func (h *Handler) GetMe(c *gin.Context) {
	userID, ok := httputil.ParseUserID(c)
	if !ok {
		return
	}

	user, err := h.service.GetByID(userID)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, user)
}

// GetByID godoc
// @Summary      Get user by ID
// @Tags         Users
// @Produce      json
// @Security     BearerAuth
// @Param        userId path string true "User ID (UUID)"
// @Success      200  {object}  models.User
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Router       /users/{userId} [get]
func (h *Handler) GetByID(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	user, err := h.service.GetByID(userID)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, user)
}

// UpdateProfile godoc
// @Summary      Update the authenticated user's profile
// @Tags         Users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body UpdateProfileInput true "Profile data"
// @Success      200  {object}  models.User
// @Failure      400  {object}  map[string]string
// @Failure      409  {object}  map[string]string
// @Router       /users/me [patch]
func (h *Handler) UpdateProfile(c *gin.Context) {
	userID, ok := httputil.ParseUserID(c)
	if !ok {
		return
	}

	var input UpdateProfileInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.service.UpdateProfile(c.Request.Context(), userID, input)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, user)
}

// ChangePassword godoc
// @Summary      Change the authenticated user's password
// @Tags         Users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body ChangePasswordInput true "Password data"
// @Success      200  {object}  map[string]string
// @Failure      400  {object}  map[string]string
// @Failure      401  {object}  map[string]string
// @Router       /users/me/password [patch]
func (h *Handler) ChangePassword(c *gin.Context) {
	userID, ok := httputil.ParseUserID(c)
	if !ok {
		return
	}

	var input ChangePasswordInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.ChangePassword(c.Request.Context(), userID, input); err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password updated successfully"})
}

// GetByUsername godoc
// @Summary      Get user by username
// @Tags         Users
// @Produce      json
// @Security     BearerAuth
// @Param        username path string true "Username"
// @Success      200  {object}  models.User
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Router       /users/username/{username} [get]
func (h *Handler) GetByUsername(c *gin.Context) {
	username := c.Param("username")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username is required"})
		return
	}

	user, err := h.service.GetByUsername(username)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, user)
}


// SearchByUsername godoc
// @Summary      Search users by username prefix
// @Tags         Users
// @Produce      json
// @Security     BearerAuth
// @Param        prefix  query     string  true  "Username prefix"
// @Param        limit   query     int     false "Limit the number of results (default is 10)"
// @Success      200  {array}   UserSearchDTO
// @Failure      400  {object}  map[string]string
// @Router	   /users/search [get]
func (h *Handler) SearchByUsername(c *gin.Context) {
	userID, ok := httputil.ParseUserID(c)
	if !ok {
		return
	}

	prefix, ok := c.GetQuery("prefix")
	if !ok || prefix == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Prefix query parameter is required"})
		return
	}
	
	limitStr := c.DefaultQuery("limit", "10")
    limit, err := strconv.Atoi(limitStr)
    if err != nil || limit <= 0 {
        limit = 10
    }

	users, err := h.service.SearchByUsername(userID, prefix, limit)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	var otherIDs []uuid.UUID
	for _, user := range users {
		if user.ID != userID {
			otherIDs = append(otherIDs, user.ID)
		}
	}

	statusMap, err := h.friends.GetBulkStatuses(userID, otherIDs)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	var response []UserSearchDTO
	for _, user := range users {
		if user.ID == userID {
			continue
		}

		status, exists := statusMap[user.ID]
		if !exists {
			status = 0 // No relationship
		} else {
			status += 1 
		}

		response = append(response, UserSearchDTO{
			ID:       user.ID,
			Username: user.Username,
			Name:     user.Name,
			Status:   status,
		})
	}

	if response == nil {
		response = []UserSearchDTO{}
	}

	c.JSON(http.StatusOK, response)

}