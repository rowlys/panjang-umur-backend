package challenge

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/rowlys/panjang-umur-backend/internal/httputil"

	"github.com/rowlys/panjang-umur-backend/internal/models"
)


type CreateChallengeRequest struct {
	Title       string      `json:"title" binding:"required"`
	Description string      `json:"description"`
	Points      int         `json:"points" binding:"required,gt=0"`
	Type        int         `json:"type"`
	ResetDay    *int        `json:"resetDay" binding:"omitempty,min=0,max=6"`
	AssigneeIDs []uuid.UUID `json:"assigneeIds"`
	ExpiresAt   *time.Time  `json:"expiresAt"`
}

type SubmitChallengeRequest struct {
	ProofImageID *uuid.UUID `json:"proofImageId,omitempty"`
}

type GetSubmissionsSubmittedResponse struct {
	ID              uuid.UUID  `json:"id"`
	ChallengeID     uuid.UUID  `json:"challengeId"`
	ChallengeTitle  string     `json:"challengeTitle"`
	ChallengePoints int        `json:"challengePoints"`
	ChallengeType   int        `json:"challengeType"`
	ChallengeStatus int        `json:"challengeStatus"`
	ProofURL        *string    `json:"proofUrl,omitempty"`
	PeriodStart     time.Time  `json:"periodStart"`
	Status          int        `json:"status"`
	SubmittedAt     time.Time  `json:"submittedAt"`
	ApprovedAt      *time.Time `json:"approvedAt"`
}

type GetSubmissionsReceivedResponse struct {
	ID              uuid.UUID          `json:"id"`
	ChallengeID     uuid.UUID          `json:"challengeId"`
	ChallengeTitle  string             `json:"challengeTitle"`
	ChallengePoints int                `json:"challengePoints"`
	ChallengeType   int                `json:"challengeType"`
	ChallengeStatus int                `json:"challengeStatus"`
	UserID          uuid.UUID          `json:"userId"`
	User            models.BareUserDTO `json:"user"`
	ProofURL        *string            `json:"proofUrl,omitempty"`
	PeriodStart     time.Time          `json:"periodStart"`
	Status          int                `json:"status"`
	SubmittedAt     time.Time          `json:"submittedAt"`
	ApprovedAt      *time.Time         `json:"approvedAt"`
}

type GetChallengeDetailResponse struct {
	ID          uuid.UUID          `json:"id"`
	Title       string             `json:"title"`
	Description string             `json:"description"`
	Points      int                `json:"points"`
	Status      int                `json:"status"`
	Type        int                `json:"type"`
	ResetDay    int                `json:"resetDay"`
	Restricted  bool               `json:"restricted"`
	Creator     models.BareUserDTO `json:"creator"`
	CreatedAt   time.Time          `json:"createdAt"`
	ExpiresAt   *time.Time         `json:"expiresAt,omitempty"`
	// MySubmissionStatus is the caller's submission status for the current
	// period (nil if they haven't submitted yet). Always nil for the creator.
	MySubmissionStatus *int `json:"mySubmissionStatus,omitempty"`
}

type GetAssignedChallengeDTO struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Points      int       `json:"points"`
	Type        int       `json:"type"`
	ResetDay    int      `json:"resetDay,omitempty"`
	Creator     models.BareUserDTO `json:"creator"`
	CreatedAt   time.Time `json:"createdAt"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty"`
}


type UserService interface {
	GetByIDs(ids []uuid.UUID) ([]*models.BareUserDTO, error)
}

type Handler struct {
	service Service
	userService UserService
}

func NewHandler(service Service, userService UserService) *Handler {
	return &Handler{service: service, userService: userService}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("", h.GetAll)
	rg.GET("/me", h.GetByAssignee)
	rg.GET("/me/created", h.GetByCreator)
	rg.POST("", h.Create)
	rg.GET("/:id", h.GetByID)

	rg.PATCH("/:challengeId/submit", h.Submit)
	rg.PATCH("/:challengeId/cancel", h.Cancel)
	rg.GET("/submissions/submitted", h.GetSubmissionsSubmitted)
	rg.GET("/submissions/received", h.GetSubmissionsReceived)
	rg.PATCH("/submissions/:submissionId/approve", h.Approve)

	rg.GET("/proof-upload-url", h.GenerateProofUploadURL)
}

// Create godoc
// @Summary      Create a challenge for one or more friends
// @Tags         Challenges
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body     body      CreateChallengeRequest true  "Challenge data"
// @Success      201      {object}  models.Challenge
// @Failure      400      {object}  map[string]string
// @Failure      403      {object}  map[string]string
// @Failure      500      {object}  map[string]string
// @Router       /challenges [post]
func (h *Handler) Create(c *gin.Context) {
	var input CreateChallengeRequest

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, ok := httputil.ParseUserID(c)
	if !ok {
		return
	}

	challenge, err := h.service.Create(c.Request.Context(), CreateChallengeInput{
		Title:       input.Title,
		Description: input.Description,
		Points:      input.Points,
		Type:        input.Type,
		ResetDay:    input.ResetDay,
		CreatorID:   userID,
		AssigneeIDs: input.AssigneeIDs,
		ExpiresAt:   input.ExpiresAt,
	})

	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusCreated, challenge)
}

// Submit godoc
// @Summary      Submit a challenge for approval
// @Tags         Challenges
// @Produce      json
// @Security     BearerAuth
// @Param        challengeId  path      string  true  "Challenge ID (UUID)"
// @Param        body         body      SubmitChallengeRequest true  "Optional proof image"
// @Success      200          {object}  models.Challenge
// @Failure      400          {object}  map[string]string
// @Failure      403          {object}  map[string]string
// @Failure      404          {object}  map[string]string
// @Router       /challenges/{challengeId}/submit [patch]
func (h *Handler) Submit(c *gin.Context) {
	id, err := uuid.Parse(c.Param("challengeId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid challenge ID"})
		return
	}

	userID, ok := httputil.ParseUserID(c)
	if !ok {
		return
	}

	var req SubmitChallengeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	challenge, err := h.service.Submit(c.Request.Context(), id, userID, req.ProofImageID)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, challenge)
}

// Approve godoc
// @Summary      Approve a submitted challenge submission and award points to that user
// @Tags         Challenges
// @Produce      json
// @Security     BearerAuth
// @Param        submissionId  path      string  true  "Submission ID (UUID)"
// @Success      200           {object}  models.ChallengeSubmission
// @Failure      400           {object}  map[string]string
// @Failure      403           {object}  map[string]string
// @Failure      404           {object}  map[string]string
// @Router       /challenges/submissions/{submissionId}/approve [patch]
func (h *Handler) Approve(c *gin.Context) {
	id, err := uuid.Parse(c.Param("submissionId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid submission ID"})
		return
	}

	userID, ok := httputil.ParseUserID(c)
	if !ok {
		return
	}

	submission, err := h.service.Approve(c.Request.Context(), id, userID)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, submission)
}

// Cancel godoc
// @Summary      Cancel an active challenge
// @Tags         Challenges
// @Produce      json
// @Security     BearerAuth
// @Param        challengeId  path      string  true  "Challenge ID (UUID)"
// @Success      200          {object}  models.Challenge
// @Failure      400          {object}  map[string]string
// @Failure      403          {object}  map[string]string
// @Failure      404          {object}  map[string]string
// @Router       /challenges/{challengeId}/cancel [patch]
func (h *Handler) Cancel(c *gin.Context) {
	id, err := uuid.Parse(c.Param("challengeId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid challenge ID"})
		return
	}

	userID, ok := httputil.ParseUserID(c)
	if !ok {
		return
	}

	challenge, err := h.service.Cancel(c.Request.Context(), id, userID)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, challenge)
}

// GetAll godoc
// @Summary      List active challenges visible to me (created by me, assigned to me, or open challenges from my friends)
// @Tags         Challenges
// @Produce      json
// @Security     BearerAuth
// @Success      200  {array}   models.Challenge
// @Failure      500  {object}  map[string]string
// @Router       /challenges [get]
func (h *Handler) GetAll(c *gin.Context) {
	userID, ok := httputil.ParseUserID(c)
	if !ok {
		return
	}

	challenges, err := h.service.GetAll(userID, []models.ChallengeStatus{models.StatusActive})
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, challenges)
}

// GetByID godoc
// @Summary      Get a challenge by ID, including creator info and the caller's submission status for the current period
// @Tags         Challenges
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "Challenge ID (UUID)"
// @Success      200  {object}  GetChallengeDetailResponse
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Router       /challenges/{id} [get]
func (h *Handler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid challenge ID"})
		return
	}

	userID, ok := httputil.ParseUserID(c)
	if !ok {
		return
	}

	challenge, err := h.service.GetByID(userID, id)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	creators, err := h.userService.GetByIDs([]uuid.UUID{challenge.CreatorID})
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}
	
	var creator models.BareUserDTO
	if len(creators) > 0 {
		creator = *creators[0]
	}

	submissionStatus, err := h.service.GetMySubmissionStatus(userID, challenge)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}
	var mySubmissionStatus *int
	if submissionStatus != nil {
		status := int(*submissionStatus)
		mySubmissionStatus = &status
	}

	c.JSON(http.StatusOK, GetChallengeDetailResponse{
		ID:                 challenge.ID,
		Title:              challenge.Title,
		Description:        challenge.Description,
		Points:             challenge.Points,
		Status:             int(challenge.Status),
		Type:               int(challenge.Type),
		ResetDay:           challenge.ResetDay,
		Restricted:         challenge.Restricted,
		Creator:            creator,
		CreatedAt:          challenge.CreatedAt,
		ExpiresAt:          challenge.ExpiresAt,
		MySubmissionStatus: mySubmissionStatus,
	})
}


func parseSubmissionsPageParams(c *gin.Context) (before *time.Time, limit int, ok bool) {
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

func parseOptionalUUIDQuery(c *gin.Context, key string) (*uuid.UUID, bool) {
	raw := c.Query(key)
	if raw == "" {
		return nil, true
	}
	parsed, err := uuid.Parse(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid '" + key + "'"})
		return nil, false
	}
	return &parsed, true
}

// challengeInfoMap batch-loads challenge details for the given submissions' challenge IDs.
func (h *Handler) challengeInfoMap(submissions []models.ChallengeSubmission) (map[uuid.UUID]models.Challenge, error) {
	uniqueChallengeIDs := make(map[uuid.UUID]struct{})
	var challengeIDs []uuid.UUID
	for _, submission := range submissions {
		if _, exists := uniqueChallengeIDs[submission.ChallengeID]; !exists {
			uniqueChallengeIDs[submission.ChallengeID] = struct{}{}
			challengeIDs = append(challengeIDs, submission.ChallengeID)
		}
	}

	challenges, err := h.service.GetByIDs(challengeIDs)
	if err != nil {
		return nil, err
	}

	challengeMap := make(map[uuid.UUID]models.Challenge)
	for _, challenge := range challenges {
		challengeMap[challenge.ID] = challenge
	}
	return challengeMap, nil
}

// GetSubmissionsSubmitted godoc
// @Summary      List challenge submissions I've made, optionally narrowed to one challenge
// @Tags         Challenges
// @Produce      json
// @Security     BearerAuth
// @Param        challengeId  query     string  false  "Only return submissions to this challenge (UUID)"
// @Param        status       query     string  false  "Filter by submission status (submitted, approved, or all)"
// @Param        before       query     string  false  "Only return submissions older than this RFC3339 timestamp (pagination cursor)"
// @Param        limit        query     int     false  "Max submissions to return (default 20, capped at 50)"
// @Success      200  {array}   GetSubmissionsSubmittedResponse
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /challenges/submissions/submitted [get]
func (h *Handler) GetSubmissionsSubmitted(c *gin.Context) {
	userID, ok := httputil.ParseUserID(c)
	if !ok {
		return
	}

	challengeID, ok := parseOptionalUUIDQuery(c, "challengeId")
	if !ok {
		return
	}

	statusFilter := c.Query("status")

	before, limit, ok := parseSubmissionsPageParams(c)
	if !ok {
		return
	}

	submissions, err := h.service.GetSubmissionsSubmitted(userID, challengeID, statusFilter, before, limit)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	challengeMap, err := h.challengeInfoMap(submissions)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	response := make([]GetSubmissionsSubmittedResponse, len(submissions))
	for i, submission := range submissions {
		ch := challengeMap[submission.ChallengeID]
		response[i] = GetSubmissionsSubmittedResponse{
			ID:              submission.ID,
			ChallengeID:     submission.ChallengeID,
			ChallengeTitle:  ch.Title,
			ChallengePoints: ch.Points,
			ChallengeType:   int(ch.Type),
			ChallengeStatus: int(ch.Status),
			ProofURL:        h.service.GetProofURL(submission.ProofImageID),
			PeriodStart:     submission.PeriodStart,
			Status:          int(submission.Status),
			SubmittedAt:     submission.SubmittedAt,
			ApprovedAt:      submission.ApprovedAt,
		}
	}

	c.JSON(http.StatusOK, response)
}

// GetSubmissionsReceived godoc
// @Summary      List submissions made on challenges I created, optionally narrowed to one challenge
// @Tags         Challenges
// @Produce      json
// @Security     BearerAuth
// @Param        challengeId  query     string  false  "Only return submissions to this challenge (UUID)"
// @Param        status       query     string  false  "Filter by submission status (submitted, approved, or all)"
// @Param        before       query     string  false  "Only return submissions older than this RFC3339 timestamp (pagination cursor)"
// @Param        limit        query     int     false  "Max submissions to return (default 20, capped at 50)"
// @Success      200  {array}   GetSubmissionsReceivedResponse
// @Failure      400  {object}  map[string]string
// @Failure      403  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Router       /challenges/submissions/received [get]
func (h *Handler) GetSubmissionsReceived(c *gin.Context) {
	userID, ok := httputil.ParseUserID(c)
	if !ok {
		return
	}

	challengeID, ok := parseOptionalUUIDQuery(c, "challengeId")
	if !ok {
		return
	}

	statusFilter := c.Query("status")

	before, limit, ok := parseSubmissionsPageParams(c)
	if !ok {
		return
	}

	submissions, err := h.service.GetSubmissionsReceived(userID, challengeID, statusFilter, before, limit)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	challengeMap, err := h.challengeInfoMap(submissions)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	uniqueSubmitterIDs := make(map[uuid.UUID]struct{})
	var submitterIDs []uuid.UUID
	for _, submission := range submissions {
		if _, exists := uniqueSubmitterIDs[submission.UserID]; !exists {
			uniqueSubmitterIDs[submission.UserID] = struct{}{}
			submitterIDs = append(submitterIDs, submission.UserID)
		}
	}

	submitters, err := h.userService.GetByIDs(submitterIDs)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	submitterMap := make(map[uuid.UUID]models.BareUserDTO)
	for _, submitter := range submitters {
		submitterMap[submitter.ID] = *submitter
	}

	response := make([]GetSubmissionsReceivedResponse, len(submissions))
	for i, submission := range submissions {
		ch := challengeMap[submission.ChallengeID]
		response[i] = GetSubmissionsReceivedResponse{
			ID:              submission.ID,
			ChallengeID:     submission.ChallengeID,
			ChallengeTitle:  ch.Title,
			ChallengePoints: ch.Points,
			ChallengeType:   int(ch.Type),
			ChallengeStatus: int(ch.Status),
			UserID:          submission.UserID,
			User:            submitterMap[submission.UserID],
			ProofURL:        h.service.GetProofURL(submission.ProofImageID),
			PeriodStart:     submission.PeriodStart,
			Status:          int(submission.Status),
			SubmittedAt:     submission.SubmittedAt,
			ApprovedAt:      submission.ApprovedAt,
		}
	}

	c.JSON(http.StatusOK, response)
}

// GetByAssignee godoc
// @Summary      Get challenges assigned to the current user that are still open or awaiting approval
// @Tags         Challenges
// @Produce      json
// @Security     BearerAuth
// @Success      200  {array}   models.Challenge
// @Failure      500  {object}  map[string]string
// @Router       /challenges/me [get]
func (h *Handler) GetByAssignee(c *gin.Context) {
	userID, ok := httputil.ParseUserID(c)
	if !ok {
		return
	}

	challenges, err := h.service.GetByAssignee(userID)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	uniqueCreatorIDs := make(map[uuid.UUID]struct{})
	var creatorIDs []uuid.UUID

	for _, challenge := range challenges {
		if _, exists := uniqueCreatorIDs[challenge.CreatorID]; !exists {
			uniqueCreatorIDs[challenge.CreatorID] = struct{}{}
			creatorIDs = append(creatorIDs, challenge.CreatorID)
		}
	}

	creators, err := h.userService.GetByIDs(creatorIDs)
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	creatorMap := make(map[uuid.UUID]models.BareUserDTO)
	for _, creator := range creators {
		creatorMap[creator.ID] = *creator
	}

	response := make([]GetAssignedChallengeDTO, len(challenges))
	for i, challenge := range challenges {
		if creator, exists := creatorMap[challenge.CreatorID]; exists {
			response[i] = GetAssignedChallengeDTO{
				ID: challenge.ID,
				Title: challenge.Title,
				Description: challenge.Description,
				Points: challenge.Points,
				Type: int(challenge.Type),
				ResetDay: challenge.ResetDay,
				Creator:   creator,
				CreatedAt: challenge.CreatedAt,
				ExpiresAt: challenge.ExpiresAt,
			}
		}
	}

	c.JSON(http.StatusOK, response)
}

// GetByCreator godoc
// @Summary      Get challenges created by the current user
// @Tags         Challenges
// @Produce      json
// @Security     BearerAuth
// @Success      200  {array}   models.Challenge
// @Failure      500  {object}  map[string]string
// @Router       /challenges/me/created [get]
func (h *Handler) GetByCreator(c *gin.Context) {
	userID, ok := httputil.ParseUserID(c)
	if !ok {
		return
	}

	challenges, err := h.service.GetByCreator(userID, []models.ChallengeStatus{models.StatusActive})
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, challenges)
}

// GenerateProofUploadURL godoc
// @Summary      Generate a presigned URL for uploading a proof image to S3
// @Tags         Challenges
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /challenges/proof-upload-url [get]
func (h *Handler) GenerateProofUploadURL(c *gin.Context) {
	uploadURL, imageID, err := h.service.GenerateProofUploadURL(c.Request.Context())
	if err != nil {
		code, msg := httputil.ResolveServiceError(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"uploadURL": uploadURL,
		"imageID":   imageID,
	})
}
