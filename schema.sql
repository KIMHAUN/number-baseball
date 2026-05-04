CREATE DATABASE IF NOT EXISTS number_baseball DEFAULT CHARACTER SET utf8mb4;
USE number_baseball;

CREATE TABLE matches (
    id          BIGINT AUTO_INCREMENT PRIMARY KEY,
    winner_id   VARCHAR(255) NOT NULL,
    loser_id    VARCHAR(255) NOT NULL,
    turns       INT          NOT NULL,
    reason      VARCHAR(50)  NOT NULL,
    duration_ms BIGINT       NOT NULL,
    season_id   INT          DEFAULT 1,
    created_at  TIMESTAMP    DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_winner (winner_id, created_at DESC),
    INDEX idx_loser (loser_id, created_at DESC),
    INDEX idx_season (season_id)
) ENGINE=InnoDB;

CREATE TABLE game_config (
    id         INT PRIMARY KEY DEFAULT 1,
    digits     INT DEFAULT 3,
    max_turns  INT DEFAULT 10,
    turn_time  INT DEFAULT 30,
    season_id  INT DEFAULT 1,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB;

INSERT INTO game_config (id) VALUES (1);
