package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v7"
)

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

type Reply struct {
	Text     string     `json:"text"`
	Datetime time.Time  `json:"datetime"`
	Coords   [2]float64 `json:"coords"`
}

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

func (app *HailingApp) Cancel(userID string) (int64, error) {

	n, err := app.rdb.Del(userID).Result()
	if err != nil {
		return -1, err
	}
	return n, nil

}

func (app *HailingApp) Reserve(userID string) (*ReservationRecord, error) {
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

func isLocation(reply Reply) bool {
	// TODO: implement this
	return true
}

func isTime(reply Reply) (time.Time, bool) {
	// TODO: implement this
	return time.Now(), true
}

func (app *HailingApp) ProcessReservationStep(userID string, reply Reply) (*ReservationRecord, error) {

	result, err := app.rdb.Get(userID).Result()
	if err != nil {
		return nil, errors.New("There is a problem")
	}
	var rec ReservationRecord
	json.Unmarshal([]byte(result), &rec)

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

	buff, _ := json.Marshal(&rec)
	if err := app.rdb.Set(userID, buff, 5*time.Minute).Err(); err != nil {
		log.Fatal(err)
		return nil, err
	}
	return &rec, nil
}
