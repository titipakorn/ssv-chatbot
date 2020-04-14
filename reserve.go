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
	From         string    `json:"from"`
	To           string    `json:"to"`
	UserID       string    `json:"user_id"`
	DriverID     string    `json:"driver_id"`
	ReservedAt   time.Time `json:"reserved_at"`
	PickedUpAt   time.Time `json:"picked_up_at"`
	DroppedOffAt time.Time `json:"dropped_off_at"`
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
	}

	buff, _ := json.Marshal(&newRecord)
	err := app.rdb.Set(userID, buff, 5*time.Minute).Err()
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	return &newRecord, nil
}
