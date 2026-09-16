package user

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"
	"gorm.io/gorm"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"

	"github.com/rowlys/panjang-umur-backend/internal/config"
	"github.com/rowlys/panjang-umur-backend/internal/httputil"
	"github.com/rowlys/panjang-umur-backend/internal/models"
)

type LoginResponse struct {
	User  models.BareUserDTO `json:"user"`
	Token string  `json:"token"`
}

type UpdateProfileInput struct {
	Name     string `json:"name" binding:"required"`
	Username string `json:"username" binding:"required"`
}

type ChangePasswordInput struct {
	CurrentPassword string `json:"currentPassword" binding:"required"`
	NewPassword     string `json:"newPassword" binding:"required,min=6"`
}

type Service interface {
	Register(ctx context.Context, input RegisterInput) (*models.User, error)
	Login(ctx context.Context, input LoginInput) (*LoginResponse, error)
	GetByID(id uuid.UUID) (*models.User, error)
	GetByUsername(username string) (*models.User, error)
	GetByIDs(ids []uuid.UUID) ([]*models.BareUserDTO, error)
	SearchByUsername(callerId uuid.UUID, prefix string, limit int) ([]models.BareUserDTO, error)
	UpdateProfile(ctx context.Context, userID uuid.UUID, input UpdateProfileInput) (*models.User, error)
	ChangePassword(ctx context.Context, userID uuid.UUID, input ChangePasswordInput) error
}

type service struct {
	repo Repository
	jwtSecret string
}

func NewService(repo Repository) Service {
	secret := config.GetEnv("JWT_SECRET", "")
	if secret == "" {
		log.Fatal("JWT_SECRET environment variable must be set")
	}
	return &service{repo: repo, jwtSecret: secret}
}

func (s *service) Register(ctx context.Context, input RegisterInput) (*models.User, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)

	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to secure password"}
	}

	user := &models.User{
		ID:           uuid.New(),
		Username:     input.Username,
		Name:         input.Name,
		PasswordHash: string(hashedPassword),
	}

	if err := s.repo.Create(user); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, &httputil.ServiceError{Code: http.StatusConflict, Message: "Username already exists"}
		}
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to create user"}
	}

	return user, nil
}


func (s *service) Login(ctx context.Context, input LoginInput) (*LoginResponse, error) {
	user, err := s.repo.FindByUsername(input.Username)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusUnauthorized, Message: "Invalid credentials"}
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusUnauthorized, Message: "Invalid credentials"}
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": user.ID,                                 
		"exp": time.Now().Add(time.Hour * 72).Unix(),   
		"iat": time.Now().Unix(),                       
	})

	tokenString, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to generate token"}
	}

	userDTO := models.BareUserDTO{
		ID:       user.ID,
		Username: user.Username,
		Name:     user.Name,
	}

	loginResponse := LoginResponse{
		User:  userDTO,
		Token: tokenString,
	}

	return &loginResponse, nil
}

func (s *service) GetByID(id uuid.UUID) (*models.User, error) {
	user, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &httputil.ServiceError{Code: http.StatusNotFound, Message: "User not found"}
		}
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to retrieve user"}
	}
	return user, nil
}

func (s *service) GetByUsername(username string) (*models.User, error) {
	user, err := s.repo.FindByUsername(username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &httputil.ServiceError{Code: http.StatusNotFound, Message: "User not found"}
		}
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to retrieve user"}
	}
	return user, nil
}

func (s *service) GetByIDs(ids []uuid.UUID) ([]*models.BareUserDTO, error) {
	users, err := s.repo.FindByIDs(ids)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to retrieve users"}
	}
	return users, nil
}

func (s *service) SearchByUsername(callerId uuid.UUID, prefix string, limit int) ([]models.BareUserDTO, error) {
	return s.repo.SearchByUsername(callerId, prefix, limit)
}

func (s *service) UpdateProfile(ctx context.Context, userID uuid.UUID, input UpdateProfileInput) (*models.User, error) {
	user, err := s.repo.FindByID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &httputil.ServiceError{Code: http.StatusNotFound, Message: "User not found"}
		}
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to retrieve user"}
	}

	user.Name = input.Name
	user.Username = input.Username

	if err := s.repo.Save(user); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, &httputil.ServiceError{Code: http.StatusConflict, Message: "Username already exists"}
		}
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to update profile"}
	}

	return user, nil
}

func (s *service) ChangePassword(ctx context.Context, userID uuid.UUID, input ChangePasswordInput) error {
	user, err := s.repo.FindByID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &httputil.ServiceError{Code: http.StatusNotFound, Message: "User not found"}
		}
		return &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to retrieve user"}
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.CurrentPassword)); err != nil {
		return &httputil.ServiceError{Code: http.StatusUnauthorized, Message: "Current password is incorrect"}
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to secure password"}
	}

	user.PasswordHash = string(hashedPassword)
	if err := s.repo.Save(user); err != nil {
		return &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to update password"}
	}

	return nil
}