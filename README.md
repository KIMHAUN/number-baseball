# ⚾ 숫자 야구 온라인 대전 서버

WebSocket 기반 실시간 숫자 야구 대전 게임 서버입니다.

> 모바일 게임 서버의 핵심 패턴(매칭, 세션 관리, 실시간 통신, 랭킹)을
> Go의 동시성 모델(goroutine + channel)로 구현했습니다.

---

## 아키텍처

```
브라우저 / 모바일 클라이언트
  │
  ├── WebSocket ──→ [게임 서버 :8082]
  │                    │
  │                    ├── 매칭 큐 (Redis List)
  │                    │     └── 2명 모이면 → 게임 세션 생성
  │                    │
  │                    ├── 게임 세션 (goroutine per game)
  │                    │     ├── 턴 처리 루프
  │                    │     ├── 타임아웃 (channel + select)
  │                    │     └── 재접속 지원
  │                    │
  │                    ├── 랭킹 (Redis Sorted Set)
  │                    │     └── 승리 +10 / 패배 -5
  │                    │
  │                    └── 매치 이력 (MySQL)
  │
  ├── HTTP GET ──→ /api/ranking         랭킹 조회
  ├── HTTP GET ──→ /api/ranking/:id     개인 랭킹
  └── HTTP GET ──→ /api/matches/:id     매치 이력
```

---

## 게임 서버 핵심 설계

### 1. 매칭 시스템

```
Player A ──→ Redis RPUSH "match:queue"
Player B ──→ Redis RPUSH "match:queue"
                    │
              큐 길이 ≥ 2 감지
                    │
              LPOP × 2 → 매칭 완료
                    │
              goroutine으로 게임 세션 시작
```

### 2. 게임 세션 (goroutine per game)

```go
// 각 게임이 독립된 goroutine에서 실행
go func() {
    result := session.Run()  // 게임 루프
    saveMatch(result)        // DB 저장
    updateRanking(result)    // 랭킹 반영
}()
```

- 세션마다 독립된 goroutine → 수천 게임 동시 진행 가능
- channel + select로 턴 타임아웃 처리
- 연결 끊김 시 세션 유지 → 재접속 가능

### 3. 턴 처리 흐름

```
서버                          Player A              Player B
 │                               │                     │
 ├── your_turn ────────────────→ │                     │
 ├── wait_turn ──────────────────┼───────────────────→ │
 │                               │                     │
 │ ←── guess: "123" ─────────── │                     │
 │                               │                     │
 │   judge("123", secret) → 1S 1B 1O                  │
 │                               │                     │
 ├── guess_result: 1S1B1O ─────→ │                     │
 ├── opponent_guess: 1S1B1O ─────┼───────────────────→ │
 │                               │                     │
 │   턴 교대                      │                     │
 │                               │                     │
 ├── wait_turn ────────────────→ │                     │
 ├── your_turn ──────────────────┼───────────────────→ │
```

### 4. 타임아웃 처리 (channel + select)

```go
select {
case guess := <-guessCh:    // 정상 입력
    judge(guess)
case <-time.After(30s):     // 타임아웃 → 상대 승리
    endGame(opponent, "timeout")
}
```

### 5. 재접속 처리 (라이브 유지보수)

```
Player A 연결 끊김
  → 세션은 유지 (goroutine 살아있음)
  → 상대는 "wait_turn" 상태로 대기

Player A 재접속
  → userID로 기존 세션 탐색
  → WebSocket 교체
  → 현재 턴 상태 전송 → 게임 이어서 진행
```

### 6. 컨텐츠 데이터 설계

게임 룰을 DB 테이블(`game_config`)로 관리하여 서버 재시작 없이 변경 가능:

| 설정 | 기본값 | 설명 |
|------|--------|------|
| `digits` | 3 | 숫자 자릿수 |
| `max_turns` | 10 | 최대 턴 수 |
| `turn_time` | 30초 | 턴 제한 시간 |
| `season_id` | 1 | 현재 시즌 |

### 7. 업무 자동화

- 시즌 리셋 스케줄러: 매주 월요일 00:00 자동 실행
- 랭킹 초기화 + 시즌 ID 증가

---

## 기술 스택

| 기술 | 역할 | 선택 이유 |
|------|------|-----------|
| **Go** (Gin) | 게임 서버 | goroutine으로 세션당 독립 실행, channel로 턴 동기화 |
| **gorilla/websocket** | 실시간 통신 | 양방향 메시지, 서버 주도 이벤트 전송 |
| **Redis** | 매칭 큐 + 랭킹 | List(매칭), Sorted Set(랭킹) |
| **MySQL** | 매치 이력 | 영속 데이터, 통계 쿼리 |
| **Docker Compose** | 환경 구성 | 원커맨드 실행 |

---

## DB 스키마

```sql
-- 매치 이력
matches (
    id, winner_id, loser_id, turns, reason, duration_ms, season_id, created_at
)

-- 게임 설정 (컨텐츠 데이터)
game_config (
    id, digits, max_turns, turn_time, season_id, updated_at
)
```

---

## API 명세

### WebSocket (`ws://localhost:8082/ws?user_id=xxx`)

| 방향 | type | 설명 |
|------|------|------|
| ← 서버 | `game_start` | 매칭 완료, 게임 설정 전달 |
| ← 서버 | `your_turn` | 내 턴 시작 |
| ← 서버 | `wait_turn` | 상대 턴 대기 |
| → 클라 | `guess` | 숫자 제출 `{"guess":"123"}` |
| ← 서버 | `guess_result` | 내 추측 결과 (S/B/O) |
| ← 서버 | `opponent_guess` | 상대 추측 결과 |
| ← 서버 | `game_over` | 게임 종료 (승자, 사유, 턴 수) |
| ← 서버 | `reconnected` | 재접속 성공 + 현재 상태 |
| → 클라 | `cancel` | 매칭 취소 |

### REST API

| Method | Endpoint | 설명 |
|--------|----------|------|
| `GET` | `/api/ranking?limit=20` | 랭킹 Top N |
| `GET` | `/api/ranking/{user_id}` | 개인 랭킹 |
| `GET` | `/api/matches/{user_id}?limit=20` | 매치 이력 |

---

## 실행 방법

```bash
git clone <repo-url>
cd number-baseball
docker-compose up --build
```

| 서비스 | URL |
|--------|-----|
| 게임 서버 | `http://localhost:8082` |
| 웹 클라이언트 | `http://localhost:8082/web/index.html` |
| MySQL | `localhost:3307` |
| Redis | `localhost:6380` |

### 테스트 방법

1. 브라우저 탭 2개에서 `http://localhost:8082/web/index.html` 접속
2. 각각 닉네임 입력 → 접속
3. 자동 매칭 → 대전 시작
4. 턴마다 3자리 숫자 입력 → 스트라이크/볼/아웃 확인
5. 3스트라이크 또는 10턴 초과 시 게임 종료
6. 랭킹 보기 버튼으로 점수 확인

---

## 프로젝트 구조

```
number-baseball/
├── cmd/server/main.go           엔트리포인트 (컴포넌트 조립)
├── internal/
│   ├── game/session.go          게임 세션 (턴 루프, 판정, 재접속)
│   ├── match/matcher.go         매칭 시스템 (Redis 큐, 시즌 스케줄러)
│   ├── handler/handler.go       HTTP + WebSocket 핸들러
│   ├── repository/repo.go       MySQL 쿼리
│   ├── ranking/ranking.go       Redis Sorted Set 랭킹
│   └── model/model.go           데이터 모델 + 게임 설정
├── web/index.html               테스트용 웹 클라이언트
├── schema.sql                   MySQL DDL
├── docker-compose.yml           원커맨드 실행
└── Dockerfile                   멀티스테이지 빌드
```
