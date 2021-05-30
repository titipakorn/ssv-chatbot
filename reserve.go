package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v7"
	"github.com/google/uuid"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/planar"
)

// ReservationRecord : whole process record
type ReservationRecord struct {
	State           string     `json:"state"` // i.e. init, to, from, when, final -> done
	Waiting         string     `json:"waiting"`
	From            string     `json:"from"`
	FromCoords      [2]float64 `json:"from_coords"`
	To              string     `json:"to"`
	ToCoords        [2]float64 `json:"to_coords"`
	UserID          uuid.UUID  `json:"user_id"` // this is our system id, not line's
	LineUserID      string     `json:"line_user_id"`
	DriverID        string     `json:"driver_id"`
	ReservedAt      time.Time  `json:"reserved_at"`
	PickedUpAt      time.Time  `json:"picked_up_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	TripID          int        `json:"trip_id"` // postgresql id
	IsConfirmed     bool       `json:"is_confirmed"`
	Polyline        string     `json:"polyline"`
	NumOfPassengers int        `json:"num_of_passengers"` // postgresql id
	// DroppedOffAt time.Time  `json:"dropped_off_at"`
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
	YesInput      bool
}

// WhatsNext : to ask what should be the next step
func (record *ReservationRecord) WhatsNext() string {
	/* Step is as follows

	init -> to -> from -> when -> final -> done

	redis record will not live long anymore
	*/
	done, missing := record.IsComplete()
	if done {
		record.State = "done"
	}

	// all states are init, from, to, when, num_of_passengers
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
	case "num_of_passengers":
		if record.NumOfPassengers == 0 {
			return "num_of_passengers"
		}
	case "final":
		return "done"
	case "done":
		return "pickup"
	}
	return missing
}

// Cancel : to cancel this reservation
func (app *HailingApp) Cancel(userID string) (int64, error) {
	rec, err := app.FindRecord(userID)
	if err != nil {
		// if record not found, it's good
		return 0, nil
	}

	if rec.TripID != -1 {
		_, err := app.CancelReservation(rec)
		if err != nil && !strings.Contains(err.Error(), "no rows in result set") {
			return -1, err
		}
	}
	// if there is no tripID yet, then continue with cancel process
	n, err := app.rdb.Del(userID).Result()
	if err != nil {
		return -1, err
	}
	return n, nil

}

// Cleanup meant to clear all redis cache out of the system
func (app *HailingApp) Cleanup(userID string) error {
	// if there is no tripID yet, then continue with cancel process
	_, err := app.rdb.Del(userID).Result()
	if err != nil {
		return err
	}
	return nil
}

// NextStep will return next state and update the state of the record to that
func (app *HailingApp) NextStep(userID string) (*ReservationRecord, string) {
	rec, err := app.FindOrCreateRecord(userID)
	if err != nil {
		return nil, "-"
	}
	nextStep := rec.WhatsNext()
	rec.Waiting = nextStep

	err = app.SaveRecordToRedis(rec)
	if err != nil {
		return nil, "-"
	}
	return rec, nextStep
}

// DoneAndSave is to record this completed reservation to a permanent medium (postgresl)
func (app *HailingApp) DoneAndSave(lineUserID string) (int, error) {
	// Double check
	result, err := app.rdb.Get(lineUserID).Result()
	if err != nil {
		return -1, errors.New("There is a problem")
	}
	var rec ReservationRecord
	json.Unmarshal([]byte(result), &rec)
	if rec.From == "" || rec.To == "" || rec.ReservedAt.Format("2006-01-01") == "0001-01-01" {
		return -1, errors.New("Something is wrong [ERR: R76]")
	}
	return app.SaveReservationToPostgres(&rec)
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
	if record.NumOfPassengers == 0 {
		return false, "num_of_passengers"
	}
	if !record.IsConfirmed {
		return false, "final"
	}
	return true, "done"
}

// SaveRecordToRedis which save ReservationRecord to redis for faster process
func (app *HailingApp) SaveRecordToRedis(record *ReservationRecord) error {
	buff, _ := json.Marshal(&record)
	cacheDuration := 10 * time.Minute
	// log.Printf("[ProcessReservationStep] post_status_change: %s \n   >> record: %v", rec.State, rec.UpdatedAt)
	if err := app.rdb.Set(record.LineUserID, buff, cacheDuration).Err(); err != nil {
		log.Printf("Redis Error: %v", err)
		return err
	}
	return nil
}

// FindRecord : this is the one to ask if we have any reservation
func (app *HailingApp) FindRecord(lineUserID string) (*ReservationRecord, error) {
	result, err := app.rdb.Get(lineUserID).Result()
	if err != nil || err == redis.Nil {
		// Redis doesn't do, PostgreSQL will take over
		rec, err := app.FindActiveReservation(lineUserID)
		if err != nil {
			return nil, errors.New("No record found")
		}
		// save to redis before return
		err = app.SaveRecordToRedis(rec)
		if err != nil {
			return nil, err
		}
		return rec, nil
	}
	var rec ReservationRecord
	json.Unmarshal([]byte(result), &rec)
	return &rec, nil
}

// FindOrCreateRecord : this is the one to start everything
func (app *HailingApp) FindOrCreateRecord(lineUserID string) (*ReservationRecord, error) {
	// fmt.Println("Reserve: ", lineUserID)
	rec, err := app.FindRecord(lineUserID)
	if err != nil {
		return app.initReservation(lineUserID)
	}
	return rec, nil
}

func (app *HailingApp) initReservation(lineUserID string) (*ReservationRecord, error) {
	user, err := app.FindOrCreateUser(lineUserID)
	log.Printf("[initReservation] lineID: %v\n", lineUserID)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	log.Printf("[initReservation] ==> user: %v\n", user)
	return app.InitReservation(*user)
}

// InitReservation is a function to start reservation by User record
func (app *HailingApp) InitReservation(user User) (*ReservationRecord, error) {
	newRecord := ReservationRecord{
		UserID:     user.ID,
		LineUserID: user.LineUserID,
		State:      "init",
		Waiting:    "to",
		TripID:     -1,
	}

	err := app.SaveRecordToRedis(&newRecord)
	if err != nil {
		log.Printf("SAVE to Redis FAILED: %v", err)
		return nil, err
	}
	log.Printf("SAVE to Redis OK: %v", newRecord)
	return &newRecord, nil
}

func (app *HailingApp) QuickReplyLocations(record *ReservationRecord) []QuickReplyButton {
	// NOTE: it should consider user's history too actually
	results := []QuickReplyButton{}
	user, _, err := app.Localizer(record.LineUserID)
	if err != nil {
		return results
	}
	locations, err := app.GetLocations(user.Language, 4)
	if err != nil {
		return results
	}
	for _, loc := range locations {
		if loc.Place.Coordinates == record.FromCoords || loc.Place.Coordinates == record.ToCoords {
			continue
		}
		txt := strings.Join([]string{"loc", fmt.Sprintf("%d", loc.ID), loc.Name}, ":")
		// label must be less than 20-char
		re_citi := regexp.MustCompile(`(?i)citi resort`)
		label := re_citi.ReplaceAllString(loc.Name, "CITI")
		label = strings.ReplaceAll(label, "Sukhumvit 39", "")
		re_branch := regexp.MustCompile(`(?i)\(.*(\d)\)`)
		label = re_branch.ReplaceAllString(label, `$1`)
		re_space := regexp.MustCompile(` +`)
		label = re_space.ReplaceAllString(label, " ")

		results = append(results, QuickReplyButton{
			Label: label,
			Text:  txt,
		})

		if len(results) == 3 { // 3 records max
			break
		}
	}
	return results
}

// QuestionToAsk returns a question appropriate for each state
func (app *HailingApp) QuestionToAsk(record *ReservationRecord, localizer *i18n.Localizer) Question {
	// step: init -> to -> from -> when -> final -> done
	switch strings.ToLower(record.Waiting) {
	case "to":
		buttons := app.QuickReplyLocations(record)
		return Question{
			Text: localizer.MustLocalize(&i18n.LocalizeConfig{
				DefaultMessage: &i18n.Message{
					ID:    "WhereTo",
					Other: "Where to?",
				},
			}),
			Buttons:       buttons,
			LocationInput: false,
		}
	case "from":
		buttons := app.QuickReplyLocations(record)
		return Question{
			Text: localizer.MustLocalize(&i18n.LocalizeConfig{
				DefaultMessage: &i18n.Message{
					ID:    "PickupLocation",
					Other: "Pickup location?",
				},
			}),
			Buttons:       buttons,
			LocationInput: false,
		}
	case "when":
		buttons := []QuickReplyButton{
			{
				Label: "Now",
				Text:  "now",
			},
			{
				Label: localizer.MustLocalize(&i18n.LocalizeConfig{
					DefaultMessage: &i18n.Message{
						ID:    "InXMin",
						Other: "In {{.Min}} mins",
					},
					TemplateData: map[string]string{
						"Min": "15",
					},
				}),
				Text: "+15min",
			},
			{
				Label: localizer.MustLocalize(&i18n.LocalizeConfig{
					DefaultMessage: &i18n.Message{
						ID:    "InXMin",
						Other: "In {{.Min}} mins",
					},
					TemplateData: map[string]string{
						"Min": "30",
					},
				}),
				Text: "+30min",
			},
		}
		return Question{
			Text: localizer.MustLocalize(&i18n.LocalizeConfig{
				DefaultMessage: &i18n.Message{
					ID:    "When",
					Other: "When?",
				},
			}),
			Buttons:       buttons,
			DatetimeInput: true,
		}
	case "num_of_passengers":
		buttons := []QuickReplyButton{
			{
				Label: "1",
				Text:  "1",
			},
			{
				Label: "2",
				Text:  "2",
			},
			{
				Label: "3",
				Text:  "3",
			},
			{
				Label: "4",
				Text:  "4",
			},
		}
		return Question{
			Text: localizer.MustLocalize(&i18n.LocalizeConfig{
				DefaultMessage: &i18n.Message{
					ID:    "HowManyPassengers",
					Other: "How many passengers?",
				},
			}),
			Buttons: buttons,
		}
	case "final":
		return Question{
			Text: localizer.MustLocalize(&i18n.LocalizeConfig{
				DefaultMessage: &i18n.Message{
					ID:    "Confirm",
					Other: "Confirm",
				},
			}),
			YesInput: true,
		}
	}
	return Question{
		Text: "n/a",
	}
}

// IsLocation validates if the location is in the service area
func IsLocation(reply Reply) (bool, error) {
	/// TODO: handle "loc:15:Living @ CITI RESORT"
	if reply.Coords != [2]float64{0, 0} {
		b, _ := ioutil.ReadFile("./static/service_area.json")
		feature, _ := geojson.UnmarshalFeature(b)
		pnt := orb.Point(reply.Coords)
		// NOTE: change to postgis if accuracy needed
		if planar.PolygonContains(feature.Geometry.Bound().ToPolygon(), pnt) {
			return true, nil
		}
		return false, errors.New("Outside service area")
	}
	locPostback := strings.Split(reply.Text, ":")
	if len(locPostback) > 1 && locPostback[0] == "location" {
		return true, nil
	}
	// check for text if it's match
	if IsThisIn(strings.ToLower(reply.Text), TargetPlaces) {
		return true, nil
	}
	return false, errors.New("Not a location")
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
		// log.Printf("[isTime] %v \n", diffFromNow)
		return &t, errors.New("Time is in the past")
	}
	if diffFromNow.Hours() > 24 {
		// log.Printf("[isTime] %v \n", diffFromNow)
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

	log.Printf("[ProcessReservationStep] GOT record: %v", rec)

	switch rec.Waiting {
	case "from":
		_, err := IsLocation(reply)
		if err != nil {
			return rec, err
		}
		if reply.Coords != [2]float64{0, 0} {
			rec.From = "custom"
			rec.FromCoords = reply.Coords
		} else {
			locPostback := strings.Split(reply.Text, ":")
			if len(locPostback) > 1 && locPostback[0] == "location" {
				log.Printf("[ProcessReservationStep] location-postback: %v", reply.Text)
				ID, err := strconv.Atoi(locPostback[2])
				if err != nil {
					return rec, err
				}
				loc, err := app.GetLocationByID(ID)
				if err != nil {
					return rec, err
				}
				rec.From = loc.Name
				rec.FromCoords = loc.Place.Coordinates
			} else {
				log.Printf("[ProcessReservationStep] location-text: %v", reply.Text)
				rec.From = reply.Text
				rec.FromCoords = GetCoordsFromPlace(reply.Text)
			}
		}
	case "to":
		_, err := IsLocation(reply)
		if err != nil {
			return rec, err
		}
		if reply.Coords != [2]float64{0, 0} {
			rec.To = "custom"
			rec.ToCoords = reply.Coords
		} else {
			locPostback := strings.Split(reply.Text, ":")
			if len(locPostback) > 1 && locPostback[0] == "location" {
				log.Printf("[ProcessReservationStep] location-postback: %v", reply.Text)
				ID, err := strconv.Atoi(locPostback[2])
				if err != nil {
					return rec, err
				}
				loc, err := app.GetLocationByID(ID)
				if err != nil {
					return rec, err
				}
				rec.To = loc.Name
				rec.ToCoords = loc.Place.Coordinates
			} else {
				log.Printf("[ProcessReservationStep] location-text: %v", reply.Text)
				rec.To = reply.Text
				rec.ToCoords = GetCoordsFromPlace(reply.Text)
			}
		}
	case "when":
		tm, err := isTime(reply)
		if err != nil {
			log.Printf("[ProcessReservationStep] when: %v %v \n", tm, err)
			return rec, err
		}
		rec.ReservedAt = *tm
	case "num_of_passengers":
		num, err := strconv.Atoi(reply.Text)
		if err != nil {
			log.Printf("[ProcessReservationStep] num_of_passengers: '%v' is not number \n", reply.Text)
			return rec, err
		}
		rec.NumOfPassengers = num
	case "final":
		// if it's confirmed, then it's done
		var yesWords = []string{"last-step-confirmation", "confirm", "yes"}
		if IsThisIn(reply.Text, yesWords) {
			rec.IsConfirmed = true
		}
	case "done":
		// nothing to save here save record to postgres
	// case "pickup":  //
	// 1st case is "modify-pickup-time"
	// if reply.Text == "modify-pickup-time" {
	// 	tm, err := isTime(reply)
	// 	if err != nil {
	// 		return rec, err
	// 	}
	// 	rec.ReservedAt = *tm
	// }
	default:
		// or "pickup" state -- it could
		if reply.Text == "modify-pickup-time" {
			tm, err := isTime(reply)
			if err != nil {
				return rec, err
			}
			rec.ReservedAt = *tm
		} else {
			return rec, errors.New("Wrong state")
		}
	}
	log.Printf("[ProcessReservationStep] pre_status_change: %s \n   >> record: %v", rec.State, rec.UpdatedAt)

	rec.State = rec.Waiting
	rec.Waiting = rec.WhatsNext()
	rec.UpdatedAt = time.Now() // always show the last updated timestamp

	log.Printf("[ProcessReservationStep] mid_status_change: %s \n   >> record: %v", rec.State, rec.UpdatedAt)
	if rec.State == "done" {
		tripID, err := app.SaveReservationToPostgres(rec)
		if err != nil {
			return rec, err
		}
		rec.TripID = tripID
	}
	err = app.SaveRecordToRedis(rec)
	if err != nil {
		return nil, err
	}
	return rec, nil
}
