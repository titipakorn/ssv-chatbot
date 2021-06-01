package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/encoding/wkb"
)

// User stores user information
type User struct {
	ID         uuid.UUID
	LineUserID string
	Username   string
	ProfileURL string
	Language   string
}

// Coords stores geojson data
type Coords struct {
	Coordinates [2]float64 `json:"coordinates"`
	Type        string     `json:"type"`
}

// Trip stores trip information
type Trip struct {
	ID           int        `json:"id"`
	UserID       uuid.UUID  `json:"user_id"`
	DriverID     uuid.UUID  `json:"driver_id"`
	ReservedAt   *time.Time `json:"reserved_at"`
	AcceptedAt   *time.Time `json:"accepted_at"`
	PickedUpAt   *time.Time `json:"picked_up_at"`
	DroppedOffAt *time.Time `json:"dropped_off_at"`
	CancelledAt  *time.Time `json:"cancelled_at"`
	From         string     `json:"from"`
	To           string     `json:"to"`
	// PlaceFrom      Coords     `json:"place_from"`
	// PlaceTo        Coords     `json:"place_to"`
	Note           string `json:"note"`
	UserFeedback   int    `json:"user_feedback"`
	DriverFeedback int    `json:"driver_feedback"`
}

// Location stores a list of available choices
type Location struct {
	ID    int    `json:"id"`
	Place Coords `json:"place"`
	Name  string `json:"name"`
}

// GetLocationByID returns a location in Location struct
func (app *HailingApp) GetLocationByID(ID int) (*Location, error) {
	result := Location{}
	var p []byte
	err := app.pdb.QueryRow(`SELECT id, name, ST_AsGeoJSON(place)
		FROM location
		WHERE id=$1`, ID).Scan(&result.ID, &result.Name, &p)
	if err != nil {
		return nil, err
	}
	json.Unmarshal(p, &result.Place)
	log.Printf("[GetLocationByID] location ID: %v -- %v", ID, result)
	return &result, nil
}

// GetLocations return most popular locations
func (app *HailingApp) GetLocations(lang string, total int) ([]Location, error) {
	results := []Location{}
	maxTotal := 10
	if total < 1 || total > maxTotal {
		total = maxTotal
	}
	fieldName := "name"
	switch lang {
	case "ja":
		fieldName = "name_ja"
	case "th":
		fieldName = "name_th"
	}
	q := fmt.Sprintf(`SELECT id, %s, ST_AsGeoJSON(place)
		FROM location
		ORDER BY popularity DESC
		LIMIT $1`, fieldName)
	rows, err := app.pdb.Query(q, total)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var one Location
		var p []byte
		err = rows.Scan(&one.ID, &one.Name, &p)
		if err != nil {
			continue
		}
		json.Unmarshal(p, &one.Place)
		results = append(results, one)
	}
	return results, nil
}

// FindOrCreateUser handles user query from line user id
func (app *HailingApp) FindOrCreateUser(lineUserID string) (*User, error) {
	row := User{}
	err := app.pdb.QueryRow(`
		SELECT id,line_user_id,username,profile_url,lang FROM "user"
		WHERE line_user_id=$1`,
		lineUserID).Scan(
		&row.ID, &row.LineUserID, &row.Username, &row.ProfileURL, &row.Language)

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
	return &row, nil
}

// FindUserByID is to find LineUserID from the system ID
func (app *HailingApp) FindUserByID(ID uuid.UUID) (*User, error) {
	u := User{}
	err := app.pdb.QueryRow(`
	SELECT line_user_id
	FROM "user"
	WHERE id=$1`,
		ID).Scan(&u.LineUserID)
	if err != nil {
		return nil, err
	}
	return &u, nil
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
		Language:   "en",
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
		INSERT INTO trip("user_id", "from", "place_from", "to", "place_to", "reserved_at", "polyline")
		VALUES($1, $2, $3, $4, $5, $6, $7) RETURNING id
		`, rec.UserID, rec.From, placeFrom, rec.To, placeTo, rec.ReservedAt, rec.Polyline).Scan(&tripID)
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

	var pFrom orb.Point
	var pTo orb.Point
	err := app.pdb.QueryRow(`
	SELECT
		t."id", t."user_id", t."from", t."to", t."place_from", t."place_to",
		"reserved_at", t."picked_up_at", t."polyline"
	FROM "trip" t
	LEFT JOIN "user" u ON t.user_id = u.id
	WHERE u.line_user_id = $1
		AND t.dropped_off_at is null
		AND t.cancelled_at is null`, lineUserID).Scan(
		&record.TripID, &record.UserID, &record.From, &record.To,
		wkb.Scanner(&pFrom), wkb.Scanner(&pTo), &record.ReservedAt,
		&record.PickedUpAt, &record.Polyline,
	)
	record.FromCoords = [2]float64{pFrom.Lon(), pFrom.Lat()}
	record.ToCoords = [2]float64{pTo.Lon(), pTo.Lat()}
	if err != nil {
		// log.Printf("[FindActiveReservation] %v", err)
		return nil, err
	}
	return &record, nil
}

// SaveTripFeedback update feedback from user
func (app *HailingApp) SaveTripFeedback(tripID int, rating int) (string, error) {
	var resultTripID int
	err := app.pdb.QueryRow(`
	UPDATE "trip" SET "user_feedback" = $2
	WHERE id=$1
	RETURNING id
	`, tripID, rating).Scan(&resultTripID)
	if err != nil {
		log.Printf("[save2psql-cancel] %v", err)
		return "failed", err
	}
	return strconv.Itoa(resultTripID), nil
}

// SetLanguage save preferred lanague to "user" table
func (app *HailingApp) SetLanguage(userID uuid.UUID, lang string) (string, error) {
	var resultID uuid.UUID
	err := app.pdb.QueryRow(`
	UPDATE "user" SET "lang" = $2
	WHERE id=$1
	RETURNING id
	`, userID, lang).Scan(&resultID)
	if err != nil {
		log.Printf("[save2psql-cancel] %v", err)
		return "failed", err
	}
	return "ok", nil
}

// GetTripRecordByID returns trip record
func (app *HailingApp) GetTripRecordByID(tripID int) (*Trip, error) {
	trip := Trip{}
	err := app.pdb.QueryRow(`
	SELECT "user_id", "reserved_at", "picked_up_at", "from", "to", "dropped_off_at"
	FROM "trip"
	WHERE id=$1`, tripID).Scan(
		&trip.UserID, &trip.ReservedAt, &trip.PickedUpAt, &trip.From, &trip.To,
		&trip.DroppedOffAt,
	)
	if err != nil {
		log.Printf("[GetTripRecord] %v", err)
		return nil, err
	}
	return &trip, nil
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
	return "success", nil
}
