package user

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/aws/smithy-go"
	"github.com/google/uuid"

	"github.com/rowlys/panjang-umur-backend/internal/httputil"
	"github.com/rowlys/panjang-umur-backend/internal/models"
)

const avatarUploadExpiry = 15 * time.Minute

// avatarTmpKey is where the client uploads. The user ID in the path stops a
// caller from promoting an object they do not own.
func avatarTmpKey(userID, imageID uuid.UUID) string {
	return fmt.Sprintf("tmp/avatars/%s/%s", userID, imageID)
}

// avatarKey is the permanent object. It is unique per upload, so a new picture
// gets a new URL and clients can cache the old one indefinitely.
func avatarKey(userID, imageID uuid.UUID) string {
	return fmt.Sprintf("avatars/%s/%s.jpg", userID, imageID)
}

func (s *service) GenerateAvatarUploadURL(ctx context.Context, userID uuid.UUID) (string, uuid.UUID, error) {
	imageID := uuid.New()

	url, err := s.imageStorage.GeneratePresignedURL(ctx, avatarTmpKey(userID, imageID), avatarUploadExpiry)
	if err != nil {
		return "", uuid.Nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to prepare avatar upload"}
	}

	return url, imageID, nil
}

func (s *service) UpdateAvatar(ctx context.Context, userID uuid.UUID, imageID uuid.UUID) (*models.User, error) {
	if imageID == uuid.Nil {
		return nil, &httputil.ServiceError{Code: http.StatusBadRequest, Message: "Invalid image ID"}
	}

	user, err := s.repo.FindByID(userID)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusNotFound, Message: "User not found"}
	}

	destination := avatarKey(userID, imageID)
	if err := s.imageStorage.PromoteFile(ctx, avatarTmpKey(userID, imageID), destination); err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && (apiErr.ErrorCode() == "NoSuchKey" || apiErr.ErrorCode() == "NotFound") {
			return nil, &httputil.ServiceError{Code: http.StatusBadRequest, Message: "Upload not found or expired"}
		}
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to save avatar"}
	}

	previousURL := user.AvatarURL
	user.AvatarURL = s.imageStorage.GetPublicURL(destination)

	if err := s.repo.Save(user); err != nil {
		// Do not leave an orphaned object if the database refused the change.
		_ = s.imageStorage.DeleteFile(ctx, destination)
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to save avatar"}
	}

	s.deleteAvatarFile(ctx, previousURL)
	return user, nil
}

func (s *service) RemoveAvatar(ctx context.Context, userID uuid.UUID) (*models.User, error) {
	user, err := s.repo.FindByID(userID)
	if err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusNotFound, Message: "User not found"}
	}

	if user.AvatarURL == "" {
		return user, nil
	}

	previousURL := user.AvatarURL
	user.AvatarURL = ""
	if err := s.repo.Save(user); err != nil {
		return nil, &httputil.ServiceError{Code: http.StatusInternalServerError, Message: "Failed to remove avatar"}
	}

	s.deleteAvatarFile(ctx, previousURL)
	return user, nil
}

// deleteAvatarFile is best-effort: the profile change is already saved, so a
// storage hiccup should not turn it into a failure.
func (s *service) deleteAvatarFile(ctx context.Context, url string) {
	key := s.imageStorage.KeyFromURL(url)
	if key == "" {
		return
	}
	if err := s.imageStorage.DeleteFile(ctx, key); err != nil {
		log.Printf("failed to delete avatar object %s: %v", key, err)
	}
}
