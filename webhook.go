package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/line/line-bot-sdk-go/linebot"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

// EventData struct
type EventData struct {
	Old Trip `json:"old"`
	New Trip `json:"new"`
}

// EventPayload struct
type EventPayload struct {
	SessionVariables map[string]json.RawMessage `json:"session_variables"`
	Op               string                     `json:"op"`
	Data             EventData                  `json:"data"`
}

// BodyPayload struct
type BodyPayload struct {
	Event        EventPayload               `json:"event"`
	CreatedAt    time.Time                  `json:"created_at"`
	ID           string                     `json:"id"`
	DeliveryInfo map[string]json.RawMessage `json:"delivery_info"`
	Trigger      map[string]json.RawMessage `json:"trigger"`
	Table        map[string]json.RawMessage `json:"table"`
}

// Response struct
type Response struct {
	Message string `json:"message"`
}

func jsonResponse(w http.ResponseWriter, httpCode int, resp Response) {
	w.WriteHeader(httpCode)
	w.Header().Set("Content-Type", "application/json")
	jMessage, _ := json.Marshal(resp)
	w.Write(jMessage)
}

// Webhook handles any request from Hasura server
func (app *HailingApp) Webhook(w http.ResponseWriter, req *http.Request) {
	method := req.Method
	if method != "POST" {
		errMessage := Response{Message: fmt.Sprintf("%s Method is not allowed", method)}
		jsonResponse(w, 400, errMessage)
		return
	}
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		panic(err)
	}
	var t BodyPayload
	err = json.Unmarshal(body, &t)
	if err != nil {
		log.Println("[WEBHOOK] Error JSON: ", err)
		errMessage := Response{Message: fmt.Sprintf("Wrong payload")}
		jsonResponse(w, 500, errMessage)
		return
	}
	if t.Event.Op != "UPDATE" {
		msg := Response{Message: "This operation doesn't need to do anything"}
		jsonResponse(w, 200, msg)
	}
	log.Println("[WEBHOOK] Old: ", t.Event.Data.Old)
	log.Println("[WEBHOOK] New: ", t.Event.Data.New)
	log.Println("[WEBHOOK] Trigger: ", t.Trigger["name"])

	// check what is updated: AcceptedAt, PickedUpAt, DroppedOffAt
	oldData := t.Event.Data.Old
	newData := t.Event.Data.New
	log.Printf("[WEBHOOK] event ID#%d | userID#%s\n", newData.ID, newData.UserID)
	user, _ := app.FindUserByID(newData.UserID)
	localizer := i18n.NewLocalizer(app.i18nBundle, user.Language)
	log.Printf("[WEBHOOK] user=%v, lang=%v\n", user.Username, user.Language)
	bkk, _ := time.LoadLocation("Asia/Bangkok")
	if oldData.AcceptedAt == nil && newData.AcceptedAt != nil {
		// notify
		bkkReservedTime := newData.ReservedAt.In(bkk)
		hhmm := localizer.MustLocalize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{
				ID:    "HHMM",
				Other: "at {{.hhmm}}.",
			},
			TemplateData: map[string]string{
				"hhmm": bkkReservedTime.Format(time.Kitchen),
			},
		})
		if time.Now().After(bkkReservedTime) {
			hhmm = localizer.MustLocalize(&i18n.LocalizeConfig{
				DefaultMessage: &i18n.Message{
					ID:    "ASAP",
					Other: "asap",
				},
			})
		}
		// TODO: add car name & color here too
		vehicle, err := app.GetActiveVehicleByDriverID(newData.DriverID)
		// default message whether there is vehicle info or not
		log.Printf("[WEBHOOK] vehicle=%v, err=%v\n", vehicle, err)
		txt := localizer.MustLocalize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{
				ID:    "DriverAcceptedJob",
				Other: "Driver accepts the job. Please meet at designated location {{.LocalTime}}",
			},
			TemplateData: map[string]string{
				"LocalTime": hhmm,
			},
		})
		msgs := []linebot.SendingMessage{
			linebot.NewTextMessage(txt),
		}
		if err == nil {
			log.Printf("[WEBHOOK] vehicle=%v, err=%v\n", vehicle, err)
			txt2 := localizer.MustLocalize(&i18n.LocalizeConfig{
				DefaultMessage: &i18n.Message{
					ID:    "VehicleWillPickYouUp",
					Other: "{{.VehicleName}}: No. {{.LicensePlate}} by driver {{.DriverName}}, will pick you up.",
				},
				TemplateData: map[string]string{
					"VehicleName": vehicle.Name,
					"DriverName": vehicle.DriverName,
					"LicensePlate": vehicle.LicensePlate,
				},
			})
			txt3 := fmt.Sprintf("%s \n%s", txt, txt2)
			msgs[0] = linebot.NewTextMessage(txt3)
		}
		app.PushNotification(user.LineUserID, msgs...)

	} else if oldData.PickedUpAt == nil && newData.PickedUpAt != nil {
		// Do not need to do anything
		welcome := localizer.MustLocalize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{
				ID:    "WelcomeAboard",
				Other: "Welcome aboard!",
			},
		})
		msg := linebot.NewTextMessage(welcome)
		app.PushNotification(user.LineUserID, msg)

	} else if oldData.DroppedOffAt == nil && newData.DroppedOffAt != nil {
		var record *ReservationRecord
		var questions []Questionaire
		// send feedback form
		// msg := linebot.NewTextMessage("Ride is done, any feedback?")
		// TODO: should get tripID and pass along too
		// tripID := newData.ID
		// msg := app.StarFeedbackFlex(tripID, localizer)
		record, err = app.FindRecord(user.LineUserID)
		if err != nil {
			log.Println("[WEBHOOK] Error FindRecord: ", err)
			errMessage := Response{Message: fmt.Sprintf("No Record")}
			jsonResponse(w, 500, errMessage)
		}
		record.State = "jobOver"
		questions, err = app.GetQuestions(user.Language)
		if err != nil {
			log.Println("[WEBHOOK] Error GetQuestions: ", err)
			errMessage := Response{Message: fmt.Sprintf("No Record")}
			jsonResponse(w, 500, errMessage)
		}
		record.QList = questions
		record.QState = 0
		// record.QState = questions[0].ID
		err = app.SaveRecordToRedis(record)
		if err != nil {
			log.Println("[WEBHOOK] Error SaveToRedis: ", err)
			errMessage := Response{Message: fmt.Sprintf("No Record")}
			jsonResponse(w, 500, errMessage)
		}
		// msg := app.StarFeedbackFlex(tripID, localizer)
		msg := app.QuestionFlex(record.QList[record.QState],record.TripID,user.Language)
		app.PushNotification(user.LineUserID, msg)
		// app.Cleanup(user.LineUserID)
	}

	msg := Response{Message: "success"}
	jsonResponse(w, 200, msg)
}
