package challenge

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// proofPurgeBatchSize caps how many rows are read per query so a large backlog
// is processed in chunks instead of one big load.
const proofPurgeBatchSize = 100

// maxReportedPurgeFailures keeps a bad day at the storage provider from writing
// an unbounded error list to the log.
const maxReportedPurgeFailures = 5

// PurgeOldProofImages deletes proof images for submissions approved before
// cutoff, then clears the stored reference so the API stops advertising a URL
// that no longer resolves. A row whose file could not be deleted keeps its
// reference and is retried on the next run.
func (s *service) PurgeOldProofImages(ctx context.Context, cutoff time.Time) (int, error) {
	deleted := 0
	var failures []error

	for {
		submissions, err := s.repo.FindProofImagesToPurge(cutoff, proofPurgeBatchSize)
		if err != nil {
			return deleted, err
		}
		if len(submissions) == 0 {
			break
		}

		deletedBefore := deleted
		for _, submission := range submissions {
			if submission.ProofImageID == nil {
				continue
			}

			imageID := *submission.ProofImageID
			key := fmt.Sprintf("submissions/%s", imageID.String())

			if err := s.imageStorage.DeleteFile(ctx, key); err != nil {
				failures = appendPurgeFailure(failures, fmt.Errorf("delete %s: %w", key, err))
				continue
			}

			if err := s.repo.ClearProofImage(submission.ID, imageID); err != nil {
				failures = appendPurgeFailure(failures, fmt.Errorf("clear image for submission %s: %w", submission.ID, err))
				continue
			}

			deleted++
		}

		// If a whole batch failed, the next query would return the same rows
		// forever. Leave them for the next run instead.
		if deleted == deletedBefore {
			break
		}
	}

	return deleted, errors.Join(failures...)
}

func appendPurgeFailure(failures []error, err error) []error {
	if len(failures) >= maxReportedPurgeFailures {
		return failures
	}
	return append(failures, err)
}
