package user

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"

	"github.com/rowlys/panjang-umur-backend/internal/config"
	"github.com/rowlys/panjang-umur-backend/internal/httputil"
	"github.com/rowlys/panjang-umur-backend/internal/models"
)

type LoginResponse struct {
	UserID uuid.UUID 
	Token string 
}

type Service interface {
	Register(ctx context.Context, input RegisterInput) (*models.User, error)
	Login(ctx context.Context, input LoginInput) (*LoginResponse, error)
	GetByID(id uuid.UUID) (*models.User, error)
	GetByUsername(username string) (*models.User, error)
	GetByIDs(ids []uuid.UUID) ([]*models.User, error)
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

	loginResponse := LoginResponse{
		UserID: user.ID,
		Token:   tokenString,
	}

	return &loginResponse, nil
}

func (s *service) GetByID(id uuid.UUID) (*models.User, error) {
	return s.repo.FindByID(id)
}

func (s *service) GetByUsername(username string) (*models.User, error) {
	return s.repo.FindByUsername(username)
}

func (s *service) GetByIDs(ids []uuid.UUID) ([]*models.User, error) {
	return s.repo.FindByIDs(ids)
}