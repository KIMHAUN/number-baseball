package game

import (
	"encoding/json"
	"log"
	"math/rand"
	"number-baseball/internal/model"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type Player struct {
	ID     string
	Conn   *websocket.Conn
	Secret string // 상대가 맞춰야 할 숫자
	mu     sync.Mutex
}

func (p *Player) Send(msg model.WSMessage) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.Conn == nil {
		return
	}
	data, _ := json.Marshal(msg)
	p.Conn.WriteMessage(websocket.TextMessage, data)
}

type Session struct {
	ID        string
	Players   [2]*Player
	Config    model.GameConfig
	Turn      int // 0 = player[0]의 턴, 1 = player[1]의 턴
	TurnCount int
	StartedAt time.Time
	guessCh   [2]chan string
	done      chan model.GameOverPayload
	mu        sync.RWMutex
}

func NewSession(p1, p2 *Player, cfg model.GameConfig) *Session {
	p1.Secret = generateSecret(cfg.Digits)
	p2.Secret = generateSecret(cfg.Digits)

	s := &Session{
		ID:        uuid.New().String(),
		Players:   [2]*Player{p1, p2},
		Config:    cfg,
		Turn:      0,
		TurnCount: 0,
		StartedAt: time.Now(),
		done:      make(chan model.GameOverPayload, 1),
	}
	s.guessCh[0] = make(chan string, 1)
	s.guessCh[1] = make(chan string, 1)
	return s
}

func (s *Session) Run() model.GameOverPayload {
	// 게임 시작 알림
	s.broadcast(model.WSMessage{Type: "game_start", Payload: map[string]any{
		"session_id": s.ID,
		"config":     s.Config,
		"your_index": 0,
	}}, 0)
	s.broadcast(model.WSMessage{Type: "game_start", Payload: map[string]any{
		"session_id": s.ID,
		"config":     s.Config,
		"your_index": 1,
	}}, 1)

	for s.TurnCount < s.Config.MaxTurns*2 {
		current := s.Turn
		opponent := 1 - current

		// 턴 시작 알림
		s.Players[current].Send(model.WSMessage{Type: "your_turn", Payload: map[string]int{"turn": s.TurnCount/2 + 1}})
		s.Players[opponent].Send(model.WSMessage{Type: "wait_turn"})

		// 턴 타임아웃 대기
		var guess string
		select {
		case g := <-s.guessCh[current]:
			guess = g
		case <-time.After(s.Config.TurnTime):
			return s.endGame(s.Players[opponent].ID, s.Players[current].ID, "timeout")
		}

		// 판정
		strike, ball, out := judge(guess, s.Players[opponent].Secret, s.Config.Digits)
		result := model.GuessResult{
			Turn:   s.TurnCount/2 + 1,
			Guess:  guess,
			Strike: strike,
			Ball:   ball,
			Out:    out,
		}

		s.Players[current].Send(model.WSMessage{Type: "guess_result", Payload: result})
		s.Players[opponent].Send(model.WSMessage{Type: "opponent_guess", Payload: result})

		// 3스트라이크 = 승리
		if strike == s.Config.Digits {
			return s.endGame(s.Players[current].ID, s.Players[opponent].ID, "3strike")
		}

		s.TurnCount++
		s.Turn = opponent
	}

	// 최대 턴 초과 - 무승부 처리 (선공 패배)
	return s.endGame(s.Players[1].ID, s.Players[0].ID, "max_turns")
}

func (s *Session) SubmitGuess(playerIdx int, guess string) {
	select {
	case s.guessCh[playerIdx] <- guess:
	default:
	}
}

// 재접속 처리
func (s *Session) Reconnect(playerIdx int, conn *websocket.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Players[playerIdx].mu.Lock()
	s.Players[playerIdx].Conn = conn
	s.Players[playerIdx].mu.Unlock()

	s.Players[playerIdx].Send(model.WSMessage{Type: "reconnected", Payload: map[string]any{
		"session_id": s.ID,
		"turn":       s.TurnCount/2 + 1,
		"your_turn":  s.Turn == playerIdx,
	}})
	log.Printf("[session %s] player %d reconnected", s.ID, playerIdx)
}

func (s *Session) endGame(winnerID, loserID, reason string) model.GameOverPayload {
	result := model.GameOverPayload{
		Winner:   winnerID,
		Loser:    loserID,
		Reason:   reason,
		Turns:    s.TurnCount/2 + 1,
		Duration: time.Duration(time.Since(s.StartedAt).Milliseconds()),
	}
	s.broadcast(model.WSMessage{Type: "game_over", Payload: result}, 0)
	s.broadcast(model.WSMessage{Type: "game_over", Payload: result}, 1)
	return result
}

func (s *Session) broadcast(msg model.WSMessage, playerIdx int) {
	s.Players[playerIdx].Send(msg)
}

// 스트라이크/볼/아웃 판정
func judge(guess, secret string, digits int) (strike, ball, out int) {
	for i := 0; i < digits; i++ {
		if guess[i] == secret[i] {
			strike++
		} else {
			found := false
			for j := 0; j < digits; j++ {
				if guess[i] == secret[j] {
					ball++
					found = true
					break
				}
			}
			if !found {
				out++
			}
		}
	}
	return
}

// 중복 없는 랜덤 숫자 생성
func generateSecret(digits int) string {
	nums := []byte("0123456789")
	rand.Shuffle(len(nums), func(i, j int) { nums[i], nums[j] = nums[j], nums[i] })
	// 첫 자리 0 방지
	if nums[0] == '0' {
		nums[0], nums[1] = nums[1], nums[0]
	}
	return string(nums[:digits])
}
