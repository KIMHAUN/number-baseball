package repository

import (
	"database/sql"
	"number-baseball/internal/model"
)

type Repo struct {
	DB *sql.DB
}

func New(db *sql.DB) *Repo {
	return &Repo{DB: db}
}

func (r *Repo) SaveMatch(m model.GameOverPayload, seasonID int) error {
	_, err := r.DB.Exec(
		`INSERT INTO matches (winner_id, loser_id, turns, reason, duration_ms, season_id)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		m.Winner, m.Loser, m.Turns, m.Reason, int64(m.Duration), seasonID,
	)
	return err
}

func (r *Repo) GetMatchHistory(userID string, limit int) ([]model.MatchRecord, error) {
	rows, err := r.DB.Query(
		`SELECT id, winner_id, loser_id, turns, reason, duration_ms, season_id, created_at
		 FROM matches WHERE winner_id = ? OR loser_id = ?
		 ORDER BY created_at DESC LIMIT ?`,
		userID, userID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []model.MatchRecord
	for rows.Next() {
		var m model.MatchRecord
		if err := rows.Scan(&m.ID, &m.WinnerID, &m.LoserID, &m.Turns, &m.Reason, &m.DurationMs, &m.SeasonID, &m.CreatedAt); err != nil {
			return nil, err
		}
		records = append(records, m)
	}
	return records, nil
}

func (r *Repo) GetWinLoss(userID string) (wins, losses int, err error) {
	err = r.DB.QueryRow("SELECT COUNT(*) FROM matches WHERE winner_id = ?", userID).Scan(&wins)
	if err != nil {
		return
	}
	err = r.DB.QueryRow("SELECT COUNT(*) FROM matches WHERE loser_id = ?", userID).Scan(&losses)
	return
}

func (r *Repo) ResetSeason(newSeasonID int) error {
	_, err := r.DB.Exec("UPDATE game_config SET season_id = ? WHERE id = 1", newSeasonID)
	return err
}
