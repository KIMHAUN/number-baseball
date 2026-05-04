package ranking

import (
	"context"
	"number-baseball/internal/model"
	"strconv"

	"github.com/redis/go-redis/v9"
)

const rankingKey = "ranking:season"

type Ranking struct {
	RDB *redis.Client
}

func New(rdb *redis.Client) *Ranking {
	return &Ranking{RDB: rdb}
}

func (r *Ranking) AddWin(userID string) {
	ctx := context.Background()
	r.RDB.ZIncrBy(ctx, rankingKey, 10, userID) // 승리 +10점
}

func (r *Ranking) AddLoss(userID string) {
	ctx := context.Background()
	r.RDB.ZIncrBy(ctx, rankingKey, -5, userID) // 패배 -5점
}

func (r *Ranking) GetTop(n int) ([]model.RankEntry, error) {
	ctx := context.Background()
	results, err := r.RDB.ZRevRangeWithScores(ctx, rankingKey, 0, int64(n-1)).Result()
	if err != nil {
		return nil, err
	}

	entries := make([]model.RankEntry, len(results))
	for i, z := range results {
		entries[i] = model.RankEntry{
			Rank:   i + 1,
			UserID: z.Member.(string),
			Score:  int(z.Score),
		}
	}
	return entries, nil
}

func (r *Ranking) GetUserRank(userID string) (*model.RankEntry, error) {
	ctx := context.Background()
	rank, err := r.RDB.ZRevRank(ctx, rankingKey, userID).Result()
	if err != nil {
		return nil, err
	}
	score, err := r.RDB.ZScore(ctx, rankingKey, userID).Result()
	if err != nil {
		return nil, err
	}
	return &model.RankEntry{
		Rank:   int(rank) + 1,
		UserID: userID,
		Score:  int(score),
	}, nil
}

func (r *Ranking) Reset() {
	ctx := context.Background()
	r.RDB.Del(ctx, rankingKey)
}

// 시즌 키 분리 (시즌별 랭킹 보존)
func seasonKey(seasonID int) string {
	return "ranking:season:" + strconv.Itoa(seasonID)
}
