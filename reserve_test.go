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

func TestLocationWords(t *testing.T) {
	reply := Reply{}
	reply.Text = "Uh"
	_, err := IsLocation(reply)
	if err == nil {
		t.Error("Wrong location, but why does it works [bad1]: ", err)
	}
	reply.Text = "what"
	_, err = IsLocation(reply)
	if err == nil {
		t.Error("Wrong location, but why does it works [bad1]: ", err)
	}
	reply.Text = "Citi resort"
	_, err = IsLocation(reply)
	if err != nil {
		t.Error("Wrong location, but why does it works [good1]: ", err)
	}
}

func TestCheckingServiceArea(t *testing.T) {
	reply := Reply{}

	bad1 := [2]float64{135.5346316, 34.8151187}
	reply.Coords = bad1
	_, err := IsLocation(reply)
	if err == nil {
		t.Error("Wrong location, but why does it works [bad1]: ", err)
	}

	bad2 := [2]float64{100.5666977, 13.723}
	// bad2 := [2]float64{100.566719, 13.724}
	// bad2 := [2]float64{100.578748, 13.724480}
	reply.Coords = bad2
	fmt.Printf("[Test] %v\n", reply)
	_, err = IsLocation(reply)
	if err == nil {
		t.Error("Wrong location, but why does it works [bad2]: ", err)
	}

	bad3 := [2]float64{100.587, 13.7302877}
	reply.Coords = bad3
	_, err = IsLocation(reply)
	if err == nil {
		t.Error("Wrong location, but why does it works [bad3]: ", err)
	}

	bad4 := [2]float64{100.558355, 13.716948}
	reply.Coords = bad4
	_, err = IsLocation(reply)
	if err == nil {
		t.Error("Wrong location, but why does it works [bad4]: ", err)
	}

	good1 := [2]float64{100.578748, 13.724480}
	reply.Coords = good1
	_, err = IsLocation(reply)
	if err != nil {
		t.Error("Good location, why doesn't it work [good1]: ", err)
	}
	good2 := [2]float64{100.5691311, 13.7298491}
	reply.Coords = good2
	_, err = IsLocation(reply)
	if err != nil {
		t.Error("Good location, why doesn't it work [good2]: ", err)
	}
	good3 := [2]float64{100.561785, 13.736299}
	reply.Coords = good3
	_, err = IsLocation(reply)
	if err != nil {
		t.Error("Good location, why doesn't it work [good3]: ", err)
	}
	good4 := [2]float64{100.563497, 13.748527}
	reply.Coords = good4
	_, err = IsLocation(reply)
	if err != nil {
		t.Error("Good location, why doesn't it work [good4]: ", err)
	}
}

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
	if err := cleanUp(app.pdb, userID); err != nil {
		t.Error("Cleanup failed: ", err)
	}

	user, err := app.CreateUser(userID, userID, "dummy-profile")
	rec, err := app.InitReservation(*user)
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
	// user return answer "TO question" correctly
	step1reply := Reply{
		Text: "BTS Phromphong",
	}
	rec, err = app.ProcessReservationStep(user.LineUserID, step1reply)
	if err != nil {
		t.Error("    processing failed: ", err)
	}
	//
	if rec.State != "to" && rec.To == step1reply.Text {
		t.Errorf("    variable is not correct != %v", rec)
	}

	// [2]
	// rec, step2 := app.NextStep(user.LineUserID)
	step2 := rec.Waiting
	if step2 != "from" {
		t.Errorf("[2] App state is not 'from' != %v", step2)
	}

	// user return answer "FROM question" correctly
	step2reply := Reply{
		Coords: [2]float64{100.561785, 13.736299},
	}
	rec, err = app.ProcessReservationStep(user.LineUserID, step2reply)
	if err != nil {
		t.Error("    processing failed: ", err)
	}
	//
	if rec.State != "from" && rec.From == step2reply.Text {
		t.Errorf("    variable is not correct != %v", rec)
	}

	// [3]
	// rec, step3 := app.NextStep(user.LineUserID)
	step3 := rec.Waiting
	if step3 != "when" {
		t.Errorf("[3] App state is not 'when' != %v", step3)
	}

	// user return answer "WHEN" correctly
	step3reply := Reply{
		Datetime: time.Now().Add(15 * time.Minute),
	}
	rec, err = app.ProcessReservationStep(user.LineUserID, step3reply)
	if err != nil {
		t.Error("    processing failed: ", err)
	}

	step4 := rec.Waiting
	if step4 != "num_of_passengers" {
		t.Errorf("[4] App state is not 'num_of_passengers' != %v", step4)
	}
	// user return answer to "NUMBER OF PASSENGERS"
	step4reply := Reply{
		Text: "2",
	}
	rec, err = app.ProcessReservationStep(user.LineUserID, step4reply)
	if err != nil {
		t.Error("    processing failed: ", err)
	}

	step5 := rec.Waiting
	if step5 != "final" {
		t.Errorf("[5] App state is not 'final' != %v", step3)
	}
	// user return answer to "FINAL" or last confirm state
	// it's actually POSTBACK
	step5reply := Reply{
		Text: "last-step-confirmation",
	}
	rec, err = app.ProcessReservationStep(user.LineUserID, step5reply)
	if err != nil {
		t.Error("    processing failed: ", err)
	}

	// after this, record should be in DONE state
	if rec.State != "done" && rec.ReservedAt == step3reply.Datetime {
		t.Errorf("   variable is not correct != %v", rec)
	}

	// [4]
	// lastStep := rec.WhatsNext()
	lastStep := rec.State
	if lastStep != "done" {
		t.Errorf("[4] App state is not 'done' != %v", lastStep)
	}

	tripID, err := app.DoneAndSave(user.LineUserID)
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
