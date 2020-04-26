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
		log.Println("[Callback] ", event)
		switch event.Type {
		case linebot.EventTypeMessage:
			app.extractReplyFromMessage(event)
		case linebot.EventTypePostback:
			app.extractReplyFromPostback(event)
		default:
			log.Printf("[Callback] Unknown event: %v\n", event)
		}
	}
}

// extractReplyFromPostback will convert event.Message to Reply for the next process
func (app *HailingApp) extractReplyFromPostback(event *linebot.Event) error {

	data := event.Postback.Data
	userID := event.Source.UserID
	log.Printf("[PostbackExtractor] %v\n     %v", data, event.Postback)
	var reply Reply
	switch strings.ToLower(data) {
	case "init":
		// NOTE: this should go to handleNextStep automatically
		reply = Reply{Text: "call the cab"}
	case "cancel":
		reply = Reply{Text: "cancel"}
	case "from":
		msg := fmt.Sprintf("%v", event.Postback.Params)
		reply = Reply{Text: msg}
	case "to":
		msg := fmt.Sprintf("%v", event.Postback.Params)
		reply = Reply{Text: msg}
	case "datetime-change":
		// TODO: implement this --> change time after it's already done
		layout := "2006-01-02T15:04-07:00"
		str := fmt.Sprintf("%v+07:00", event.Postback.Params.Datetime)
		log.Printf("[PostbackExtractor] datetime: %v\n", event.Postback.Params)
		t, err := time.Parse(layout, str)
		if err != nil {
			log.Println(err)
		}
		reply = Reply{Text: "modify-pickup-time", Datetime: t}
	case "datetime":
		layout := "2006-01-02T15:04-07:00"
		str := fmt.Sprintf("%v+07:00", event.Postback.Params.Datetime)
		log.Printf("[PostbackExtractor] datetime: %v\n", event.Postback.Params)
		t, err := time.Parse(layout, str)
		if err != nil {
			log.Println(err)
		}
		reply = Reply{Text: "datetime", Datetime: t}
		log.Printf("[PostbackExtractor] datetime result = %v\n", reply.Datetime.Format(time.RubyDate))
	default:
		log.Printf("[PostbackExtractor] unhandled case\n")
		msg := fmt.Sprintf("Got postback: %v", data)
		if err := app.replyText(event.ReplyToken, msg); err != nil {
			log.Println(err)
		}
	}

	log.Printf("[PostbackExtractor] reply: %v\n", reply)
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
			log.Println(err)
		}
	}
	if err := app.handleNextStep(event.ReplyToken, userID, reply); err != nil {
		return err
	}
	return nil
}

func (app *HailingApp) handleNextStep(replyToken string, userID string, reply Reply) error {
	var record *ReservationRecord
	var err error
	msgs := []string{"", ""}

	if reply.Text == "modify-pickup-time" {
		// TODO: deal with this case -- which I am not sure how yet.
		record, err = app.ProcessReservationStep(userID, reply)
		if err != nil {
			// this supposes to ask the same question again.
			log.Printf("[handleNextStep] reply incorrectly: %v", err)
			msgs[0] = "Error, try again"
		}
	}

	if IsThisIn(reply.Text, WordsToCancel) {
		total, err := app.Cancel(userID)
		if err != nil {
			return err
		}
		msg := fmt.Sprintf("Your reservation cancelled [%v]", total)
		if _, err := app.bot.ReplyMessage(
			replyToken,
			linebot.NewTextMessage(msg),
		).Do(); err != nil {
			return err
		}
		return nil
	}

	if IsThisIn(reply.Text, WordsToAskForStatus) {
		record, err = app.FindRecord(userID)
		if err != nil {
			// tell user first:
			// (1) what is wrong
			// (2) wanna start reservation record?
			errMsg := fmt.Sprintf("%v", err)
			if _, err := app.bot.ReplyMessage(
				replyToken,
				linebot.NewTextMessage(errMsg),
				WannaStart(),
			).Do(); err != nil {
				return err
			}
			return nil
		}
		log.Printf("[handleNextStep] status query: %s \n   >> record: %v", record.State, record)
		// msg := fmt.Sprintf("Your reservation detail is here [%v]", record)
		if _, err := app.bot.ReplyMessage(
			replyToken,
			record.RecordConfirmFlex(),
		).Do(); err != nil {
			return err
		}
		return nil
	}

	if IsThisIn(reply.Text, WordsToInit) { // initial state
		record, err = app.FindOrCreateRecord(userID)
		if err != nil {
			return err
		}
	} else {
		record, err = app.ProcessReservationStep(userID, reply)
		if err != nil {
			// this supposes to ask the same question again.
			log.Printf("[handleNextStep] reply incorrectly: %v", err)
			msgs[0] = fmt.Sprintf("Error, try again")
			msgs[1] = fmt.Sprintf("%v", err)
		}
	}

	log.Printf("[handleNextStep] %v\n   PrevReply = %v", record, reply)
	if record.State == "done" {
		// this need special care
		if _, err := app.bot.ReplyMessage(
			replyToken,
			linebot.NewTextMessage("Your ride reservation is done."),
			record.RecordConfirmFlex(),
		).Do(); err != nil {
			return err
		}
		return nil
	}
	question := record.QuestionToAsk()
	if err := app.replyBack(replyToken, question, msgs...); err != nil {
		return err
	}
	log.Println("[handleNextStep] ", record)
	return nil
}

func (app *HailingApp) replyBack(replyToken string, question Question, messages ...string) error {
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
			linebot.NewDatetimePickerAction("Pick date & time", "DATETIME", "datetime", "", "", ""))
	}
	replyItems.Items = items
	sendingMsgs := []linebot.SendingMessage{}
	for i := 0; i < len(messages); i++ {
		if messages[i] != "" {
			sendingMsgs = append(sendingMsgs, linebot.NewTextMessage(messages[i]))
		}
	}
	// ask question
	sendingMsgs = append(sendingMsgs, linebot.NewTextMessage(question.Text).WithQuickReplies(replyItems))

	if _, err := app.bot.ReplyMessage(
		replyToken,
		sendingMsgs...,
	).Do(); err != nil {
		return err
	}
	return nil
}

func (app *HailingApp) replyText(replyToken string, text ...string) error {
	msgs := make([]linebot.SendingMessage, len(text))
	for i := 0; i < len(text); i++ {
		msgs[i] = linebot.NewTextMessage(text[i])
	}
	if _, err := app.bot.ReplyMessage(
		replyToken,
		msgs...,
	).Do(); err != nil {
		return err
	}
	return nil
}

func (app *HailingApp) replyMessage(replyToken string, messages ...linebot.SendingMessage) error {
	if _, err := app.bot.ReplyMessage(
		replyToken,
		messages...,
	).Do(); err != nil {
		return err
	}
	return nil
}

// WannaStart returns sendingMessage to ask if user want to start the process
func WannaStart() linebot.SendingMessage {
	contents := &linebot.BubbleContainer{
		Type: linebot.FlexContainerTypeBubble,
		Body: &linebot.BoxComponent{
			Type:   linebot.FlexComponentTypeBox,
			Layout: linebot.FlexBoxLayoutTypeVertical,
			Contents: []linebot.FlexComponent{
				&linebot.TextComponent{
					Type:   linebot.FlexComponentTypeText,
					Text:   "Need a ride now?",
					Weight: linebot.FlexTextWeightTypeBold,
					Size:   linebot.FlexTextSizeTypeXl,
				},
			},
		},
		Footer: &linebot.BoxComponent{
			Type:   linebot.FlexComponentTypeBox,
			Layout: linebot.FlexBoxLayoutTypeVertical,
			Contents: []linebot.FlexComponent{
				&linebot.BoxComponent{
					Type:   linebot.FlexComponentTypeBox,
					Layout: linebot.FlexBoxLayoutTypeVertical,
					Contents: []linebot.FlexComponent{
						&linebot.ButtonComponent{
							Height: linebot.FlexButtonHeightTypeMd,
							Style:  linebot.FlexButtonStyleTypePrimary,
							Action: linebot.NewPostbackAction("Yes", "init", "", ""),
						},
					},
				},
			},
		},
	}
	return linebot.NewFlexMessage("Start reservation", contents)
}

// RecordConfirmFlex to return information in form of FLEX
func (record *ReservationRecord) RecordConfirmFlex() linebot.SendingMessage {
	flexLabel := 2
	flexDesc := 8
	flexBtnLeft := 3
	flexBtnRight := 6
	contents := &linebot.BubbleContainer{
		Type: linebot.FlexContainerTypeBubble,
		Body: &linebot.BoxComponent{
			Type:   linebot.FlexComponentTypeBox,
			Layout: linebot.FlexBoxLayoutTypeVertical,
			Contents: []linebot.FlexComponent{
				&linebot.TextComponent{
					Type:   linebot.FlexComponentTypeText,
					Text:   "Ride confirmation",
					Weight: linebot.FlexTextWeightTypeBold,
					Size:   linebot.FlexTextSizeTypeXl,
				},
				&linebot.BoxComponent{
					Type:    linebot.FlexComponentTypeBox,
					Layout:  linebot.FlexBoxLayoutTypeBaseline,
					Spacing: linebot.FlexComponentSpacingTypeXs,
					Margin:  linebot.FlexComponentMarginTypeXl,
					Contents: []linebot.FlexComponent{
						&linebot.TextComponent{
							Type:   linebot.FlexComponentTypeText,
							Text:   "Pickup",
							Weight: linebot.FlexTextWeightTypeRegular,
							Flex:   &flexLabel,
							Size:   linebot.FlexTextSizeTypeSm,
						},
						&linebot.TextComponent{
							Type:   linebot.FlexComponentTypeText,
							Text:   record.From,
							Weight: linebot.FlexTextWeightTypeRegular,
							Flex:   &flexDesc,
							Size:   linebot.FlexTextSizeTypeSm,
						},
					},
				},
				&linebot.BoxComponent{
					Type:    linebot.FlexComponentTypeBox,
					Layout:  linebot.FlexBoxLayoutTypeBaseline,
					Spacing: linebot.FlexComponentSpacingTypeXs,
					Margin:  linebot.FlexComponentMarginTypeXl,
					Contents: []linebot.FlexComponent{
						&linebot.TextComponent{
							Type:   linebot.FlexComponentTypeText,
							Text:   "To",
							Weight: linebot.FlexTextWeightTypeRegular,
							Flex:   &flexLabel,
							Size:   linebot.FlexTextSizeTypeSm,
						},
						&linebot.TextComponent{
							Type:   linebot.FlexComponentTypeText,
							Text:   record.To,
							Weight: linebot.FlexTextWeightTypeRegular,
							Flex:   &flexDesc,
							Size:   linebot.FlexTextSizeTypeSm,
						},
					},
				},
				&linebot.BoxComponent{
					Type:    linebot.FlexComponentTypeBox,
					Layout:  linebot.FlexBoxLayoutTypeBaseline,
					Spacing: linebot.FlexComponentSpacingTypeXs,
					Margin:  linebot.FlexComponentMarginTypeXl,
					Contents: []linebot.FlexComponent{
						&linebot.TextComponent{
							Type:   linebot.FlexComponentTypeText,
							Text:   "Time",
							Weight: linebot.FlexTextWeightTypeRegular,
							Flex:   &flexLabel,
							Size:   linebot.FlexTextSizeTypeSm,
						},
						&linebot.TextComponent{
							Type:   linebot.FlexComponentTypeText,
							Text:   record.ReservedAt.Format(time.Kitchen),
							Weight: linebot.FlexTextWeightTypeRegular,
							Flex:   &flexDesc,
							Size:   linebot.FlexTextSizeTypeSm,
						},
					},
				},
			},
		},
		Footer: &linebot.BoxComponent{
			Type:   linebot.FlexComponentTypeBox,
			Layout: linebot.FlexBoxLayoutTypeVertical,
			Contents: []linebot.FlexComponent{
				&linebot.ButtonComponent{
					Height: linebot.FlexButtonHeightTypeMd,
					Style:  linebot.FlexButtonStyleTypeLink,
					Action: linebot.NewPostbackAction("Call driver", "call", "", ""),
				},

				&linebot.BoxComponent{
					Type:   linebot.FlexComponentTypeBox,
					Layout: linebot.FlexBoxLayoutTypeHorizontal,
					Contents: []linebot.FlexComponent{
						&linebot.ButtonComponent{
							Height: linebot.FlexButtonHeightTypeMd,
							Style:  linebot.FlexButtonStyleTypeSecondary,
							Flex:   &flexBtnLeft,
							Action: linebot.NewPostbackAction("Cancel", "cancel", "", ""),
						},
						&linebot.ButtonComponent{
							Height: linebot.FlexButtonHeightTypeMd,
							Style:  linebot.FlexButtonStyleTypePrimary,
							Flex:   &flexBtnRight,
							Action: linebot.NewDatetimePickerAction(
								"Change pickup time", "datetime-change", "datetime",
								"", "", ""),
						},
					},
				},
			},
		},
	}
	return linebot.NewFlexMessage("Ride confirmation", contents)
}
