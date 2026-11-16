package challenge

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rowlys/panjang-umur-backend/internal/models"
	"github.com/rowlys/panjang-umur-backend/internal/pkg/image_storage"
)

// purgeRepo implements only the two methods the purge uses; the embedded
// interface keeps the rest of the Repository contract away from the test.
type purgeRepo struct {
	Repository

	pending  []models.ChallengeSubmission
	cleared  map[uuid.UUID]uuid.UUID
	queryErr error
}

func (r *purgeRepo) FindProofImagesToPurge(cutoff time.Time, limit int) ([]models.ChallengeSubmission, error) {
	if r.queryErr != nil {
		return nil, r.queryErr
	}
	return r.pending, nil
}

func (r *purgeRepo) ClearProofImage(id uuid.UUID, imageID uuid.UUID) error {
	r.cleared[id] = imageID
	remaining := r.pending[:0]
	for _, submission := range r.pending {
		if submission.ID != id {
			remaining = append(remaining, submission)
		}
	}
	r.pending = remaining
	return nil
}

// purgeStorage records deletions and can fail selected keys.
type purgeStorage struct {
	image_storage.Service

	deleted  []string
	failKeys map[string]bool
}

func (s *purgeStorage) DeleteFile(_ context.Context, key string) error {
	if s.failKeys[key] {
		return errors.New("storage unavailable")
	}
	s.deleted = append(s.deleted, key)
	return nil
}

func oldApprovedSubmission(imageID uuid.UUID, approvedAt time.Time) models.ChallengeSubmission {
	return models.ChallengeSubmission{
		ID:           uuid.New(),
		ProofImageID: &imageID,
		Status:       models.SubmissionApproved,
		ApprovedAt:   &approvedAt,
	}
}

func TestPurgeOldProofImagesDeletesFilesAndClearsReferences(t *testing.T) {
	first := uuid.New()
	second := uuid.New()
	approvedAt := time.Now().AddDate(0, -2, 0)

	repo := &purgeRepo{
		pending: []models.ChallengeSubmission{
			oldApprovedSubmission(first, approvedAt),
			oldApprovedSubmission(second, approvedAt),
		},
		cleared: make(map[uuid.UUID]uuid.UUID),
	}
	storage := &purgeStorage{failKeys: map[string]bool{}}
	service := &service{repo: repo, imageStorage: storage}

	deleted, err := service.PurgeOldProofImages(context.Background(), time.Now().AddDate(0, -1, 0))
	if err != nil {
		t.Fatalf("PurgeOldProofImages() error = %v", err)
	}
	if deleted != 2 {
		t.Fatalf("deleted = %d, want 2", deleted)
	}
	if len(storage.deleted) != 2 {
		t.Fatalf("deleted keys = %v, want 2 keys", storage.deleted)
	}
	if got, want := storage.deleted[0], "submissions/"+first.String(); got != want {
		t.Fatalf("first key = %q, want %q", got, want)
	}
	if len(repo.cleared) != 2 {
		t.Fatalf("cleared %d references, want 2", len(repo.cleared))
	}
	if len(repo.pending) != 0 {
		t.Fatalf("pending rows left = %d, want 0", len(repo.pending))
	}
}

func TestPurgeOldProofImagesKeepsRowsWhoseFileFailed(t *testing.T) {
	broken := uuid.New()
	healthy := uuid.New()
	approvedAt := time.Now().AddDate(0, -2, 0)

	repo := &purgeRepo{
		pending: []models.ChallengeSubmission{
			oldApprovedSubmission(broken, approvedAt),
			oldApprovedSubmission(healthy, approvedAt),
		},
		cleared: make(map[uuid.UUID]uuid.UUID),
	}
	storage := &purgeStorage{failKeys: map[string]bool{"submissions/" + broken.String(): true}}
	service := &service{repo: repo, imageStorage: storage}

	deleted, err := service.PurgeOldProofImages(context.Background(), time.Now().AddDate(0, -1, 0))
	if err == nil {
		t.Fatal("PurgeOldProofImages() error = nil, want the storage failure")
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}
	if len(repo.pending) != 1 || repo.pending[0].ProofImageID == nil || *repo.pending[0].ProofImageID != broken {
		t.Fatal("the row whose file failed to delete should still be pending")
	}
	if _, cleared := repo.cleared[repo.pending[0].ID]; cleared {
		t.Fatal("a failed delete must not clear the stored reference")
	}
}

func TestPurgeOldProofImagesReturnsQueryError(t *testing.T) {
	wantErr := errors.New("database down")
	service := &service{
		repo:         &purgeRepo{cleared: map[uuid.UUID]uuid.UUID{}, queryErr: wantErr},
		imageStorage: &purgeStorage{failKeys: map[string]bool{}},
	}

	deleted, err := service.PurgeOldProofImages(context.Background(), time.Now())
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if deleted != 0 {
		t.Fatalf("deleted = %d, want 0", deleted)
	}
}
