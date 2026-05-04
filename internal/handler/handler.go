package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"number-baseball/internal/match"
	"number-baseball/internal/model"
	"number-baseball/internal/ranking"
	"number-baseball/internal/repository"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Handler struct {
	Matcher *match.Matcher
	Repo    *repository.Repo
	Ranking *ranking.Ranking
}

func New(m *match.Matcher, repo *repository.Repo, rank *ranking.Ranking) *Handler {
	return &Handler{Matcher: m, Repo: repo, Ranking: rank}
}

func (h *Handler) Register(r *gin.Engine) {
	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
	r.Static("/web", "./web")
	r.GET("/ws", h.WebSocket)
	r.GET("/api/ranking", h.GetRanking)
	r.GET("/api/ranking/:user_id", h.GetUserRank)
	r.GET("/api/matches/:user_id", h.GetMatchHistory)
}

// WebSocket - 매칭 + 게임 통신
func (h *Handler) WebSocket(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(400, gin.H{"error": "user_id required"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[ws] upgrade error: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("[ws] %s connected", userID)

	// 매칭 큐에 등록
	h.Matcher.Enqueue(userID, conn)

	// 메시지 수신 루프
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			log.Printf("[ws] %s disconnected: %v", userID, err)
			h.Matcher.CancelQueue(userID)
			return
		}

		var msg model.WSMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case "set_secret":
			sess, idx := h.Matcher.GetSession(userID)
			if sess == nil {
				continue
			}
			payloadBytes, _ := json.Marshal(msg.Payload)
			var req model.GuessRequest
			json.Unmarshal(payloadBytes, &req)
			sess.SubmitSecret(idx, req.Guess)

		case "guess":
			sess, idx := h.Matcher.GetSession(userID)
			if sess == nil {
				continue
			}
			payloadBytes, _ := json.Marshal(msg.Payload)
			var req model.GuessRequest
			json.Unmarshal(payloadBytes, &req)
			sess.SubmitGuess(idx, req.Guess)

		case "cancel":
			h.Matcher.CancelQueue(userID)
		}
	}
}

// REST - 랭킹 Top N
func (h *Handler) GetRanking(c *gin.Context) {
	n, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	entries, err := h.Ranking.GetTop(n)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true, "ranking": entries})
}

// REST - 유저 랭킹
func (h *Handler) GetUserRank(c *gin.Context) {
	entry, err := h.Ranking.GetUserRank(c.Param("user_id"))
	if err != nil {
		c.JSON(404, gin.H{"error": "not ranked"})
		return
	}
	c.JSON(200, gin.H{"ok": true, "rank": entry})
}

// REST - 매치 이력
func (h *Handler) GetMatchHistory(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	records, err := h.Repo.GetMatchHistory(c.Param("user_id"), limit)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true, "matches": records})
}
