package store

import (
	"database/sql"
	"strings"
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
)

// findingReviewTTL is the local triage window. After it passes the review is
// ignored and the finding is shown as unreviewed again.
const findingReviewTTL = 30 * 24 * time.Hour

// ValidFindingReviewStatuses are the decisions a reviewer can record.
var ValidFindingReviewStatuses = map[string]bool{
	"pending":        true,
	"confirmed":      true,
	"false_positive": true,
	"resolved":       true,
}

// NormalizeFindingKey keeps review keys stable across reimports and scans.
func NormalizeFindingKey(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func normalizeReviewSiteID(siteID int64) int64 {
	if siteID < 1 {
		return 1
	}
	return siteID
}

// UpsertFindingReview stores or renews a human decision for a finding.
func (s *Store) UpsertFindingReview(r *model.FindingReview) error {
	if r == nil {
		return sql.ErrNoRows
	}
	r.SiteID = normalizeReviewSiteID(r.SiteID)
	r.FindingType = strings.TrimSpace(r.FindingType)
	r.HostKey = NormalizeFindingKey(r.HostKey)
	r.Status = strings.ToLower(strings.TrimSpace(r.Status))
	r.Note = strings.TrimSpace(r.Note)
	r.Reviewer = strings.TrimSpace(r.Reviewer)
	now := time.Now().UTC()
	r.UpdatedAt = now
	if r.ExpiresAt.IsZero() || !r.ExpiresAt.After(now) {
		r.ExpiresAt = now.Add(findingReviewTTL)
	}
	if previous, err := s.GetFindingReview(r.SiteID, r.FindingType, r.HostKey); err == nil {
		r.CreatedAt = previous.CreatedAt
	} else {
		r.CreatedAt = now
	}
	_, err := s.db.Exec(`INSERT INTO finding_reviews
		(site_id, finding_type, host_key, status, note, reviewer, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(site_id, finding_type, host_key) DO UPDATE SET
			status=excluded.status, note=excluded.note, reviewer=excluded.reviewer,
			expires_at=excluded.expires_at, updated_at=excluded.updated_at`,
		r.SiteID, r.FindingType, r.HostKey, r.Status, r.Note, r.Reviewer,
		r.ExpiresAt.UnixMilli(), r.CreatedAt.UnixMilli(), r.UpdatedAt.UnixMilli())
	return err
}

// GetFindingReview returns an active review or ErrNotFound.
func (s *Store) GetFindingReview(siteID int64, findingType, hostKey string) (*model.FindingReview, error) {
	siteID = normalizeReviewSiteID(siteID)
	findingType = strings.TrimSpace(findingType)
	hostKey = NormalizeFindingKey(hostKey)
	var r model.FindingReview
	var expires, created, updated int64
	err := s.db.QueryRow(`SELECT site_id, finding_type, host_key, status, note, reviewer,
		expires_at, created_at, updated_at FROM finding_reviews
		WHERE site_id=? AND finding_type=? AND host_key=? AND expires_at>?`,
		siteID, findingType, hostKey, nowMS()).Scan(
		&r.SiteID, &r.FindingType, &r.HostKey, &r.Status, &r.Note, &r.Reviewer,
		&expires, &created, &updated)
	if err != nil {
		return nil, err
	}
	r.ExpiresAt = fromMS(expires)
	r.CreatedAt = fromMS(created)
	r.UpdatedAt = fromMS(updated)
	return &r, nil
}

// ListFindingReviews returns active reviews, optionally filtered by status.
func (s *Store) ListFindingReviews(siteID int64, status string, limit int) ([]model.FindingReview, error) {
	if limit < 1 || limit > 500 {
		limit = 200
	}
	query := `SELECT site_id, finding_type, host_key, status, note, reviewer,
		expires_at, created_at, updated_at FROM finding_reviews WHERE expires_at>?`
	args := []any{nowMS()}
	// Unlike single-review operations, listing has no implicit default site:
	// 0 means all sites so the console can show every local decision.
	if siteID > 0 {
		query += ` AND site_id=?`
		args = append(args, siteID)
	}
	if status = strings.ToLower(strings.TrimSpace(status)); status != "" {
		query += ` AND status=?`
		args = append(args, status)
	}
	query += ` ORDER BY updated_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []model.FindingReview{}
	for rows.Next() {
		var r model.FindingReview
		var expires, created, updated int64
		if err := rows.Scan(&r.SiteID, &r.FindingType, &r.HostKey, &r.Status, &r.Note, &r.Reviewer,
			&expires, &created, &updated); err != nil {
			return nil, err
		}
		r.ExpiresAt = fromMS(expires)
		r.CreatedAt = fromMS(created)
		r.UpdatedAt = fromMS(updated)
		items = append(items, r)
	}
	return items, rows.Err()
}

// DeleteFindingReview removes a decision so the finding returns to unreviewed.
func (s *Store) DeleteFindingReview(siteID int64, findingType, hostKey string) error {
	_, err := s.db.Exec(`DELETE FROM finding_reviews WHERE site_id=? AND finding_type=? AND host_key=?`,
		normalizeReviewSiteID(siteID), strings.TrimSpace(findingType), NormalizeFindingKey(hostKey))
	return err
}

// PurgeExpiredFindingReviews removes stale decisions from the database.
func (s *Store) PurgeExpiredFindingReviews() error {
	_, err := s.db.Exec(`DELETE FROM finding_reviews WHERE expires_at<=?`, nowMS())
	return err
}
