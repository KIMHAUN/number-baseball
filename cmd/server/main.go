package main

import (
	"database/sql"
	"log"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"number-baseball/internal/handler"
	"number-baseball/internal/match"
	"number-baseball/internal/model"
	"number-baseball/internal/ranking"
	"number-baseball/internal/repository"
)

func main() {
	// MySQL
	dsn := env("DB_DSN", "root:root@tcp(127.0.0.1:3306)/number_baseball?parseTime=true")
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)

	// MySQL 준비 대기 (최대 30초)
	for i := 0; i < 30; i++ {
		if err := db.Ping(); err == nil {
			break
		}
		log.Printf("waiting for MySQL... (%d/30)", i+1)
		time.Sleep(1 * time.Second)
	}
	if err := db.Ping(); err != nil {
		log.Fatalf("db ping: %v", err)
	}
	log.Println("MySQL connected")

	// Redis
	rdb := redis.NewClient(&redis.Options{
		Addr: env("REDIS_ADDR", "127.0.0.1:6379"),
	})

	repo := repository.New(db)
	rank := ranking.New(rdb)

	// 게임 종료 콜백: 매치 저장 + 랭킹 업데이트
	onGameOver := func(result model.GameOverPayload) {
		if err := repo.SaveMatch(result, model.DefaultConfig.SeasonID); err != nil {
			log.Printf("[error] save match: %v", err)
		}
		rank.AddWin(result.Winner)
		rank.AddLoss(result.Loser)
		log.Printf("[game] saved: %s > %s (%s, %d turns)", result.Winner, result.Loser, result.Reason, result.Turns)
	}

	matcher := match.NewMatcher(rdb, onGameOver)

	// 시즌 리셋 스케줄러 (업무 자동화)
	matcher.StartSeasonResetScheduler(func() {
		rank.Reset()
		log.Println("[scheduler] ranking reset complete")
	})

	h := handler.New(matcher, repo, rank)

	r := gin.Default()
	h.Register(r)

	port := env("PORT", "8082")
	log.Printf("number-baseball server on :%s", port)
	r.Run(":" + port)
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
