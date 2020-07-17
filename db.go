package main

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
)

// User stores user information
type User struct {
	ID         uuid.UUID
	LineUserID string
	Username   string
	ProfileURL string
}

// Trip stores trip information
type Trip struct {
	ID         int
	UserID     uuid.UUID
	DriverID   uuid.UUID
	ReservedAt *time.Time
	PickedUpAt *time.Time
}

// FindOrCreateUser handles user query from line user id
func (app *HailingApp) FindOrCreateUser(lineUserID string) (*User, error) {
	row := User{}
	err := app.pdb.QueryRow(`
		SELECT id,line_user_id,username,profile_url FROM "user"
		WHERE line_user_id=$1`,
		lineUserID).Scan(&row.ID, &row.LineUserID, &row.Username, &row.ProfileURL)

	if err != nil && strings.Contains(err.Error(), "no rows in result set") {
		// we have to create a new record
		profile, botErr := app.bot.GetProfile(lineUserID).Do()
		if botErr != nil {
			return nil, botErr
		}
		username := profile.DisplayName
		profileURL := profile.PictureURL
		uC, errC := app.CreateUser(username, lineUserID, profileURL)
		if errC != nil {
			return nil, errC
		}
		return uC, nil
	}

	// if err == nil || !strings.Contains(err.Error(), "expected 2 destination arguments") {
	// 	log.Printf("expected error from wrong number of arguments; actually got: %v", err)
	// 	return nil, err
	// }

	return &row, nil
}

// CreateUser handles user creation
func (app *HailingApp) CreateUser(username string, lineUserID string, profileURL string) (*User, error) {
	var uid uuid.UUID
	err := app.pdb.QueryRow(`
	INSERT INTO "user"("username", "line_user_id", "profile_url")
	VALUES($1, $2, $3) RETURNING id
	`, username, lineUserID, profileURL).Scan(&uid)

	if err != nil && strings.Contains(err.Error(), "user_username_key") {
		// duplicate username found
		newRandom, _ := uuid.NewRandom()
		four := fmt.Sprintf("%v", newRandom)[:4]
		newUsername := fmt.Sprintf("%s_%s", username, four)
		return app.CreateUser(newUsername, lineUserID, profileURL)
	}

	if err != nil {
		log.Fatalf("[CreateUser] %v", err)
		return nil, err
	}

	u := User{
		ID:         uid,
		Username:   username,
		LineUserID: lineUserID,
		ProfileURL: profileURL,
	}
	return &u, nil
}

// SaveReservationToPostgres is to record this completed reservation to a permanent medium (postgresl)
func (app *HailingApp) SaveReservationToPostgres(rec *ReservationRecord) (int, error) {
	var tripID int
	if rec.TripID == -1 {
		placeFrom := fmt.Sprintf("POINT(%.8f %.8f)", rec.FromCoords[0], rec.FromCoords[1])
		placeTo := fmt.Sprintf("POINT(%.8f %.8f)", rec.ToCoords[0], rec.ToCoords[1])
		// insert if no trip_id yet
		err := app.pdb.QueryRow(`
		INSERT INTO trip("user_id", "from", "place_from", "to", "place_to", "reserved_at")
		VALUES($1, $2, $3, $4, $5, $6) RETURNING id
		`, rec.UserID, rec.From, placeFrom, rec.To, placeTo, rec.ReservedAt).Scan(&tripID)
		if err != nil {
			log.Printf("[save2psql-create] %v", err)
			return -1, err
		}
		return tripID, nil
	}
	// update postgresql record
	err := app.pdb.QueryRow(`
	UPDATE "trip" SET ("from", "to", "reserved_at") = ($2, $3, $4)
	WHERE id=$1
	RETURNING id
	`, rec.TripID, rec.From, rec.To, rec.ReservedAt).Scan(&tripID)
	if err != nil {
		log.Printf("[save2psql-update] %v", err)
		return -1, err
	}
	return tripID, nil
}

// FindActiveReservation query from postgresql and put in redis
func (app *HailingApp) FindActiveReservation(lineUserID string) (*ReservationRecord, error) {
	record := ReservationRecord{LineUserID: lineUserID, State: "done", IsConfirmed: true}
	err := app.pdb.QueryRow(`
	SELECT
		"id", "user_id", "from", "to", "place_from", "place_to",
		"reserved_at", "picked_up_at"
	FROM "trip"
	WHERE user_id=$1
		AND dropped_off_at = null
		AND cancelled_at = null`, lineUserID).Scan(
		&record.TripID, &record.UserID, &record.From, &record.To,
		&record.FromCoords, &record.ToCoords, &record.ReservedAt,
		&record.PickedUpAt,
	)
	if err != nil {
		log.Printf("[FindActiveReservation] %v", err)
		return nil, err
	}
	return &record, nil
}

// GetTripRecord returns trip record
func (app *HailingApp) GetTripRecord(rec *ReservationRecord) (*Trip, error) {
	trip := Trip{}
	err := app.pdb.QueryRow(`
	SELECT "user_id", "driver_id", "reserved_at", "picked_up_at"
	FROM "trip"
	WHERE id=$1`, rec.TripID).Scan(
		&trip.UserID, &trip.DriverID, &trip.ReservedAt, &trip.PickedUpAt,
	)
	if err != nil {
		log.Printf("[GetTripRecord] %v", err)
		return nil, err
	}
	return &trip, nil
}

// CancelReservation will handle whether it's okay to cancel or not too
func (app *HailingApp) CancelReservation(rec *ReservationRecord) (string, error) {
	trip, err := app.GetTripRecord(rec)
	if err != nil {
		return "failed", err
	}
	blankUUID := uuid.UUID{}
	if trip.DriverID != blankUUID {
		return "failed", errors.New("Contact assigned driver for cancellation")
	}
	fmt.Print("[PSQL-CANCEL] ", trip)
	if trip.PickedUpAt != nil && trip.PickedUpAt.Format("2006-01-01") != "0001-01-01" {
		// cancel isn't possible now
		return "failed", errors.New("Cancellation is not allowed at this point")
	}
	var tripID int
	now := time.Now()
	note := "User cancelled via line-bot"
	// update postgresql record
	err = app.pdb.QueryRow(`
		UPDATE "trip" SET ("note", "cancelled_at") = ($2, $3)
		WHERE id=$1
		RETURNING id
		`, rec.TripID, note, now).Scan(&tripID)
	if err != nil {
		log.Printf("[save2psql-cancel] %v", err)
		return "failed", err
	}
	// err = app.pdb.QueryRow(`
	// 	DELETE FROM "trip" WHERE id=$1`, rec.TripID).Scan()
	// if err != nil && !strings.Contains(err.Error(), "no rows in result set") {
	// 	return "failed", err
	// }
	return "success", nil
}
