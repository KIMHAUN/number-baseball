package match

import (
	"context"
	"log"
	"number-baseball/internal/game"
	"number-baseball/internal/model"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

const matchQueue = "match:queue"

type WaitingPlayer struct {
	ID   string
	Conn *websocket.Conn
}

type Matcher struct {
	rdb      *redis.Client
	waiting  map[string]*WaitingPlayer // userID → waiting player
	sessions map[string]*game.Session  // sessionID → session
	playerSession map[string]string    // userID → sessionID (재접속용)
	onGameOver func(model.GameOverPayload)
	mu       sync.RWMutex
}

func NewMatcher(rdb *redis.Client, onGameOver func(model.GameOverPayload)) *Matcher {
	return &Matcher{
		rdb:           rdb,
		waiting:       make(map[string]*WaitingPlayer),
		sessions:      make(map[string]*game.Session),
		playerSession: make(map[string]string),
		onGameOver:    onGameOver,
	}
}

func (m *Matcher) Enqueue(userID string, conn *websocket.Conn) {
	m.mu.Lock()

	// 이미 게임 중이면 재접속 처리
	if sessID, ok := m.playerSession[userID]; ok {
		if sess, exists := m.sessions[sessID]; exists {
			m.mu.Unlock()
			for i, p := range sess.Players {
				if p.ID == userID {
					sess.Reconnect(i, conn)
					return
				}
			}
			return
		}
	}

	m.waiting[userID] = &WaitingPlayer{ID: userID, Conn: conn}
	m.mu.Unlock()

	ctx := context.Background()
	m.rdb.RPush(ctx, matchQueue, userID)

	log.Printf("[match] %s enqueued (queue size: %d)", userID, m.rdb.LLen(ctx, matchQueue).Val())
	m.tryMatch()
}

func (m *Matcher) tryMatch() {
	ctx := context.Background()

	for {
		qLen := m.rdb.LLen(ctx, matchQueue).Val()
		if qLen < 2 {
			return
		}

		id1 := m.rdb.LPop(ctx, matchQueue).Val()
		id2 := m.rdb.LPop(ctx, matchQueue).Val()

		if id1 == "" || id2 == "" {
			return
		}

		m.mu.Lock()
		w1, ok1 := m.waiting[id1]
		w2, ok2 := m.waiting[id2]

		if !ok1 || !ok2 {
			m.mu.Unlock()
			continue
		}

		delete(m.waiting, id1)
		delete(m.waiting, id2)

		p1 := &game.Player{ID: id1, Conn: w1.Conn}
		p2 := &game.Player{ID: id2, Conn: w2.Conn}
		sess := game.NewSession(p1, p2, model.DefaultConfig)

		m.sessions[sess.ID] = sess
		m.playerSession[id1] = sess.ID
		m.playerSession[id2] = sess.ID
		m.mu.Unlock()

		log.Printf("[match] matched %s vs %s → session %s", id1, id2, sess.ID)

		// 게임 세션을 goroutine으로 실행
		go func() {
			result := sess.Run()

			m.mu.Lock()
			delete(m.sessions, sess.ID)
			delete(m.playerSession, id1)
			delete(m.playerSession, id2)
			m.mu.Unlock()

			log.Printf("[match] session %s ended: %s beat %s (%s)", sess.ID, result.Winner, result.Loser, result.Reason)

			if m.onGameOver != nil {
				m.onGameOver(result)
			}
		}()
	}
}

func (m *Matcher) GetSession(userID string) (*game.Session, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessID, ok := m.playerSession[userID]
	if !ok {
		return nil, -1
	}
	sess, exists := m.sessions[sessID]
	if !exists {
		return nil, -1
	}
	for i, p := range sess.Players {
		if p.ID == userID {
			return sess, i
		}
	}
	return nil, -1
}

func (m *Matcher) CancelQueue(userID string) {
	ctx := context.Background()
	m.rdb.LRem(ctx, matchQueue, 1, userID)

	m.mu.Lock()
	delete(m.waiting, userID)
	m.mu.Unlock()
}

// 시즌 리셋 스케줄러 (업무 자동화)
func (m *Matcher) StartSeasonResetScheduler(resetFunc func()) {
	go func() {
		for {
			now := time.Now()
			// 매주 월요일 00:00에 리셋
			next := now.Truncate(24 * time.Hour)
			for next.Weekday() != time.Monday || next.Before(now) {
				next = next.Add(24 * time.Hour)
			}
			time.Sleep(time.Until(next))
			log.Println("[scheduler] season reset triggered")
			resetFunc()
		}
	}()
}
