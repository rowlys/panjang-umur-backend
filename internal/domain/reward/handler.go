package reward

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rowlys/panjang-umur-backend/internal/httputil"
	"github.com/rowlys/panjang-umur-backend/internal/models"
)

type Handler struct {
	service Service
}

type CreateRewardRequest struct {
	Title          string                      `json:"title" binding:"required"`
	Description    string                      `json:"description"`
	Cost           int                         `json:"cost" binding:"required,gt=0"`
	Visibility     models.RewardVisibilityMode `json:"visibility"`
	AllowedUserIDs []uuid.UUID                 `json:"allowedUserIds"`
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	// Static routes before wildcard
	rg.GET("/me/given", h.GetByGiver)
	rg.GET("/shop/:giverId", h.GetShopByGiver)
	rg.POST("", h.Create)
	rg.PATCH("/:rewardId/redeem", h.Redeem)
	rg.PATCH("/:rewardId/cancel", h.Cancel)
}

// Create godoc
// @Summary      Create a reward in my shop
// @Tags         Rewards
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body     body      CreateRewardRequest  true  "Reward data"
// @Success      201      {object}  models.Reward
// @Failure      400      {object}  map[string]string
// @Failure      403      {object}  map[string]string
// @Failure      500      {object}  map[string]string
// @Router       /rewards [post]
func (h *Handler) Create(c *gin.Context) {
	userID, ok := httputil.ParseUserID(c)
	if !ok {
		return
	}

	var input CreateRewardRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	reward, err := h.service.Create(c.Request.Context(), CreateRewardInput{
		Title:          input.Title,
		Description:    input.Description,
		Cost:           input.Cost,
		Visibility:     input.Visibility,
		AllowedUserIDs: input.AllowedUserIDs,
		GiverID:        userID,
	})
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusCreated, reward)
}

// Redeem godoc
// @Summary      Redeem a reward using points
// @Tags         Rewards
// @Produce      json
// @Security     BearerAuth
// @Param        rewardId  path      string  true  "Reward ID (UUID)"
// @Success      200       {object}  models.Reward
// @Failure      400       {object}  map[string]string
// @Failure      403       {object}  map[string]string
// @Failure      404       {object}  map[string]string
// @Router       /rewards/{rewardId}/redeem [patch]
func (h *Handler) Redeem(c *gin.Context) {
	rewardID, err := uuid.Parse(c.Param("rewardId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid reward ID"})
		return
	}

	userID, ok := httputil.ParseUserID(c)
	if !ok {
		return
	}

	reward, err := h.service.Redeem(c.Request.Context(), rewardID, userID)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, reward)
}

// GetShopByGiver godoc
// @Summary      Browse a friend's shop (rewards visible to me: public + restricted-to-me)
// @Tags         Rewards
// @Produce      json
// @Security     BearerAuth
// @Param        giverId    path      string  true   "Reward giver's user ID (UUID)"
// @Param        available  query     boolean false  "Filter to available rewards only"
// @Success      200        {array}   models.Reward
// @Failure      400        {object}  map[string]string
// @Failure      403        {object}  map[string]string
// @Failure      500        {object}  map[string]string
// @Router       /rewards/shop/{giverId} [get]
func (h *Handler) GetShopByGiver(c *gin.Context) {
	giverID, err := uuid.Parse(c.Param("giverId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid giver ID"})
		return
	}

	userID, ok := httputil.ParseUserID(c)
	if !ok {
		return
	}

	availableOnly := c.Query("available") == "true"

	rewards, err := h.service.GetShopByGiver(userID, giverID, availableOnly)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, rewards)
}

// Cancel godoc
// @Summary      Cancel an available reward
// @Tags         Rewards
// @Produce      json
// @Security     BearerAuth
// @Param        rewardId  path      string  true  "Reward ID (UUID)"
// @Success      200       {object}  models.Reward
// @Failure      400       {object}  map[string]string
// @Failure      403       {object}  map[string]string
// @Failure      404       {object}  map[string]string
// @Router       /rewards/{rewardId}/cancel [patch]
func (h *Handler) Cancel(c *gin.Context) {
	rewardID, err := uuid.Parse(c.Param("rewardId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid reward ID"})
		return
	}

	userID, ok := httputil.ParseUserID(c)
	if !ok {
		return
	}

	reward, err := h.service.Cancel(c.Request.Context(), rewardID, userID)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, reward)
}

// GetByGiver godoc
// @Summary      List rewards I have created (my shop management view)
// @Tags         Rewards
// @Produce      json
// @Security     BearerAuth
// @Success      200  {array}   models.Reward
// @Failure      500  {object}  map[string]string
// @Router       /rewards/me/given [get]
func (h *Handler) GetByGiver(c *gin.Context) {
	userID, ok := httputil.ParseUserID(c)
	if !ok {
		return
	}

	rewards, err := h.service.GetByGiver(userID)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, rewards)
}
