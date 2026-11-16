package jobs

import (
	"context"
	"log"
	"time"
)

// ProofImageCleaner deletes stored proof images for submissions approved
// before the cutoff. The challenge service implements it.
type ProofImageCleaner interface {
	PurgeOldProofImages(ctx context.Context, cutoff time.Time) (int, error)
}

// proofCleanupInterval is how often the cleanup runs while the process stays
// alive. It also runs once on startup, so a restarted or freshly scaled
// instance catches up without waiting a full day.
const proofCleanupInterval = 24 * time.Hour

// RunProofImageCleanup deletes proof images of submissions approved more than a
// month ago, then repeats daily until ctx is cancelled.
//
// It is safe to run on several instances at once: files are deleted by key, and
// a row is only cleared after its file is gone, so a duplicate run either finds
// nothing to do or deletes an already-missing object harmlessly.
func RunProofImageCleanup(ctx context.Context, cleaner ProofImageCleaner) {
	run := func() {
		// AddDate keeps the cutoff on the same calendar day one month back,
		// which matches how "older than a month" is read.
		cutoff := time.Now().AddDate(0, -1, 0)

		deleted, err := cleaner.PurgeOldProofImages(ctx, cutoff)
		switch {
		case err != nil:
			log.Printf("proof image cleanup: deleted %d images before failing: %v", deleted, err)
		case deleted > 0:
			log.Printf("proof image cleanup: deleted %d images approved before %s", deleted, cutoff.Format(time.RFC3339))
		}
	}

	run()

	ticker := time.NewTicker(proofCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
