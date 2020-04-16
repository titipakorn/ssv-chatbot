package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v7"
)

// ReservationRecord : whole process record
type ReservationRecord struct {
	State      string    `json:"state"` // i.e. init, from, to, when,
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

// WhatsNext : to ask what should be the next step
func (record *ReservationRecord) WhatsNext() string {
	// all states are init, from, to, when
	switch record.State {
	case "init":
		// TODO: checkout if drivers are all occupied or not. If so, pickup time first.
		return "to"
	case "to":
		return "from"
	case "from":
		return "when"
	case "when":
		// there is a chance that when starts first if all drivers are occupied.
		if record.To == "" {
			return "to"
		}
		return "done"
	default:
		return "-"
	}
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
	rec, err := app.FindRecord(userID)
	if err != nil {
		return nil, "-"
	}
	nextStep := rec.WhatsNext()
	rec.State = nextStep

	buff, _ := json.Marshal(&rec)
	err = app.rdb.Set(userID, buff, 5*time.Minute).Err()
	if err != nil {
		log.Fatal(err)
		return nil, "-"
	}
	return rec, nextStep
}

// DoneAndSave is to record this completed reservation to a permanent medium (postgresl) instead of Redis
func (app *HailingApp) DoneAndSave(userID string) (int, error) {
	// Double check
	result, err := app.rdb.Get(userID).Result()
	if err != nil {
		return -1, errors.New("There is a problem")
	}
	var rec ReservationRecord
	json.Unmarshal([]byte(result), &rec)
	if rec.From == "" || rec.To == "" || rec.ReservedAt.String() == "0001-01-01 00:00:00 +0000" {
		return -1, errors.New("Something is wrong [ERR: R76]")
	}
	var tripID int
	err = app.pdb.QueryRow(`
	INSERT INTO trip("user_id", "from", "to", "reserved_at")
	VALUES($1, $2, $3, $4) RETURNING id
	`, rec.UserID, rec.From, rec.To, rec.ReservedAt).Scan(&tripID)
	if err != nil {
		return -1, err
	}
	return tripID, nil
}

// FindRecord : this is the one to start everything
func (app *HailingApp) FindRecord(userID string) (*ReservationRecord, error) {
	fmt.Println("Reserve: ", userID)
	result, err := app.rdb.Get(userID).Result()
	if err == redis.Nil {
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
		UserID: userID,
		State:  "init",
	}

	buff, _ := json.Marshal(&newRecord)
	err := app.rdb.Set(userID, buff, 5*time.Minute).Err()
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	return &newRecord, nil
}

// ThrowbackQuestion will throw back a question for this state
// It should return the expected question & answer type
func (app *HailingApp) ThrowbackQuestion(userID string) error {
	return nil
}

func isLocation(reply Reply) bool {
	// TODO: implement this
	return true
}

func isTime(reply Reply) (time.Time, bool) {
	// TODO: implement this
	return time.Now(), true
}

// ProcessReservationStep will handle every step of reservation
func (app *HailingApp) ProcessReservationStep(userID string, reply Reply) (*ReservationRecord, error) {

	result, err := app.rdb.Get(userID).Result()
	fmt.Println("1 result -- : ", userID, result, err)
	if err != nil {
		return nil, errors.New("There is a problem")
	}
	fmt.Println("2 result -- : ", result)
	var rec ReservationRecord
	json.Unmarshal([]byte(result), &rec)
	fmt.Println("3 result -- : ", rec)

	switch rec.State {
	case "from":
		if !isLocation(reply) {
			return nil, errors.New("No location")
		}
		rec.From = reply.Text
	case "to":
		if !isLocation(reply) {
			return nil, errors.New("No location")
		}
		rec.To = reply.Text
	case "when":
		tm, good := isTime(reply)
		if !good {
			return nil, errors.New("No location")
		}
		rec.ReservedAt = tm
	default:
		rec.State = "init"
		buff, _ := json.Marshal(&rec)
		if err := app.rdb.Set(userID, buff, 5*time.Minute).Err(); err != nil {
			log.Fatal(err)
			return nil, err
		}
		return nil, errors.New("Wrong state")
	}
	fmt.Println("here : ", rec)

	buff, _ := json.Marshal(&rec)
	if err := app.rdb.Set(userID, buff, 5*time.Minute).Err(); err != nil {
		log.Fatal(err)
		return nil, err
	}
	return &rec, nil
}
