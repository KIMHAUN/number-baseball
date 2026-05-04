package model

import "time"

// 컨텐츠 데이터 설계 - DB/JSON으로 관리 가능한 게임 룰
type GameConfig struct {
	Digits    int           `json:"digits"`     // 숫자 자릿수 (기본 3)
	MaxTurns  int           `json:"max_turns"`  // 최대 턴 수 (기본 10)
	TurnTime  time.Duration `json:"turn_time"`  // 턴 제한 시간 (기본 30초)
	SeasonID  int           `json:"season_id"`
}

var DefaultConfig = GameConfig{
	Digits:   3,
	MaxTurns: 10,
	TurnTime: 30 * time.Second,
	SeasonID: 1,
}

// WebSocket 메시지 프로토콜
type WSMessage struct {
	Type    string `json:"type"`
	Payload any    `json:"payload,omitempty"`
}

// 클라이언트 → 서버
type GuessRequest struct {
	Guess string `json:"guess"`
}

// 서버 → 클라이언트
type GuessResult struct {
	Turn    int    `json:"turn"`
	Guess   string `json:"guess"`
	Strike  int    `json:"strike"`
	Ball    int    `json:"ball"`
	Out     int    `json:"out"`
}

type GameOverPayload struct {
	Winner   string        `json:"winner"`
	Loser    string        `json:"loser"`
	Reason   string        `json:"reason"` // "3strike", "max_turns", "disconnect", "timeout"
	Turns    int           `json:"turns"`
	Duration time.Duration `json:"duration_ms"`
}

// DB 저장용 매치 이력
type MatchRecord struct {
	ID         int64     `json:"id"`
	WinnerID   string    `json:"winner_id"`
	LoserID    string    `json:"loser_id"`
	Turns      int       `json:"turns"`
	Reason     string    `json:"reason"`
	DurationMs int64     `json:"duration_ms"`
	SeasonID   int       `json:"season_id"`
	CreatedAt  time.Time `json:"created_at"`
}

// 랭킹
type RankEntry struct {
	Rank   int    `json:"rank"`
	UserID string `json:"user_id"`
	Score  int    `json:"score"`
	Wins   int    `json:"wins"`
	Losses int    `json:"losses"`
}
