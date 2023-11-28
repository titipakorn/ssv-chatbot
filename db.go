package main

import (
	"database/sql"
	"encoding/json"
	// "errors"
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
	FirstName  string
	LastName   string
	Age			string
	Gender		string
	PrimaryMode	string
	FirstImpression string
	UserType	string
	Telephone	string
	Email      string
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

// Question stores question information
type Questionaire struct {
	ID           	int        `json:"id"`
	Question       string     `json:"question"`
	Type           string     `json:"type"`
}

// Answer stores answers in question
type Answer struct {
	ID           	int        `json:"id"`
	Answer       string     `json:"answer"`
}

// Location stores a list of available choices
type Location struct {
	ID    int    `json:"id"`
	Place Coords `json:"place"`
	Name  string `json:"name"`
}

type Vehicle struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	DriverName string `json:"driver_name"`
	LicensePlate string `json:"license_plate"`
}

type WorkTable struct {
	START_WORK_TIME     string `json:"start_work_time"`
	END_WORK_TIME       string `json:"end_work_time"`
	START_BREAK_TIME 	string `json:"start_break_time"`
	END_BREAK_TIME 		string `json:"end_break_time"`
}

type CNTResult struct {
	CNT     string `json:"cnt"`
}

func (app *HailingApp) GetActiveVehicleByDriverID(ID uuid.UUID) (*Vehicle, error) {
	result := Vehicle{}
	err := app.pdb.QueryRow(`SELECT v.id, v.name, u.username, v.license_plate
		FROM working_shift ws
		LEFT JOIN vehicle v on ws.vehicle_id = v.id
		LEFT JOIN public.user u on ws.user_id = u.id
		WHERE ws.user_id = $1
		AND ws.end is NULL;`, ID).Scan(&result.ID, &result.Name, &result.DriverName, &result.LicensePlate)
	if err != nil {
		return nil, err
	}
	log.Printf("[GetActiveVehicleByDriverID] vehicle ID: %v -- %v", ID, result)
	return &result, nil
}

func (app *HailingApp) GetWorkTable() (*WorkTable, error) {
	result := WorkTable{}
	err := app.pdb.QueryRow(`SELECT start_working_time, end_working_time,start_break_time,end_break_time
		FROM work_table order by created_dt desc limit 1`).Scan(&result.START_WORK_TIME, &result.END_WORK_TIME, &result.START_BREAK_TIME, &result.END_BREAK_TIME)
	if err != nil {
		return nil, err
	}
	log.Printf("[GET WORKTABLE] %v", result)
	return &result, nil
}

func (app *HailingApp) GetBookedJob() (*CNTResult, error) {
	result := CNTResult{}
	err := app.pdb.QueryRow(`select count(*) as cnt from trip where cancelled_at is null and dropped_off_at is null and reserved_at between now()-interval '1 hour' and now()+interval '1 hour'`).Scan(&result.CNT)
	if err != nil {
		return nil, err
	}
	log.Printf("[GET BookedJob] %v", result)
	return &result, nil
}

func (app *HailingApp) GetAvailableCar() (*CNTResult, error) {
	result := CNTResult{}
	err := app.pdb.QueryRow(`select coalesce(sum(case when cnt=0 then 1 end),0) as cnt from (select w.user_id, coalesce(sum(case when t.user_id is not null then 1 end),0) as cnt from working_shift w left join trip t on w.user_id=t.driver_id and t.dropped_off_at is null and t.cancelled_at is null where now()::date=w.start::date and w.end is null group by w.user_id) a`).Scan(&result.CNT)
	if err != nil {
		return nil, err
	}
	log.Printf("[GET AvailableCar] %v", result)
	return &result, nil
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
		FROM location WHERE active=true and length(%s)<=20
		ORDER BY popularity DESC
		LIMIT $1`, fieldName, fieldName)
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
		SELECT
			id, line_user_id, username, profile_url, lang,
			COALESCE(first_name,''), COALESCE(last_name,''), COALESCE(email,''), COALESCE(user_type,''), COALESCE(gender,''), COALESCE(age,''), COALESCE(primary_mode,''), COALESCE(first_impression,''), COALESCE(tel,'')
		FROM "user"
		WHERE line_user_id=$1`,
		lineUserID).Scan(
		&row.ID, &row.LineUserID, &row.Username, &row.ProfileURL, &row.Language, &row.FirstName, &row.LastName, &row.Email, &row.UserType, &row.Gender, &row.Age, &row.PrimaryMode, &row.FirstImpression, &row.Telephone)
	log.Printf("[FindOrCreateUser] log err: %v", err)
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
	SELECT id,line_user_id,username,profile_url,lang,first_name,last_name,email
	FROM "user"
	WHERE id=$1`,
		ID).Scan(&u.ID, &u.LineUserID, &u.Username, &u.ProfileURL, &u.Language, &u.FirstName, &u.LastName, &u.Email)
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
		INSERT INTO trip(
			"user_id", "from", "place_from", "to", "place_to",
			"reserved_at", "polyline", "no_passengers", "is_shared"
		)
		VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`,
			rec.UserID, rec.From, placeFrom, rec.To, placeTo,
			rec.ReservedAt, rec.Polyline, rec.NumOfPassengers, rec.IsShared=="Yes").Scan(&tripID)
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
	//add userfeedback check to avoid skip answering questionaires
	var pFrom orb.Point
	var pTo orb.Point
	var pickedUpAt sql.NullTime
	err := app.pdb.QueryRow(`
	SELECT
		t.id, t.user_id,
		t.from, t.to, t.reserved_at,
		t.picked_up_at, t.polyline, t.no_passengers,
		ST_AsBinary(t.place_from), ST_AsBinary(t.place_to)
	FROM "trip" t
	LEFT JOIN "user" u ON t.user_id = u.id
	WHERE u.line_user_id = $1
		AND t.dropped_off_at is null
		AND t.cancelled_at is null`, lineUserID).Scan(
		&record.TripID, &record.UserID,
		&record.From, &record.To, &record.ReservedAt,
		&pickedUpAt, &record.Polyline, &record.NumOfPassengers,
		wkb.Scanner(&pFrom), wkb.Scanner(&pTo),
	)
	record.FromCoords = [2]float64{pFrom.Lon(), pFrom.Lat()}
	record.ToCoords = [2]float64{pTo.Lon(), pTo.Lat()}
	if pickedUpAt.Valid {
		record.PickedUpAt = pickedUpAt.Time
	}
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

// SaveTripQuestionaire update feedback from user
func (app *HailingApp) SaveTripQuestionaire(tripID int, questionID int, answer int) (string, error) {
	var resultRecordID int
	err := app.pdb.QueryRow(`
	insert into "trip_answers"(trip_id,question_id,answer) values ($1,$2,$3)
	RETURNING trip_id
	`, tripID, questionID, answer).Scan(&resultRecordID)
	if err != nil {
		log.Printf("[save2psql-cancel] %v", err)
		return "failed", err
	}
	return strconv.Itoa(resultRecordID), nil
}

// UpdateUserInfo handles user info change
func (app *HailingApp) UpdateUserInfo(userID uuid.UUID, updateQuery string) (string, error) {
	var resultID uuid.UUID
	query := fmt.Sprintf(`UPDATE "user" SET %s
		WHERE id=$1
		RETURNING id`, updateQuery)

	err := app.pdb.QueryRow(query, userID).Scan(&resultID)
	if err != nil {
		log.Printf("[save2psql-cancel] %v", err)
		return "failed", err
	}
	return "ok", nil
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


// GetQuestions returns questions record
func (app *HailingApp) GetQuestions(lang string) ([]Questionaire, error) {
	results := []Questionaire{}
	fieldName := "question"
	switch lang {
	case "ja":
		fieldName = "question_ja"
	case "th":
		fieldName = "question_th"
	}
	q := fmt.Sprintf(`SELECT id, %s, type
		FROM question_table WHERE active=true
		ORDER BY "order" ASC`, fieldName)
	rows, err := app.pdb.Query(q)
	if err != nil {
		log.Printf("[GetQuestions] %v", err)
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var one Questionaire
		err = rows.Scan(&one.ID, &one.Question, &one.Type)
		if err != nil {
			continue
		}
		results = append(results, one)
	}
	return results, nil
}

// GetQuestions returns questions record
func (app *HailingApp) GetQuestion(lang string, questionID int) (*Questionaire, error) {
	question := Questionaire{}
	fieldName := "question"
	switch lang {
	case "ja":
		fieldName = "question_ja"
	case "th":
		fieldName = "question_th"
	}
	q := fmt.Sprintf(`SELECT id, %s, type
		FROM question_table WHERE id=$1`, fieldName)
	err := app.pdb.QueryRow(q,questionID).Scan(
		&question.ID, &question.Question, &question.Type,
	)
	if err != nil {
		log.Printf("[GetQuestion] %v", err)
		return nil, err
	}
	return &question, nil
}

// GetAnswers returns questions record
func (app *HailingApp) GetAnswers(lang string,questionID int) ([]Answer, error) {
	results := []Answer{}
	fieldName := "answer"
	switch lang {
	case "ja":
		fieldName = "answer_ja"
	case "th":
		fieldName = "answer_th"
	}
	q := fmt.Sprintf(`SELECT id, %s
	FROM answer_table WHERE question_id=$1 order by id asc`, fieldName)
	rows, err := app.pdb.Query(q, questionID)
	if err != nil {
		log.Printf("[GetAnswers] %v", err)
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var one Answer
		err = rows.Scan(&one.ID, &one.Answer)
		if err != nil {
			continue
		}
		results = append(results, one)
	}
	return results, nil
}


// CancelReservation will handle whether it's okay to cancel or not too
func (app *HailingApp) CancelReservation(rec *ReservationRecord) (string, error) {
	trip, err := app.GetTripRecord(rec)
	if err != nil {
		return "failed", err
	}
	//OPEN FOR CANCELATION
	// blankUUID := uuid.UUID{}
	// if trip.DriverID != blankUUID {
	// 	return "failed", errors.New("Contact assigned driver for cancellation")
	// }
	
	fmt.Print("[PSQL-CANCEL] ", trip)
	// if trip.PickedUpAt != nil && trip.PickedUpAt.Format("2006-01-01") != "0001-01-01" {
	// 	// cancel isn't possible now
	// 	// app.Cleanup(rec.LineUserID)
	// 	return "failed", errors.New("Cancellation is not allowed at this point")
	// }
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
		log.Printf("[save2psql-cancel] [1] %v", err)
		return "failed", err
	}
	return "success", nil
}

func (app *HailingApp) UpdateCancellationReason(tripID string, reason string, cancelID string) (string, error) {
	note := fmt.Sprintf("User cancelled via line-bot\nreason: %s", reason)
	cID, _ := strconv.Atoi(cancelID)
	// update postgresql record
	err := app.pdb.QueryRow(`
		UPDATE "trip" SET "note" = $2, cancel_feedback = $3
		WHERE id=$1
		RETURNING id
		`, tripID, note, cID).Scan(&tripID)
	if err != nil {
		log.Printf("[save2psql-cancel] [2] %v", err)
		return "failed", err
	}
	return "success", nil

}

// isLocationInServiceArea returns boolean
func (app *HailingApp) isLocationInServiceArea(point [2]float64) (bool, error) {
	var isIn bool
	q := fmt.Sprintf(`select ST_WITHIN(ST_GeomFromText('POINT(%.6f %.6f)',4326), geometry) from new_service_area`, point[0],point[1])
	err := app.pdb.QueryRow(q).Scan(&isIn)
	if err != nil {
		log.Printf("[isLocationInServiceArea] %v", err)
		return false, err
	}
	return isIn, nil
}