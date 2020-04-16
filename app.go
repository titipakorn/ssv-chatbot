package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"strconv"

	"github.com/go-redis/redis/v7"
	_ "github.com/lib/pq"
	"github.com/line/line-bot-sdk-go/linebot"
)

// HailingApp app
type HailingApp struct {
	bot         *linebot.Client
	rdb         *redis.Client
	pdb         *sql.DB
	appBaseURL  string
	downloadDir string
}

// NewHailingApp function
func NewHailingApp(channelSecret, channelToken, appBaseURL string) (*HailingApp, error) {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	redisPassword := os.Getenv("REDIS_PASSWORD")
	redisDB, _ := strconv.Atoi(os.Getenv("REDIS_DB"))
	postgresURI := os.Getenv("POSTGRES_URI")
	if postgresURI == "" {
		postgresURI = "postgres://sipp11:banshee10@localhost/hailing?sslmode=verify-full"
	}
	apiEndpointBase := os.Getenv("ENDPOINT_BASE")
	if apiEndpointBase == "" {
		apiEndpointBase = linebot.APIEndpointBase
	}

	bot, err := linebot.New(
		channelSecret,
		channelToken,
		linebot.WithEndpointBase(apiEndpointBase), // Usually you omit this.
	)
	if err != nil {
		return nil, err
	}
	downloadDir := filepath.Join(filepath.Dir(os.Args[0]), "line-bot")
	_, err = os.Stat(downloadDir)
	if err != nil {
		if err := os.Mkdir(downloadDir, 0777); err != nil {
			return nil, err
		}
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       redisDB,
	})

	psqlDB, err := sql.Open("postgres", postgresURI)

	return &HailingApp{
		bot:         bot,
		rdb:         rdb,
		pdb:         psqlDB,
		appBaseURL:  appBaseURL,
		downloadDir: downloadDir,
	}, nil
}
