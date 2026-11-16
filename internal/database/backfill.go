package database

import "gorm.io/gorm"

// BackfillSnapshots fills the snapshot columns that were added after some rows
// were already written. Each statement only touches rows whose snapshot is
// still empty, so running this on every boot is cheap and idempotent.
func BackfillSnapshots(db *gorm.DB) error {
	statements := []string{
		`UPDATE reward_claims
		 SET reward_title = rewards.title
		 FROM rewards
		 WHERE rewards.id = reward_claims.reward_id
		   AND reward_claims.reward_title = ''`,
		`UPDATE challenge_submissions
		 SET challenge_title = challenges.title, points = challenges.points
		 FROM challenges
		 WHERE challenges.id = challenge_submissions.challenge_id
		   AND challenge_submissions.challenge_title = ''`,
	}

	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}

	return nil
}
