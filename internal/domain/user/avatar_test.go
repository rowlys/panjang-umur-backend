package user

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/rowlys/panjang-umur-backend/internal/models"
	"github.com/rowlys/panjang-umur-backend/internal/pkg/image_storage"
)

type avatarRepo struct {
	Repository

	user    *models.User
	saved   *models.User
	saveErr error
}

func (r *avatarRepo) FindByID(id uuid.UUID) (*models.User, error) {
	if r.user == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return r.user, nil
}

func (r *avatarRepo) Save(user *models.User) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	r.saved = user
	return nil
}

type avatarStorage struct {
	image_storage.Service

	presignedKey  string
	promotedFrom  string
	promotedTo    string
	deleted       []string
	promoteErr    error
	presignErr    error
}

func (s *avatarStorage) GeneratePresignedURL(_ context.Context, key string, _ time.Duration) (string, error) {
	if s.presignErr != nil {
		return "", s.presignErr
	}
	s.presignedKey = key
	return "https://upload.test/" + key, nil
}

func (s *avatarStorage) PromoteFile(_ context.Context, sourceKey, destinationKey string) error {
	s.promotedFrom = sourceKey
	s.promotedTo = destinationKey
	return s.promoteErr
}

func (s *avatarStorage) DeleteFile(_ context.Context, key string) error {
	s.deleted = append(s.deleted, key)
	return nil
}

func (s *avatarStorage) GetPublicURL(key string) string {
	return "https://cdn.test/" + key
}

func (s *avatarStorage) KeyFromURL(url string) string {
	return strings.TrimPrefix(url, "https://cdn.test/")
}

func TestGenerateAvatarUploadURLScopesKeyToUser(t *testing.T) {
	userID := uuid.New()
	storage := &avatarStorage{}
	service := &service{repo: &avatarRepo{}, imageStorage: storage}

	uploadURL, imageID, err := service.GenerateAvatarUploadURL(context.Background(), userID)
	if err != nil {
		t.Fatalf("GenerateAvatarUploadURL() error = %v", err)
	}
	if imageID == uuid.Nil {
		t.Fatal("image ID is nil")
	}

	wantKey := avatarTmpKey(userID, imageID)
	if storage.presignedKey != wantKey {
		t.Fatalf("presigned key = %q, want %q", storage.presignedKey, wantKey)
	}
	if !strings.Contains(uploadURL, wantKey) {
		t.Fatalf("upload URL %q does not contain the key %q", uploadURL, wantKey)
	}
}

func TestUpdateAvatarPromotesAndDeletesTheOldFile(t *testing.T) {
	userID := uuid.New()
	imageID := uuid.New()
	oldKey := "avatars/" + userID.String() + "/old.jpg"

	repo := &avatarRepo{user: &models.User{ID: userID, AvatarURL: "https://cdn.test/" + oldKey}}
	storage := &avatarStorage{}
	service := &service{repo: repo, imageStorage: storage}

	updated, err := service.UpdateAvatar(context.Background(), userID, imageID)
	if err != nil {
		t.Fatalf("UpdateAvatar() error = %v", err)
	}

	if storage.promotedFrom != avatarTmpKey(userID, imageID) {
		t.Fatalf("promoted from = %q", storage.promotedFrom)
	}
	if storage.promotedTo != avatarKey(userID, imageID) {
		t.Fatalf("promoted to = %q", storage.promotedTo)
	}

	wantURL := "https://cdn.test/" + avatarKey(userID, imageID)
	if updated.AvatarURL != wantURL {
		t.Fatalf("avatar URL = %q, want %q", updated.AvatarURL, wantURL)
	}
	if repo.saved != updated {
		t.Fatal("the updated user was not saved")
	}
	if len(storage.deleted) != 1 || storage.deleted[0] != oldKey {
		t.Fatalf("deleted = %v, want [%q]", storage.deleted, oldKey)
	}
}

func TestUpdateAvatarKeepsTheOldFileWhenPromotionFails(t *testing.T) {
	userID := uuid.New()
	imageID := uuid.New()

	repo := &avatarRepo{user: &models.User{ID: userID, AvatarURL: "https://cdn.test/avatars/old.jpg"}}
	storage := &avatarStorage{promoteErr: errors.New("copy failed")}
	service := &service{repo: repo, imageStorage: storage}

	if _, err := service.UpdateAvatar(context.Background(), userID, imageID); err == nil {
		t.Fatal("UpdateAvatar() error = nil, want a failure")
	}
	if repo.saved != nil {
		t.Fatal("the user should not be saved when promotion fails")
	}
	if len(storage.deleted) != 0 {
		t.Fatalf("deleted = %v, want none", storage.deleted)
	}
}

func TestUpdateAvatarRejectsNilImageID(t *testing.T) {
	service := &service{repo: &avatarRepo{user: &models.User{ID: uuid.New()}}, imageStorage: &avatarStorage{}}

	if _, err := service.UpdateAvatar(context.Background(), uuid.New(), uuid.Nil); err == nil {
		t.Fatal("UpdateAvatar() error = nil, want a validation error")
	}
}

func TestRemoveAvatarClearsTheReferenceAndDeletesTheFile(t *testing.T) {
	userID := uuid.New()
	key := "avatars/" + userID.String() + "/current.jpg"

	repo := &avatarRepo{user: &models.User{ID: userID, AvatarURL: "https://cdn.test/" + key}}
	storage := &avatarStorage{}
	service := &service{repo: repo, imageStorage: storage}

	updated, err := service.RemoveAvatar(context.Background(), userID)
	if err != nil {
		t.Fatalf("RemoveAvatar() error = %v", err)
	}
	if updated.AvatarURL != "" {
		t.Fatalf("avatar URL = %q, want empty", updated.AvatarURL)
	}
	if len(storage.deleted) != 1 || storage.deleted[0] != key {
		t.Fatalf("deleted = %v, want [%q]", storage.deleted, key)
	}
}
