package transaction

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rowlys/panjang-umur-backend/internal/httputil"
	"github.com/rowlys/panjang-umur-backend/internal/models"
)

type RewardService interface {
	GetClaimContexts(ids []uuid.UUID) ([]models.ClaimContext, error)
}

type ChallengeService interface {
	GetSubmissionContexts(ids []uuid.UUID) ([]models.SubmissionContext, error)
}

type UserService interface {
	GetByIDs(ids []uuid.UUID) ([]*models.BareUserDTO, error)
}

type Handler struct {
	service          Service
	rewardService    RewardService
	challengeService ChallengeService
	userService      UserService
}

func NewHandler(service Service, rewardService RewardService, challengeService ChallengeService, userService UserService) *Handler {
	return &Handler{
		service:          service,
		rewardService:    rewardService,
		challengeService: challengeService,
		userService:      userService,
	}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/balances", h.GetBalances)
	rg.GET("/me", h.GetHistory)
}

// GetBalances godoc
// @Summary      Get my point balances per giver
// @Tags         Transactions
// @Produce      json
// @Security     BearerAuth
// @Success      200  {array}   models.UserPointBalance
// @Failure      500  {object}  map[string]string
// @Router       /transactions/balances [get]
func (h *Handler) GetBalances(c *gin.Context) {
	userID, ok := httputil.ParseUserID(c)
	if !ok {
		return
	}

	balances, err := h.service.GetAllBalances(userID)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, balances)
}

type TransactionResponse struct {
	ID                   uuid.UUID `json:"id"`
	Role                  string    `json:"role"`
	Type                  int       `json:"type"`
	Amount                int       `json:"amount"`
	ReferenceType         int       `json:"referenceType"`
	ReferenceID           uuid.UUID `json:"referenceId"`
	Title                 string    `json:"title"`
	CounterpartyUsername  string    `json:"counterpartyUsername"`
	Timestamp             time.Time `json:"timestamp"`
}

func parseReferenceTypeQuery(c *gin.Context) (*models.TransactionReferenceType, bool) {
	raw := c.Query("type")
	if raw == "" {
		return nil, true
	}
	switch raw {
	case "rewards":
		t := models.ReferenceClaim
		return &t, true
	case "challenges":
		t := models.ReferenceSubmission
		return &t, true
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid 'type', expected 'rewards' or 'challenges'"})
		return nil, false
	}
}

func parseHistoryPageParams(c *gin.Context) (before *time.Time, limit int, ok bool) {
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

type resolvedEntry struct {
	role           string
	title          string
	counterpartyID uuid.UUID
}

// GetHistory godoc
// @Summary      Get my transaction history (every point movement I took part in)
// @Tags         Transactions
// @Produce      json
// @Security     BearerAuth
// @Param        type    query     string  false  "Only return transactions of this type: 'rewards' or 'challenges'"
// @Param        before  query     string  false  "Only return transactions older than this RFC3339 timestamp (pagination cursor)"
// @Param        limit   query     int     false  "Max transactions to return (default 20, capped at 50)"
// @Success      200  {array}   TransactionResponse
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /transactions/me [get]
func (h *Handler) GetHistory(c *gin.Context) {
	userID, ok := httputil.ParseUserID(c)
	if !ok {
		return
	}

	referenceType, ok := parseReferenceTypeQuery(c)
	if !ok {
		return
	}

	before, limit, ok := parseHistoryPageParams(c)
	if !ok {
		return
	}

	txs, err := h.service.GetHistory(userID, referenceType, before, limit)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	var claimIDs []uuid.UUID
	var submissionIDs []uuid.UUID
	for _, tx := range txs {
		switch tx.ReferenceType {
		case models.ReferenceClaim:
			claimIDs = append(claimIDs, tx.ReferenceID)
		case models.ReferenceSubmission:
			submissionIDs = append(submissionIDs, tx.ReferenceID)
		}
	}

	claimContexts, err := h.rewardService.GetClaimContexts(claimIDs)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}
	claimMap := make(map[uuid.UUID]models.ClaimContext, len(claimContexts))
	for _, cc := range claimContexts {
		claimMap[cc.ClaimID] = cc
	}

	submissionContexts, err := h.challengeService.GetSubmissionContexts(submissionIDs)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}
	submissionMap := make(map[uuid.UUID]models.SubmissionContext, len(submissionContexts))
	for _, sc := range submissionContexts {
		submissionMap[sc.SubmissionID] = sc
	}

	uniqueUserIDs := make(map[uuid.UUID]struct{})
	var userIDs []uuid.UUID
	addUserID := func(id uuid.UUID) {
		if _, exists := uniqueUserIDs[id]; !exists {
			uniqueUserIDs[id] = struct{}{}
			userIDs = append(userIDs, id)
		}
	}

	resolvedByTx := make(map[uuid.UUID]resolvedEntry, len(txs))
	for _, tx := range txs {
		role := "owner"
		if tx.UserID != userID {
			role = "giver"
		}

		var title string
		var counterpartyID uuid.UUID
		switch tx.ReferenceType {
		case models.ReferenceClaim:
			cc := claimMap[tx.ReferenceID]
			title = cc.RewardTitle
			if role == "owner" {
				counterpartyID = cc.GiverID
			} else {
				counterpartyID = cc.RedeemerID
			}
		case models.ReferenceSubmission:
			sc := submissionMap[tx.ReferenceID]
			title = sc.ChallengeTitle
			if role == "owner" {
				counterpartyID = sc.CreatorID
			} else {
				counterpartyID = sc.SubmitterID
			}
		}

		addUserID(counterpartyID)
		resolvedByTx[tx.ID] = resolvedEntry{role: role, title: title, counterpartyID: counterpartyID}
	}

	users, err := h.userService.GetByIDs(userIDs)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}
	usernameMap := make(map[uuid.UUID]string, len(users))
	for _, u := range users {
		usernameMap[u.ID] = u.Username
	}

	response := make([]TransactionResponse, len(txs))
	for i, tx := range txs {
		r := resolvedByTx[tx.ID]
		username, exists := usernameMap[r.counterpartyID]
		if !exists {
			username = "Unknown"
		}

		response[i] = TransactionResponse{
			ID:                    tx.ID,
			Role:                  r.role,
			Type:                  int(tx.Type),
			Amount:                tx.Amount,
			ReferenceType:         int(tx.ReferenceType),
			ReferenceID:           tx.ReferenceID,
			Title:                 r.title,
			CounterpartyUsername:  username,
			Timestamp:             tx.Timestamp,
		}
	}

	c.JSON(http.StatusOK, response)
}
