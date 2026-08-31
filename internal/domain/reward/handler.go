package reward

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rowlys/panjang-umur-backend/internal/httputil"
	"github.com/rowlys/panjang-umur-backend/internal/models"
)

type Handler struct {
	service Service
}

type RewardClaimHistoryResponse struct {
	ID 		  	  uuid.UUID          `json:"id"`
	RewardID      uuid.UUID          `json:"rewardId"`
	RedeemerID    uuid.UUID          `json:"redeemerId"`
	GiverID       uuid.UUID          `json:"giverId"`
	Price         int                `json:"price"`
	Status        models.ClaimStatus `json:"status"`
	RedeemedAt    time.Time          `json:"redeemedAt"`
	FulfilledAt   *time.Time         `json:"fulfilledAt"`
	ResolvedAt    *time.Time         `json:"resolvedAt"`
	GiverUsername string             `json:"giverUsername"`
}

type RewardClaimGivenResponse struct {
	ID 		  	  	 uuid.UUID          `json:"id"`
	RewardID      	 uuid.UUID          `json:"rewardId"`
	RedeemerID    	 uuid.UUID          `json:"redeemerId"`
	GiverID       	 uuid.UUID          `json:"giverId"`
	Price         	 int                `json:"price"`
	Status        	 models.ClaimStatus `json:"status"`
	RedeemedAt    	 time.Time          `json:"redeemedAt"`
	FulfilledAt   	 *time.Time         `json:"fulfilledAt"`
	ResolvedAt    	 *time.Time         `json:"resolvedAt"`
	RedeemerUsername string             `json:"redeemerUsername"`
}

type CreateRewardRequest struct {
	Title          string                      `json:"title" binding:"required"`
	Description    string                      `json:"description"`
	Cost           int                         `json:"cost" binding:"required,gt=0"`
	Visibility     models.RewardVisibilityMode `json:"visibility"`
	Stock          int                         `json:"stock" binding:"required,gt=0"`
	AllowedUserIDs []uuid.UUID                 `json:"allowedUserIds"`
}

type UpdateRewardStockRequest struct {
	Stock int `json:"stock" binding:"gte=0"`
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	// Static routes before wildcard
	rg.POST("", h.Create)
	rg.GET("/shop/:giverId", h.GetShopByGiver)
	rg.GET("/shop/me", h.GetMyShop)
	rg.GET("/claims/me", h.GetClaimHistory)
	rg.GET("/claims/given", h.GetClaimsGiven)
	rg.PATCH("/claims/:claimId/fulfill", h.FulfillClaim)
	rg.PATCH("/claims/:claimId/refund/request", h.RequestRefund)
	rg.PATCH("/claims/:claimId/refund/approve", h.ApproveRefund)

	rg.PATCH("/:rewardId/redeem", h.Redeem)
	rg.PATCH("/:rewardId/stock", h.UpdateStock)
	rg.GET("/:rewardId/claims", h.GetClaimsForReward)
	rg.GET("/:rewardId", h.GetByID)
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
		Stock:          input.Stock,
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

// GetMyShop godoc
// @Summary      Browse my own shop (rewards visible to me: public + restricted-to-me)
// @Tags         Rewards
// @Produce      json
// @Security     BearerAuth
// @Param        available  query     boolean false  "Filter to available rewards only"
// @Success      200        {array}   models.Reward
// @Failure      400        {object}  map[string]string
// @Failure      403        {object}  map[string]string
// @Failure      500        {object}  map[string]string
// @Router       /rewards/shop/me [get]
func (h *Handler) GetMyShop(c *gin.Context) {
	userID, ok := httputil.ParseUserID(c)
	if !ok {
		return
	}

	availableOnly := c.Query("available") == "true"

	rewards, err := h.service.GetShopByGiver(userID, userID, availableOnly)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, rewards)
}

// GetByID godoc
// @Summary      Get a reward I own by ID
// @Tags         Rewards
// @Produce      json
// @Security     BearerAuth
// @Param        rewardId  path      string  true  "Reward ID (UUID)"
// @Success      200       {object}  models.Reward
// @Failure      400       {object}  map[string]string
// @Failure      403       {object}  map[string]string
// @Failure      404       {object}  map[string]string
// @Router       /rewards/{rewardId} [get]
func (h *Handler) GetByID(c *gin.Context) {
	rewardID, err := uuid.Parse(c.Param("rewardId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid reward ID"})
		return
	}

	userID, ok := httputil.ParseUserID(c)
	if !ok {
		return
	}

	reward, err := h.service.GetByID(rewardID, userID)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, reward)
}

// UpdateStock godoc
// @Summary      Update a reward's stock (also flips availability accordingly)
// @Tags         Rewards
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        rewardId  path      string                     true  "Reward ID (UUID)"
// @Param        body      body      UpdateRewardStockRequest   true  "New stock amount"
// @Success      200       {object}  models.Reward
// @Failure      400       {object}  map[string]string
// @Failure      403       {object}  map[string]string
// @Failure      404       {object}  map[string]string
// @Router       /rewards/{rewardId}/stock [patch]
func (h *Handler) UpdateStock(c *gin.Context) {
	rewardID, err := uuid.Parse(c.Param("rewardId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid reward ID"})
		return
	}

	userID, ok := httputil.ParseUserID(c)
	if !ok {
		return
	}

	var input UpdateRewardStockRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	reward, err := h.service.UpdateStock(c.Request.Context(), rewardID, userID, input.Stock)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, reward)
}

// GetClaimsForReward godoc
// @Summary      List claims made against a specific reward I own
// @Tags         Rewards
// @Produce      json
// @Security     BearerAuth
// @Param        rewardId  path      string  true   "Reward ID (UUID)"
// @Param        before    query     string  false  "Only return claims older than this RFC3339 timestamp (pagination cursor)"
// @Param        limit     query     int     false  "Max claims to return (default 20, capped at 50)"
// @Success      200  {array}   RewardClaimGivenResponse
// @Failure      400  {object}  map[string]string
// @Failure      403  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Router       /rewards/{rewardId}/claims [get]
func (h *Handler) GetClaimsForReward(c *gin.Context) {
	rewardID, err := uuid.Parse(c.Param("rewardId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid reward ID"})
		return
	}

	userID, ok := httputil.ParseUserID(c)
	if !ok {
		return
	}

	before, limit, ok := parseClaimsPageParams(c)
	if !ok {
		return
	}

	claims, err := h.service.GetClaimsForReward(rewardID, userID, before, limit)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	response := make([]RewardClaimGivenResponse, len(claims))
	for i, claim := range claims {
		response[i] = RewardClaimGivenResponse{
			ID:               claim.ID,
			RewardID:         claim.RewardID,
			RedeemerID:       claim.RedeemerID,
			GiverID:          claim.GiverID,
			Price:            claim.Price,
			Status:           claim.Status,
			RedeemedAt:       claim.RedeemedAt,
			FulfilledAt:      claim.FulfilledAt,
			ResolvedAt:       claim.ResolvedAt,
			RedeemerUsername: claim.RedeemerUsername,
		}
	}

	c.JSON(http.StatusOK, response)
}

func parseClaimsPageParams(c *gin.Context) (before *time.Time, limit int, ok bool) {
	if raw := c.Query("before"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid 'before' timestamp, expected RFC3339"})
			return nil, 0, false
		}
		before = &parsed
	}

	if raw := c.Query("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid 'limit', expected an integer"})
			return nil, 0, false
		}
		limit = parsed
	}

	return before, limit, true
}

// GetClaimsGiven godoc
// @Summary      List of reward claims made against my rewards (as the giver)
// @Tags         Rewards
// @Produce      json
// @Security     BearerAuth
// @Param        before  query     string  false  "Only return claims older than this RFC3339 timestamp (pagination cursor)"
// @Param        limit   query     int     false  "Max claims to return (default 20, capped at 50)"
// @Success      200  {array}   RewardClaimGivenResponse
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /rewards/claims/given [get]
func (h *Handler) GetClaimsGiven(c *gin.Context) {
	userID, ok := httputil.ParseUserID(c)
	if !ok {
		return
	}

	before, limit, ok := parseClaimsPageParams(c)
	if !ok {
		return
	}

	claims, err := h.service.GetClaimsGivenByID(userID, before, limit)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	response := make([]RewardClaimGivenResponse, len(claims))
	for i, claim := range claims {
		response[i] = RewardClaimGivenResponse{
			ID:               claim.ID,
			RewardID:         claim.RewardID,
			RedeemerID:       claim.RedeemerID,
			GiverID:          claim.GiverID,
			Price:            claim.Price,
			Status:           claim.Status,
			RedeemedAt:       claim.RedeemedAt,
			FulfilledAt:      claim.FulfilledAt,
			ResolvedAt:       claim.ResolvedAt,
			RedeemerUsername: claim.RedeemerUsername,
		}
	}

	c.JSON(http.StatusOK, response)
}

// GetClaimHistory godoc
// @Summary      List rewards I have redeemed (my claim history)
// @Tags         Rewards
// @Produce      json
// @Security     BearerAuth
// @Param        before  query     string  false  "Only return claims older than this RFC3339 timestamp (pagination cursor)"
// @Param        limit   query     int     false  "Max claims to return (default 20, capped at 50)"
// @Success      200  {array}   RewardClaimHistoryResponse
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /rewards/claims/me [get]
func (h *Handler) GetClaimHistory(c *gin.Context) {
	userID, ok := httputil.ParseUserID(c)
	if !ok {
		return
	}

	before, limit, ok := parseClaimsPageParams(c)
	if !ok {
		return
	}

	claims, err := h.service.GetClaimHistory(userID, before, limit)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	response := make([]RewardClaimHistoryResponse, len(claims))
	for i, claim := range claims {
		response[i] = RewardClaimHistoryResponse{
			ID:            claim.ID,
			RewardID:      claim.RewardID,
			RedeemerID:    claim.RedeemerID,
			GiverID:       claim.GiverID,
			Price:         claim.Price,
			Status:        claim.Status,
			RedeemedAt:    claim.RedeemedAt,
			FulfilledAt:   claim.FulfilledAt,
			ResolvedAt:    claim.ResolvedAt,
			GiverUsername: claim.GiverUsername,
		}
	}

	c.JSON(http.StatusOK, response)
}

// FulfillClaim godoc
// @Summary      Mark a reward claim as fulfilled
// @Tags         Rewards
// @Produce      json
// @Security     BearerAuth
// @Param        claimId  path      string  true  "Claim ID (UUID)"
// @Success      200       {object}  models.RewardClaim
// @Failure      400       {object}  map[string]string
// @Failure      403       {object}  map[string]string
// @Failure      404       {object}  map[string]string
// @Failure      500       {object}  map[string]string
// @Router       /rewards/claims/{claimId}/fulfill [patch]
func (h *Handler) FulfillClaim(c *gin.Context) {
	claimID, err := uuid.Parse(c.Param("claimId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid claim ID"})
		return
	}

	userID, ok := httputil.ParseUserID(c)
	if !ok {
		return
	}

	claim, err := h.service.MarkClaimFulfilled(claimID, userID)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, claim)
}

// RequestRefund godoc
// @Summary      Request a refund for a reward claim
// @Tags         Rewards
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        claimId  path      string  true  "Claim ID (UUID)"
// @Param        reason    query     string  true  "Reason for refund request"
// @Success      200      {object}  models.RewardClaim
// @Failure      400      {object}  map[string]string
// @Failure      403      {object}  map[string]string
// @Failure      404      {object}  map[string]string
// @Failure      500      {object}  map[string]string
// @Router       /rewards/claims/{claimId}/refund/request [patch]
func (h *Handler) RequestRefund(c *gin.Context) {
	userID, ok := httputil.ParseUserID(c)
	if !ok {
		return
	}
	
	claimID, err := uuid.Parse(c.Param("claimId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid claim ID"})
		return
	}

	reason := c.Query("reason")

	claim, err := h.service.MarkClaimRefundRequested(claimID, userID, reason)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, claim)
}

// ApproveRefund godoc
// @Summary      Approve a refund request for a reward claim
// @Tags         Rewards
// @Produce      json
// @Security     BearerAuth
// @Param        claimId  path      string  true  "Claim ID (UUID)"
// @Success      200       {object}  models.RewardClaim
// @Failure      400       {object}  map[string]string
// @Failure      403       {object}  map[string]string
// @Failure      404       {object}  map[string]string
// @Failure      500       {object}  map[string]string
// @Router       /rewards/claims/{claimId}/refund/approve [patch]
func (h *Handler) ApproveRefund(c *gin.Context) {
	userID, ok := httputil.ParseUserID(c)
	if !ok {
		return
	}

	claimID, err := uuid.Parse(c.Param("claimId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid claim ID"})
		return
	}

	claim, err := h.service.MarkClaimRefunded(claimID, userID)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, claim)
}