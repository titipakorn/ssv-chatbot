package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v7"
)

// ReservationRecord : whole process record
type ReservationRecord struct {
	State      string    `json:"state"` // i.e. init, from, to, when,
	Waiting    string    `json:"waiting"`
	From       string    `json:"from"`
	To         string    `json:"to"`
	UserID     string    `json:"user_id"`
	DriverID   string    `json:"driver_id"`
	ReservedAt time.Time `json:"reserved_at"`
	// PickedUpAt   time.Time `json:"picked_up_at"`
	// DroppedOffAt time.Time `json:"dropped_off_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Reply : to store reply in various message type
type Reply struct {
	Text     string     `json:"text"`
	Datetime time.Time  `json:"datetime"`
	Coords   [2]float64 `json:"coords"`
}

// QuickReplyButton contains necessary info for linebot.NewQuickReplyButton
type QuickReplyButton struct {
	Image string `json:"image"`
	Label string `json:"label"`
	Text  string `json:"text"`
}

// Question contains what's the message and extra options for Chatbot
type Question struct {
	Text          string `json:"text"`
	Buttons       []QuickReplyButton
	DatetimeInput bool
	LocationInput bool
}

// WhatsNext : to ask what should be the next step
func (record *ReservationRecord) WhatsNext() string {

	done, missing := record.IsComplete()
	if done {
		record.State = "done"
	}

	// all states are init, from, to, when
	switch record.State {
	case "init":
		// TODO: checkout if drivers are all occupied or not. If so, pickup time first.
		return "to"
	case "to":
		if record.To == "" {
			return "to"
		}
	case "from":
		if record.From == "" {
			return "from"
		}
	case "when":
		if record.ReservedAt.Format("2006-01-01") == "0001-01-01" {
			return "when"
		}
		// there is a chance that when starts first if all drivers are occupied.
		if record.To == "" {
			return "to"
		}
	case "done":
		return "pickup"
	}
	return missing
}

// Cancel : to cancel this reservation
func (app *HailingApp) Cancel(userID string) (int64, error) {

	n, err := app.rdb.Del(userID).Result()
	if err != nil {
		return -1, err
	}
	return n, nil

}

// NextStep will return next state and update the state of the record to that
func (app *HailingApp) NextStep(userID string) (*ReservationRecord, string) {
	rec, err := app.FindOrCreateRecord(userID)
	if err != nil {
		return nil, "-"
	}
	nextStep := rec.WhatsNext()
	rec.Waiting = nextStep

	buff, _ := json.Marshal(&rec)
	err = app.rdb.Set(userID, buff, 5*time.Minute).Err()
	if err != nil {
		log.Fatal(err)
		return nil, "-"
	}
	return rec, nextStep
}

// DoneAndSave is to record this completed reservation to a permanent medium (postgresl)
func (app *HailingApp) DoneAndSave(userID string) (int, error) {
	// Double check
	result, err := app.rdb.Get(userID).Result()
	if err != nil {
		return -1, errors.New("There is a problem")
	}
	var rec ReservationRecord
	json.Unmarshal([]byte(result), &rec)
	if rec.From == "" || rec.To == "" || rec.ReservedAt.Format("2006-01-01") == "0001-01-01" {
		return -1, errors.New("Something is wrong [ERR: R76]")
	}
	return app.SaveRecordToPostgreSQL(&rec)
}

// IsComplete is a shorthand to check if record is filled
// return IsComplete & missing state
func (record *ReservationRecord) IsComplete() (bool, string) {
	if record.To == "" {
		return false, "to"
	}
	if record.From == "" {
		return false, "from"
	}
	if record.ReservedAt.Format("2006-01-01") == "0001-01-01" {
		return false, "when"
	}
	return true, "done"
}

// SaveRecordToPostgreSQL is to record this completed reservation to a permanent medium (postgresl)
func (app *HailingApp) SaveRecordToPostgreSQL(rec *ReservationRecord) (int, error) {
	var tripID int
	err := app.pdb.QueryRow(`
	INSERT INTO trip("user_id", "from", "to", "reserved_at")
	VALUES($1, $2, $3, $4) RETURNING id
	`, rec.UserID, rec.From, rec.To, rec.ReservedAt).Scan(&tripID)
	if err != nil {
		log.Fatalf("[save2psql] %v", err)
		return -1, err
	}
	return tripID, nil
}

// FindRecord : this is the one to ask if we have any reservation
func (app *HailingApp) FindRecord(userID string) (*ReservationRecord, error) {
	result, err := app.rdb.Get(userID).Result()
	if err == redis.Nil {
		return nil, errors.New("No record found")
	} else if err != nil {
		return nil, errors.New("There is a problem")
	}
	var rec ReservationRecord
	json.Unmarshal([]byte(result), &rec)
	return &rec, nil
}

// FindOrCreateRecord : this is the one to start everything
func (app *HailingApp) FindOrCreateRecord(userID string) (*ReservationRecord, error) {
	// fmt.Println("Reserve: ", userID)
	result, err := app.rdb.Get(userID).Result()
	if err == redis.Nil {
		log.Println("[FindOrCreateRecord] init new one")
		return app.initReservation(userID)
	} else if err != nil {
		return nil, errors.New("There is a problem")
	}
	var rec ReservationRecord
	json.Unmarshal([]byte(result), &rec)
	return &rec, nil
}

func (app *HailingApp) initReservation(userID string) (*ReservationRecord, error) {
	newRecord := ReservationRecord{
		UserID:  userID,
		State:   "init",
		Waiting: "to",
	}

	buff, _ := json.Marshal(&newRecord)
	err := app.rdb.Set(userID, buff, 5*time.Minute).Err()
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	return &newRecord, nil
}

// QuestionToAsk returns a question appropriate for each state
func (record *ReservationRecord) QuestionToAsk() Question {
	switch strings.ToLower(record.Waiting) {
	case "when":
		buttons := []QuickReplyButton{
			{
				Label: "Now",
				Text:  "now",
			},
			{
				Label: "In 15 mins",
				Text:  "+15min",
			},
			{
				Label: "In 30 mins",
				Text:  "+30min",
			},
		}
		return Question{
			Text:          "When?",
			Buttons:       buttons,
			DatetimeInput: true,
		}
	case "to":
		buttons := []QuickReplyButton{
			{
				Label: "Condo A",
				Text:  "Condo A",
			},
			{
				Label: "CITI Resort",
				Text:  "CITI Resort",
			},
			{
				Label: "BTS Phromphong",
				Text:  "BTS Phromphong",
			},
		}
		return Question{
			Text:          "Where to?",
			Buttons:       buttons,
			LocationInput: true,
		}
	case "from":
		buttons := []QuickReplyButton{
			{
				Label: "Condo A",
				Text:  "Condo A",
			},
			{
				Label: "CITI Resort",
				Text:  "CITI Resort",
			},
			{
				Label: "BTS Phromphong",
				Text:  "BTS Phromphong",
			},
		}
		return Question{
			Text:          "Pickup location?",
			Buttons:       buttons,
			LocationInput: true,
		}
	}
	return Question{
		Text: "n/a",
	}
}

func isLocation(reply Reply) bool {
	// TODO: implement this
	if reply.Coords != [2]float64{0, 0} {
		// TODO: probably check if coords are in service area
		return true
	}
	// check for text if it's match
	if IsThisIn(strings.ToLower(reply.Text), TargetPlaces) {
		return true
	}
	return false
}

func isTime(reply Reply) (*time.Time, error) {
	var t time.Time
	now := time.Now()

	if reply.Datetime.Format("2006-01-02") != "0001-01-01" {
		t = reply.Datetime
	} else {
		lowercase := strings.ToLower(reply.Text)
		if lowercase == "now" {
			return &now, nil
		}
		pattern := regexp.MustCompile(`\+(\d+)(min|hour)`)
		res := pattern.FindAllStringSubmatch(lowercase, -1)
		if len(res) == 0 {
			return nil, errors.New("Not date")
		}
		unit := res[0][2]
		if unit != "min" && unit != "hour" {
			return nil, errors.New("Not date")
		}
		num, err := strconv.Atoi(res[0][1])
		if err != nil {
			return nil, errors.New("Not date")
		}
		duration := time.Duration(num)
		if unit == "min" {
			t = now.Add(duration * time.Minute)
		} else if unit == "hour" {
			t = now.Add(duration * time.Hour)
		}
	}
	diffFromNow := t.Sub(now)
	if diffFromNow.Minutes() < 0 {
		log.Printf("[isTime] %v \n", diffFromNow)
		return &t, errors.New("Time is in the past")
	}
	if diffFromNow.Hours() > 24 {
		log.Printf("[isTime] %v \n", diffFromNow)
		return &t, errors.New("Only allow 24-hr in advance")
	}
	return &t, nil
}

// ProcessReservationStep will handle every step of reservation
func (app *HailingApp) ProcessReservationStep(userID string, reply Reply) (*ReservationRecord, error) {

	rec, err := app.FindOrCreateRecord(userID)
	if err != nil {
		return nil, errors.New("There is a problem")
	}

	switch rec.Waiting {
	case "from":
		if !isLocation(reply) {
			return rec, errors.New("No location")
		}
		if reply.Coords != [2]float64{0, 0} {
			rec.From = fmt.Sprintf("%v", reply.Coords)
		} else {
			rec.From = reply.Text
		}
	case "to":
		if !isLocation(reply) {
			return rec, errors.New("No location")
		}
		if reply.Coords != [2]float64{0, 0} {
			rec.To = fmt.Sprintf("%v", reply.Coords)
		} else {
			rec.To = reply.Text
		}
	case "when":
		tm, err := isTime(reply)
		if err != nil {
			log.Printf("[ProcessReservationStep] when: %v %v \n", tm, err)
			return rec, err
		}
		log.Printf("[ProcessReservationStep] when passed:\n")
		rec.ReservedAt = *tm
	case "done":
		// nothing to save here save record to postgres
	case "pickup":
		// 1st case is "modify-pickup-time"
		if reply.Text == "modify-pickup-time" {
			tm, err := isTime(reply)
			if err != nil {
				return rec, err
			}
			rec.ReservedAt = *tm
		}
	default:
		return rec, errors.New("Wrong state")
	}
	log.Printf("[ProcessReservationStep] pre_status_change: %s \n   >> record: %v", rec.State, rec)

	rec.State = rec.Waiting
	rec.Waiting = rec.WhatsNext()
	cacheDuration := 5 * time.Minute
	log.Printf("[ProcessReservationStep] mid_status_change: %s \n   >> record: %v", rec.State, rec)
	if rec.State == "done" {
		cacheDuration = 24 * time.Hour
		// TODO: write to postgresql
		app.SaveRecordToPostgreSQL(rec)
	}
	buff, _ := json.Marshal(&rec)
	log.Printf("[ProcessReservationStep] post_status_change: %s \n   >> record: %v", rec.State, rec)

	if err := app.rdb.Set(userID, buff, cacheDuration).Err(); err != nil {
		log.Fatal(err)
		return nil, err
	}
	return rec, nil
}
