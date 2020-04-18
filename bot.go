package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/line/line-bot-sdk-go/linebot"
)

/* This bot will take care of message coming through line bot basically I find that there are 2 types we will have
* EventTypeMessage  - any message from user will belong here
* EventTypePostback - this is from the buttons and such.
	linebot.NewPostbackAction("言 hello2", "hello こんにちは", "hello こんにちは", ""),
	   PostbackAction struct {
			Label       string
			Data        string
			Text        string
			DisplayText string
		}
	linebot.NewDatetimePickerAction("datetime", "DATETIME", "datetime", "", "", ""),
		type DatetimePickerAction struct {
			Label   string
			Data    string
			Mode    string
			Initial string
			Max     string
			Min     string
		}

*/

// Callback function for linebot
func (app *HailingApp) Callback(w http.ResponseWriter, r *http.Request) {
	events, err := app.bot.ParseRequest(r)
	if err != nil {
		if err == linebot.ErrInvalidSignature {
			w.WriteHeader(400)
		} else {
			w.WriteHeader(500)
		}
		return
	}

	for _, event := range events {
		fmt.Print(event)
		switch event.Type {
		case linebot.EventTypeMessage:
			app.extractReplyFromMessage(event)
		case linebot.EventTypePostback:
			app.extractReplyFromPostback(event)
		default:
			log.Printf("Unknown event: %v", event)
		}
	}
}

// extractReplyFromPostback will convert event.Message to Reply for the next process
func (app *HailingApp) extractReplyFromPostback(event *linebot.Event) error {

	data := event.Postback.Data
	userID := event.Source.UserID
	var reply Reply
	switch strings.ToLower(data) {
	case "from":
	case "to":
		msg := fmt.Sprintf("%v", event.Postback.Params)
		reply = Reply{Text: msg}
	case "when":
		layout := "2006-01-02T15:04:05.000Z"
		str := fmt.Sprintf("%v+07:00", event.Postback.Params.Datetime)
		t, err := time.Parse(layout, str)
		if err != nil {
			log.Print(err)
		}
		reply = Reply{Datetime: t}
	default:
		if err := app.replyText(event.ReplyToken, "Got postback: "+data); err != nil {
			log.Print(err)
		}
	}

	if err := app.handleNextStep(event.ReplyToken, userID, reply); err != nil {
		return err
	}
	return nil
}

// extractReplyFromMessage will convert event.Message to Reply for the next process
func (app *HailingApp) extractReplyFromMessage(event *linebot.Event) error {
	reply := Reply{}
	userID := event.Source.UserID
	// question := record.QuestionToAsk()
	switch message := event.Message.(type) {
	case *linebot.TextMessage:
		txt := event.Message.(*linebot.TextMessage)
		reply.Text = txt.Text
	case *linebot.LocationMessage:
		loc := event.Message.(*linebot.LocationMessage)
		reply.Coords = [2]float64{loc.Longitude, loc.Latitude}
	case *linebot.StickerMessage:
		sticker := event.Message.(*linebot.StickerMessage)
		reply.Text = sticker.StickerID
	default:
		log.Printf("Unknown message: %v", message)
		txt := fmt.Sprintf("Got message: %v", event.Message)
		if err := app.replyText(event.ReplyToken, txt); err != nil {
			log.Print(err)
		}
	}
	if err := app.handleNextStep(event.ReplyToken, userID, reply); err != nil {
		return err
	}
	return nil
}

func (app *HailingApp) handleNextStep(replyToken string, userID string, reply Reply) error {
	record, err := app.ProcessReservationStep(userID, reply)
	if err != nil {
		return err
	}

	question := record.QuestionToAsk()
	if err := app.replyBack(replyToken, question); err != nil {
		return err
	}

	return nil
}

func (app *HailingApp) replyBack(replyToken string, question Question) error {
	replyItems := linebot.NewQuickReplyItems()
	itemTotal := len(question.Buttons)
	if question.LocationInput {
		itemTotal++
	}
	if question.DatetimeInput {
		itemTotal++
	}
	items := make([]*linebot.QuickReplyButton, itemTotal)
	ind := 0
	for i := 0; i < len(question.Buttons); i++ {
		btn := question.Buttons[i]
		qrBtn := linebot.NewQuickReplyButton(
			app.appBaseURL+"/static/quick/pin.svg",
			linebot.NewMessageAction(btn.Label, btn.Text),
		)
		items[i] = qrBtn
		ind++
	}
	if question.LocationInput == true {
		items[ind] = linebot.NewQuickReplyButton(
			"",
			linebot.NewLocationAction("Send location"))
		ind++
	}
	if question.DatetimeInput == true {
		items[ind] = linebot.NewQuickReplyButton(
			"",
			linebot.NewDatetimePickerAction("datetime", "DATETIME", "datetime", "", "", ""))
	}
	replyItems.Items = items

	if _, err := app.bot.ReplyMessage(
		replyToken,
		linebot.NewTextMessage(question.Text).
			WithQuickReplies(replyItems),
	).Do(); err != nil {
		return err
	}
	return nil
}

func (app *HailingApp) replyText(replyToken, text string) error {
	if _, err := app.bot.ReplyMessage(
		replyToken,
		linebot.NewTextMessage(text),
	).Do(); err != nil {
		return err
	}
	return nil
}
