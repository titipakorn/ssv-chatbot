package main

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestInitReserve(t *testing.T) {
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

	userID := "a3a603e2-ac05-f4c7-b8c3-8ddec538a486"
	// clean up redis first
	app.Cancel(userID)

	rec, err := app.FindRecord(userID)
	if err != nil {
		t.Error("App reservation failed: ", err)
	}
	// [0] now record initiailized. First question it is next.
	if rec.State != "init" {
		t.Errorf("[0] App state is not init != %v", rec.State)
	}

	// [1]
	rec, step1 := app.NextStep(userID)
	if step1 != "to" {
		t.Errorf("[1] App state is not 'to' != %v", step1)
	}

	// user return answer correctly
	step1reply := Reply{
		Text: "BTS A",
	}
	rec, err = app.ProcessReservationStep(userID, step1reply)
	fmt.Print(rec)
	if err != nil {
		t.Error("    processing failed: ", err)
	}
	// now record initiailized. First question it is next.
	if rec.State != "to" && rec.To == step1reply.Text {
		t.Errorf("    variable is not correct != %v", rec)
	}

	// [2]
	rec, step2 := app.NextStep(userID)
	if step2 != "from" {
		t.Errorf("[2] App state is not 'from' != %v", step2)
	}

	// user return answer correctly
	step2reply := Reply{
		Text: "CITI Resort",
	}
	rec, err = app.ProcessReservationStep(userID, step2reply)
	fmt.Print(rec)
	if err != nil {
		t.Error("    processing failed: ", err)
	}
	// now record initiailized. First question it is next.
	if rec.State != "from" && rec.From == step2reply.Text {
		t.Errorf("    variable is not correct != %v", rec)
	}

	// [3]
	rec, step3 := app.NextStep(userID)
	if step3 != "when" {
		t.Errorf("[3] App state is not 'when' != %v", step3)
	}

	// user return answer correctly
	step3reply := Reply{
		Datetime: time.Now().Add(15 * time.Minute),
	}
	rec, err = app.ProcessReservationStep(userID, step3reply)
	fmt.Print(rec)
	if err != nil {
		t.Error("    processing failed: ", err)
	}
	// now record initiailized. First question it is next.
	if rec.State != "when" && rec.ReservedAt == step3reply.Datetime {
		t.Errorf("    variable is not correct != %v", rec)
	}

	// [4]
	lastStep := rec.WhatsNext()
	if lastStep != "done" {
		t.Errorf("[4] App state is not 'done' != %v", lastStep)
	}

	total, err := app.DoneAndSave(userID)
	if err != nil {
		t.Error("    processing failed: ", err)
	}
	if total != 1 {
		t.Errorf("    psql insert count != 1 (=%d) ", total)
	}
}
