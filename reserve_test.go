package main

import (
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
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

	uuid, err := uuid.NewUUID()
	if err != nil {
		t.Error("UUID generation failed: ", err)
	}
	userID := uuid.String()
	fmt.Printf("UUID: %v\n", userID)

	// clean up redis first
	app.Cancel(userID)
	// make sure that user existed in psql - user
	if err := initUser(app.pdb, userID); err != nil {
		t.Error("InitUser failed: ", err)
	}

	rec, err := app.FindOrCreateRecord(userID)
	if err != nil {
		t.Error("App reservation failed: ", err)
	}
	// [0] now record initiailized. First question it is next.
	if rec.State != "init" {
		t.Errorf("[0] App state is not init != %v", rec.State)
	}

	// [1]
	// rec, step1 := app.NextStep(userID)
	step1 := rec.Waiting
	if step1 != "to" {
		t.Errorf("[1] App state is not 'to' != %v", step1)
	}

	// user return answer correctly
	step1reply := Reply{
		Text: "BTS A",
	}
	rec, err = app.ProcessReservationStep(userID, step1reply)
	if err != nil {
		t.Error("    processing failed: ", err)
	}
	// now record initiailized. First question it is next.
	if rec.State != "to" && rec.To == step1reply.Text {
		t.Errorf("    variable is not correct != %v", rec)
	}

	// [2]
	// rec, step2 := app.NextStep(userID)
	step2 := rec.Waiting
	if step2 != "from" {
		t.Errorf("[2] App state is not 'from' != %v", step2)
	}

	// user return answer correctly
	step2reply := Reply{
		Text: "CITI Resort",
	}
	rec, err = app.ProcessReservationStep(userID, step2reply)
	if err != nil {
		t.Error("    processing failed: ", err)
	}
	// now record initiailized. First question it is next.
	if rec.State != "from" && rec.From == step2reply.Text {
		t.Errorf("    variable is not correct != %v", rec)
	}

	// [3]
	// rec, step3 := app.NextStep(userID)
	step3 := rec.Waiting
	if step3 != "when" {
		t.Errorf("[3] App state is not 'when' != %v", step3)
	}

	// user return answer correctly
	step3reply := Reply{
		Datetime: time.Now().Add(15 * time.Minute),
	}
	rec, err = app.ProcessReservationStep(userID, step3reply)
	if err != nil {
		t.Error("    processing failed: ", err)
	}
	// now record initiailized. First question it is next.
	if rec.State != "when" && rec.ReservedAt == step3reply.Datetime {
		t.Errorf("    variable is not correct != %v", rec)
	}

	// [4]
	// lastStep := rec.WhatsNext()
	lastStep := rec.Waiting
	if lastStep != "done" {
		t.Errorf("[4] App state is not 'done' != %v", lastStep)
	}

	tripID, err := app.DoneAndSave(userID)
	if err != nil {
		t.Error("    processing failed: ", err)
	}
	if tripID < 1 {
		t.Errorf("    psql insert to tripID wrongly (=%v) ", tripID)
	}
}

func TestQuestionFromEachState(t *testing.T) {
	record := ReservationRecord{
		State:   "init",
		Waiting: "to",
	}
	q1 := record.QuestionToAsk()
	fmt.Println(q1)
	if q1.Text != "Where to?" {
		t.Errorf("Where to? != %v", q1.Text)
	}

	record.State = "to"
	record.Waiting = "from"
	q2 := record.QuestionToAsk()
	if q2.Text != "Pickup location?" {
		t.Errorf("Pickup location? != %v", q2.Text)
	}

	record.State = "from"
	record.Waiting = "when"
	q3 := record.QuestionToAsk()
	if q3.Text != "When?" {
		t.Errorf("When? != %v", q3.Text)
	}

}

func initUser(db *sql.DB, userID string) error {

	cleanUp(db, userID)

	_, err := db.Exec(`
	INSERT INTO "user" (id, username)
	VALUES ($1, $2)`, userID, "test_user")
	if err != nil {
		return err
	}
	return nil
}

func cleanUp(db *sql.DB, userID string) error {

	_, err := db.Exec(`
		DELETE FROM "trip"
        WHERE user_id IN (
	    SELECT id FROM "user" WHERE id=$1 OR username=$2
        )`, userID, "test_user")
	if err != nil {
		return err
	}
	_, err = db.Exec(`DELETE FROM "user" WHERE id=$1 OR username=$2`, userID, "test_user")
	if err != nil {
		return err
	}
	return nil
}
