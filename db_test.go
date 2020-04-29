package main

import (
	"os"
	"strings"
	"testing"
)

func TestBasicPostgreSQLOpts(t *testing.T) {

	os.Setenv("REDIS_ADDR", "localhost:6379")
	os.Setenv("REDIS_PASSWORD", "")
	os.Setenv("REDIS_PDB", "0")
	os.Setenv("POSTGRES_URI", "postgres://sipp11:banshee10@127.0.0.1:25432/hailing")

	app, err := NewHailingApp(
		os.Getenv("CHANNEL_SECRET"),
		os.Getenv("CHANNEL_TOKEN"),
		os.Getenv("APP_BASE_URL"),
	)

	if err != nil {
		t.Error("App initialization failed ", err)
	}

	lineUserID := "U9342f415d6ac8780b2487bbaa90906d9" // sipp11
	user, err := app.FindOrCreateUser(lineUserID)
	if err != nil {
		t.Error("FindOrCreateUser Failed: ", err)
	}
	if !strings.Contains(user.Username, "sipp11") {
		t.Error("FindOrCreateUser Failed: username mismatch: ", err)
	}
}
