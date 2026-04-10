package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/rowlys/panjang-umur-backend/internal/database"
	"github.com/rowlys/panjang-umur-backend/internal/models"
)

type CreateChallengeInput struct {
  Title    string   `json:"title" binding:"required"`
  Description string   `json:"description"`
  Points   int    `json:"points" binding:"required,gt=0"` 
  Type    int    `json:"type"`                
  AssigneeID *uuid.UUID `json:"assigneeId"`
}

// Handler functions for challenges

func GetAllChallenges(c *gin.Context) {
	var challenges []models.Challenge

	if result := database.DB.Where("status IN ?", []models.ChallengeStatus{models.StatusActive, models.StatusPending}).Find(&challenges); result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch challenges"})
		return
	}

	c.JSON(http.StatusOK, challenges)
}

func CreateChallenge(c *gin.Context) {
	var input CreateChallengeInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	groupIDStr := c.Param("groupId")
	groupID, err := uuid.Parse(groupIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse group ID"})
		return
	}

	rawUserID, exists := c.Get("userID")

	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized context"})
		return
	}

	userID, err := uuid.Parse(rawUserID.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Failed to parse user ID"})
		return
	}

	var count int64
	database.DB.Table("user_groups").Where("user_id = ? AND group_id = ?", userID, groupID).Count(&count)
	if count == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to create challenges here"})
		return
	}



	challenge := models.Challenge{
		ID:     uuid.New(),
		Title:    input.Title,
		Description: input.Description,
		Points:   input.Points,
		Status:   models.StatusActive,
		Type:    models.ChallengeType(input.Type),
		CreatorID:  userID,
		AssigneeID: input.AssigneeID,
		GroupID:   groupID,
	}

	if result := database.DB.Create(&challenge); result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}

	c.JSON(http.StatusCreated, challenge)
}

func SubmitChallenge(c *gin.Context) {
	challengeId := c.Param("challengeId")

	rawUserID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized context"})
		return
	}

	userID, err := uuid.Parse(rawUserID.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Failed to parse user ID"})
		return
	}

	var challenge models.Challenge

	if err := database.DB.First(&challenge, "id = ? AND assignee_id = ?", challengeId, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Challenge not found"})
		return
	}

	if challenge.Status != models.StatusActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only active challenges can be submitted for verification"})
		return
	}

	challenge.Status = models.StatusPending
	database.DB.Save(&challenge)

	c.JSON(http.StatusOK, challenge)
}

func ApproveChallenge(c *gin.Context) {
	challengeId := c.Param("challengeId")

	rawUserID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized context"})
		return
	}

	userID, err := uuid.Parse(rawUserID.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Failed to parse user ID"})
		return
	}

	var challenge models.Challenge
	var user models.User

	if err := database.DB.First(&challenge, "id = ? AND creator_id = ?", challengeId, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Challenge not found"})
		return
	}

	if err := database.DB.First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if challenge.Status != models.StatusPending {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Challenge is not pending verification"})
		return
	}

	challenge.Status = models.StatusCompleted
	user.PointBalance += challenge.Points

	database.DB.Save(&challenge)
	database.DB.Save(&user)

	c.JSON(http.StatusOK, challenge)

}


// ============== GET: Assigned Challenges ==============
func GetAllMyChallenges(c *gin.Context) {
	rawUserID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized context"})
		return
	}

	userID := rawUserID.(string)

	var challenges []models.Challenge

	err := database.DB.
		Where("assignee_id = ?", userID).
		Where("status IN ?", []models.ChallengeStatus{models.StatusActive, models.StatusPending}).
		Find(&challenges).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user challenges"})
		return
	}

	c.JSON(http.StatusOK, challenges)
}

func GetMyGroupChallenges(c *gin.Context) {
	groupID := c.Param("groupId")
	rawUserID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized context"})
		return
	}

	userID := rawUserID.(string)

	var count int64
	database.DB.Table("user_groups").Where("user_id = ? AND group_id = ?", userID, groupID).Count(&count)
	if count == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to view this group"})
		return
	}

	var challenges []models.Challenge

	err := database.DB.
		Where("group_id = ?", groupID).
		Where("assignee_id = ?", userID).
		Where("status IN ?", []models.ChallengeStatus{models.StatusActive, models.StatusPending}).
		Find(&challenges).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user challenges"})
		return
	}

	c.JSON(http.StatusOK, challenges)
}
// ========================================================

// ============== GET: Created Challenges ==============
func GetAllMyCreatedChallenges(c *gin.Context) {
	rawUserID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized context"})
		return
	}

	userID := rawUserID.(string)

	var challenges []models.Challenge

	err := database.DB.
		Where("creator_id = ?", userID).
		Where("status IN ?", []models.ChallengeStatus{models.StatusActive, models.StatusPending}).
		Find(&challenges).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user challenges"})
		return
	}

	c.JSON(http.StatusOK, challenges)
}

func GetMyGroupCreatedChallenges(c *gin.Context) {
	groupID := c.Param("groupId")
	rawUserID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized context"})
		return
	}

	userID := rawUserID.(string)

	var count int64
	database.DB.Table("user_groups").Where("user_id = ? AND group_id = ?", userID, groupID).Count(&count)
	if count == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to view this group"})
		return
	}

	var challenges []models.Challenge

	err := database.DB.
		Where("creator_id = ?", userID).
		Where("group_id = ?", groupID).
		Where("status IN ?", []models.ChallengeStatus{models.StatusActive, models.StatusPending}).
		Find(&challenges).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user challenges"})
		return
	}

	c.JSON(http.StatusOK, challenges)
}

// ========================================================

func GetChallengeByID(c *gin.Context) {
  challengeID := c.Param("id")
  var challenge models.Challenge

  err := database.DB.
    Where("id = ?", challengeID).
    First(&challenge).Error

  if err != nil {
    c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch challenge"})
    return
  }

  c.JSON(http.StatusOK, challenge)
}


