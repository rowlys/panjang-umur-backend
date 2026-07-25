package transaction

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rowlys/panjang-umur-backend/internal/httputil"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
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

// GetHistory godoc
// @Summary      Get my transaction history
// @Tags         Transactions
// @Produce      json
// @Security     BearerAuth
// @Success      200  {array}   models.Transaction
// @Failure      500  {object}  map[string]string
// @Router       /transactions/me [get]
func (h *Handler) GetHistory(c *gin.Context) {
	userID, ok := httputil.ParseUserID(c)
	if !ok {
		return
	}

	history, err := h.service.GetHistory(userID)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, history)
}
