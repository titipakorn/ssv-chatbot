package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/line/line-bot-sdk-go/linebot"
	"github.com/nicksnyder/go-i18n/v2/i18n"
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

	postbackType := strings.Split(data, ":")
	lineUserID := event.Source.UserID
	log.Printf("[PostbackExtractor] %v\n     %v", data, event.Postback)
	var reply Reply

	switch strings.ToLower(postbackType[0]) {
	case "init":
		// NOTE: this should go to handleNextStep automatically
		reply = Reply{Text: "call the cab"}
	case "confirm":
		// NOTE: this should go to handleNextStep automatically
		reply = Reply{Text: "last-step-confirmation"}
	case "cancel":
		reply = Reply{Text: "cancel"}
	case "from":
		msg := fmt.Sprintf("%v", event.Postback.Params)
		reply = Reply{Text: msg}
	case "to":
		msg := fmt.Sprintf("%v", event.Postback.Params)
		reply = Reply{Text: msg}
	case "datetime-change":
		layout := "2006-01-02T15:04-07:00"
		str := fmt.Sprintf("%v+07:00", event.Postback.Params.Datetime)
		log.Printf("[PostbackExtractor] datetime: %v\n", event.Postback.Params)
		t, err := time.Parse(layout, str)
		if err != nil {
			log.Println(err)
		}
		reply = Reply{Text: "modify-pickup-time", Datetime: t}
	case "location-options":
		reply = Reply{Text: "location-options"}
	case "location":
		if len(postbackType) > 1 {
			reply = Reply{Text: data} // include everything e.g. location:BTSxxx
		} else {
			return app.UnhandledCase(event.ReplyToken)
		}
	case "star-feedback":
		if len(postbackType) != 3 {
			log.Printf("[PostbackExtractor] star-feedback unhandled case : data: %v\n", data)
			return app.UnhandledCase(event.ReplyToken)
		}
		return app.FeedbackHandler(event.ReplyToken, lineUserID, postbackType[1], postbackType[2])
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
		return app.UnhandledCase(event.ReplyToken)
	}

	log.Printf("[PostbackExtractor] reply: %v\n", reply)
	if err := app.handleNextStep(event.ReplyToken, lineUserID, reply); err != nil {
		return err
	}
	return nil
}

// extractReplyFromMessage will convert event.Message to Reply for the next process
func (app *HailingApp) extractReplyFromMessage(event *linebot.Event) error {
	reply := Reply{}
	lineUserID := event.Source.UserID
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
		return app.UnhandledCase(event.ReplyToken)
	default:
		log.Printf("Unknown message: %v", message)
		// txt := fmt.Sprintf("Got message: %v", event.Message)
		// if err := app.replyText(event.ReplyToken, txt); err != nil {
		// 	log.Println(err)
		// }
		return app.UnhandledCase(event.ReplyToken)
	}
	log.Printf("[MessageExtractor] reply: %v\n", reply)
	if err := app.handleNextStep(event.ReplyToken, lineUserID, reply); err != nil {
		return err
	}
	return nil
}

// UnhandledCase return greeting and some initial suggestion to the service
func (app *HailingApp) UnhandledCase(replyToken string) error {

	if _, err := app.bot.ReplyMessage(
		replyToken,
		linebot.NewTextMessage("Hi there, do you need a ride?"),
		linebot.NewTextMessage("Try \"status\" to get started").WithQuickReplies(
			linebot.NewQuickReplyItems(
				linebot.NewQuickReplyButton(
					app.appBaseURL+"/static/quick/pin.png",
					linebot.NewMessageAction("Help", "help"),
				),
			),
		),
	).Do(); err != nil {
		return err
	}
	return nil
}

func (app *HailingApp) handleNextStep(replyToken string, lineUserID string, reply Reply) error {
	var record *ReservationRecord
	var err error
	msgs := []string{"", ""}

	if strings.Contains(reply.Text, "[LIFF]") {
		return app.LIFFHandler(replyToken, lineUserID, reply)
	}

	if strings.HasPrefix(reply.Text, "/") {
		return app.BotCommandHandler(replyToken, lineUserID, reply)
	}

	_, localizer, err := app.Localizer(lineUserID)
	if err != nil {
		return app.replyText(err.Error())
	}

	// location options
	if reply.Text == "location-options" {
		askLocation := localizer.MustLocalize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{
				ID:    "AskForLocation",
				Other: "Or send me your location!",
			},
		})
		log.Printf("[handleNextStep] location-option\n")
		if _, err := app.bot.ReplyMessage(
			replyToken,
			app.LocationOptionFlex(localizer),
			linebot.NewTextMessage(askLocation).
				WithQuickReplies(linebot.NewQuickReplyItems(
					linebot.NewQuickReplyButton(
						app.appBaseURL+"/static/quick/pin.png",
						linebot.NewLocationAction("Send location")),
				)),
		).Do(); err != nil {
			log.Printf("[handleNextStep] location-option err: %v", err)
			return err
		}
		return nil
	}

	// change pickup time
	if reply.Text == "modify-pickup-time" {
		nothingChanged := localizer.MustLocalize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{
				ID:    "NothingChanged",
				Other: "Error, nothing changed",
			},
		})
		// TODO: deal with this case -- which I am not sure how yet.
		record, err = app.ProcessReservationStep(lineUserID, reply)
		if err != nil {
			// this supposes to ask the same question again.
			// log.Printf("[handleNextStep] reply incorrectly: %v", err)
			// TODO: since it's "done" state, we need to return Message here
			if _, err := app.bot.ReplyMessage(
				replyToken,
				linebot.NewTextMessage(nothingChanged),
				linebot.NewTextMessage(fmt.Sprintf("%v", err)),
			).Do(); err != nil {
				return err
			}
			return nil
		}
	}

	// cancel process
	if IsThisIn(reply.Text, WordsToCancel) {
		return app.CancelHandler(replyToken, lineUserID)
	}

	initLine := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "RideInitLine",
			Other: "Need a ride now?",
		},
	})
	yes := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "Yes",
			Other: "Yes",
		},
	})
	confirm := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "RideConfirmation",
			Other: "Ride confirmation",
		},
	})
	rideIncompleted := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "ReservationIncompleted",
			Other: "The reservation isn't completed yet.",
		},
	})
	rideCompleted := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "RideReservationCompleted",
			Other: "Your ride reservation is done.",
		},
	})

	// status process
	if IsThisIn(reply.Text, WordsToAskForStatus) {
		record, err = app.FindRecord(lineUserID)
		if err != nil {
			// log.Printf("[handleNextStep] err status: %v\n", err)
			// tell user first:
			// (1) what is wrong
			// (2) wanna start reservation record?
			errMsg := fmt.Sprintf("%v", err)
			if _, err2 := app.bot.ReplyMessage(
				replyToken,
				linebot.NewTextMessage(fmt.Sprintf("%v", errMsg)),
				ConfirmDialog(initLine, yes, "init"),
			).Do(); err2 != nil {
				return err2
			}
			return nil
		}
		// check if it's done or not
		done, _ := record.IsComplete()
		if done {
			// log.Printf("[handleNextStep] status query: %s \n   >> record: %v", record.State, record)
			// msg := fmt.Sprintf("Your reservation detail is here [%v]", record)
			if _, err := app.bot.ReplyMessage(
				replyToken,
				record.RecordConfirmFlex(confirm, localizer),
			).Do(); err != nil {
				return err
			}
			return nil
		}
		// if it's not done, let this go through regular process
		msgs[0] = rideIncompleted
		return app.replyQuestion(replyToken, localizer, record, msgs...)
	}

	// initial state
	if IsThisIn(reply.Text, WordsToInit) {
		// log.Printf("[handleNextStep] init (user: %v)\n", lineUserID)
		record, err = app.FindOrCreateRecord(lineUserID)
		if err != nil {
			return err
		}
		// log.Printf("[handleNextStep] init:record => %v \n", record)
	} else {
		// if found --> Process
		// NOT --> Ask wanna start?
		record, err = app.FindRecord(lineUserID)
		if err != nil {
			if _, err2 := app.bot.ReplyMessage(
				replyToken,
				linebot.NewTextMessage(fmt.Sprintf("%v", err)),
				ConfirmDialog(initLine, yes, "init"),
			).Do(); err2 != nil {
				return err2
			}
			return nil
		}
		record, err = app.ProcessReservationStep(lineUserID, reply)
		if err != nil {
			// this supposes to ask the same question again.
			// log.Printf("[handleNextStep] reply incorrectly: %v", err)
			// msgs[0] = fmt.Sprintf("Error, try again")
			msgs[0] = fmt.Sprintf("%v", err)
		}
	}

	// log.Printf("[handleNextStep] %v\n   PrevReply = %v", record, reply)
	if record.State == "done" {
		// this need special care
		if _, err := app.bot.ReplyMessage(
			replyToken,
			linebot.NewTextMessage(rideCompleted),
			record.RecordConfirmFlex(confirm, localizer),
		).Do(); err != nil {
			return err
		}
		return nil
	}
	return app.replyQuestion(replyToken, localizer, record, msgs...)
}

func (app *HailingApp) replyQuestion(replyToken string, localizer *i18n.Localizer, record *ReservationRecord, msgs ...string) error {
	question := record.QuestionToAsk(localizer)
	if question.YesInput == true {
		return app.replyFinalStep(replyToken, localizer, record)
	}
	if question.Text == "When?" {
		return app.replyTravelTimeOptionsAndWhen(replyToken, record, question)
	}
	// regular question flow
	if err := app.replyBack(replyToken, question, msgs...); err != nil {
		return err
	}
	return nil
}

func (app *HailingApp) replyTravelTimeOptionsAndWhen(replyToken string, record *ReservationRecord, question Question) error {
	items := make([]*linebot.QuickReplyButton, len(question.Buttons))
	ind := 0
	for i := 0; i < len(question.Buttons); i++ {
		btn := question.Buttons[i]
		qrBtn := linebot.NewQuickReplyButton(
			app.appBaseURL+"/static/quick/pin.png",
			linebot.NewMessageAction(btn.Label, btn.Text),
		)
		items[i] = qrBtn
		ind++
	}
	replyItems := linebot.NewQuickReplyItems()
	replyItems.Items = items

	sendingMsgs := []linebot.SendingMessage{}
	// don't give anything since for duration w/traffic requires time obviously
	// sendingMsgs = append(sendingMsgs, app.EstimatedTravelTimeFlex(record))
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

func (app *HailingApp) replyFinalStep(replyToken string, localizer *i18n.Localizer, record *ReservationRecord) error {

	btnAction := linebot.NewPostbackAction("Confirm", "confirm", "", "")
	if _, err := app.bot.ReplyMessage(
		replyToken,
		// linebot.NewTextMessage(optionTxt),
		app.EstimatedTravelTimeFlex(record, btnAction, localizer),
	).Do(); err != nil {
		return err
	}
	return nil
}

func (app *HailingApp) replyBack(replyToken string, question Question, messages ...string) error {

	replyItems := linebot.NewQuickReplyItems()
	itemTotal := len(question.Buttons)
	if question.LocationInput {
		itemTotal = itemTotal + 2
		// 1. send location, 2. more options
	}
	if question.DatetimeInput {
		itemTotal++
	}
	items := make([]*linebot.QuickReplyButton, itemTotal)
	ind := 0
	for i := 0; i < len(question.Buttons); i++ {
		btn := question.Buttons[i]
		qrBtn := linebot.NewQuickReplyButton(
			app.appBaseURL+"/static/quick/pin.png",
			linebot.NewMessageAction(btn.Label, btn.Text),
		)
		items[i] = qrBtn
		ind++
	}
	if question.LocationInput == true {
		// pickup from map
		items[ind] = linebot.NewQuickReplyButton(
			app.appBaseURL+"/static/quick/pin.png",
			linebot.NewLocationAction("Send location"),
		)
		ind++
		// more options
		items[ind] = linebot.NewQuickReplyButton(
			"",
			linebot.NewPostbackAction("More options", "location-options", "", ""),
		)
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

// LocationOptionFlex to send location options
func (app *HailingApp) LocationOptionFlex(localizer *i18n.Localizer) linebot.SendingMessage {
	locs, err := app.GetLocations(10)
	if err != nil {
		log.Printf("[LocationOptionFlex] db failed: %v\n", err)
		return nil
	}

	picker := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "PickFromListBelow",
			Other: "Pick from the list below",
		},
	})
	altText := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "LocationOptions",
			Other: "Location options",
		},
	})

	items := []linebot.FlexComponent{
		&linebot.TextComponent{
			Type:   linebot.FlexComponentTypeText,
			Text:   picker,
			Weight: linebot.FlexTextWeightTypeBold,
			Size:   linebot.FlexTextSizeTypeMd,
		}}
	for _, location := range locs {
		items = append(items, &linebot.ButtonComponent{
			Height: linebot.FlexButtonHeightTypeSm,
			Style:  linebot.FlexButtonStyleTypeLink,
			Action: linebot.NewPostbackAction(
				location.Name,
				fmt.Sprintf("location:%s:%d", location.Name, location.ID),
				"", ""),
		})
	}

	contents := &linebot.BubbleContainer{
		Type: linebot.FlexContainerTypeBubble,
		Body: &linebot.BoxComponent{
			Type:     linebot.FlexComponentTypeBox,
			Layout:   linebot.FlexBoxLayoutTypeVertical,
			Contents: items,
		},
	}
	return linebot.NewFlexMessage(altText, contents)
}

// ConfirmDialog returns sendingMessage to ask if user want to start the process
func ConfirmDialog(message string, postbackLabel string, postbackData string) linebot.SendingMessage {
	contents := &linebot.BubbleContainer{
		Type: linebot.FlexContainerTypeBubble,
		Body: &linebot.BoxComponent{
			Type:   linebot.FlexComponentTypeBox,
			Layout: linebot.FlexBoxLayoutTypeVertical,
			Contents: []linebot.FlexComponent{
				&linebot.TextComponent{
					Type:   linebot.FlexComponentTypeText,
					Text:   message,
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
							Action: linebot.NewPostbackAction(postbackLabel, postbackData, "", ""),
						},
					},
				},
			},
		},
	}
	return linebot.NewFlexMessage("Start reservation", contents)
}

func (app *HailingApp) travelOption(icon string, distance string, duration string) *linebot.BoxComponent {
	flex0 := 0
	primaryColor := "#000000"
	secondaryColor := "#AAAAAA"
	log.Printf("icon --> %s/static/%s-100x100.png", app.appBaseURL, icon)
	return &linebot.BoxComponent{
		Layout: linebot.FlexBoxLayoutTypeBaseline,
		Contents: []linebot.FlexComponent{
			&linebot.IconComponent{
				URL:  fmt.Sprintf("%s/static/%s-100x100.png", app.appBaseURL, icon),
				Size: linebot.FlexIconSizeType3xl,
			},
			&linebot.TextComponent{
				Type:    linebot.FlexComponentTypeText,
				Text:    distance,
				Flex:    &flex0,
				Margin:  linebot.FlexComponentMarginTypeSm,
				Weight:  linebot.FlexTextWeightTypeRegular,
				Color:   secondaryColor,
				Wrap:    true,
				Gravity: linebot.FlexComponentGravityTypeCenter,
			},
			&linebot.TextComponent{
				Type:  linebot.FlexComponentTypeText,
				Text:  duration,
				Size:  linebot.FlexTextSizeTypeXl,
				Align: linebot.FlexComponentAlignTypeEnd,
				Color: primaryColor,
			},
		},
	}
}

// StarFeedbackFlex lets user rating the service
func (app *HailingApp) StarFeedbackFlex(tripID int, localizer *i18n.Localizer) linebot.SendingMessage {
	trip, err := app.GetTripRecordByID(tripID)
	if err != nil {
		return nil
	}

	title := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "RideIsDone",
			Other: "The ride is done.",
		},
	})
	question := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "HowDoYouLikeService",
			Other: "How do you like our service this time?",
		},
	})
	pickup := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "Pickup",
			Other: "Pickup",
		},
	})
	to := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "To",
			Other: "To",
		},
	})
	durationLabel := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "Duration",
			Other: "Duration",
		},
	})

	flexLabel := 3
	flexDesc := 7
	duration := trip.DroppedOffAt.Sub(*trip.PickedUpAt)

	travelMin := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "TravelMinute",
			Other: "{{.Min}} min",
		},
		TemplateData: map[string]string{
			"Min": fmt.Sprintf("%.0f", duration.Minutes()),
		},
	})

	elements := []linebot.FlexComponent{
		&linebot.TextComponent{
			Type:   linebot.FlexComponentTypeText,
			Text:   title,
			Weight: linebot.FlexTextWeightTypeBold,
			Size:   linebot.FlexTextSizeTypeLg,
		},
		&linebot.TextComponent{
			Type:   linebot.FlexComponentTypeText,
			Text:   question,
			Wrap:   true,
			Weight: linebot.FlexTextWeightTypeRegular,
			Size:   linebot.FlexTextSizeTypeMd,
		},

		&linebot.BoxComponent{
			Type:    linebot.FlexComponentTypeBox,
			Layout:  linebot.FlexBoxLayoutTypeBaseline,
			Spacing: linebot.FlexComponentSpacingTypeXs,
			Margin:  linebot.FlexComponentMarginTypeXl,
			Contents: []linebot.FlexComponent{
				&linebot.TextComponent{
					Type:   linebot.FlexComponentTypeText,
					Text:   pickup,
					Weight: linebot.FlexTextWeightTypeRegular,
					Flex:   &flexLabel,
					Size:   linebot.FlexTextSizeTypeSm,
				},
				&linebot.TextComponent{
					Type:   linebot.FlexComponentTypeText,
					Text:   trip.From,
					Weight: linebot.FlexTextWeightTypeRegular,
					Flex:   &flexDesc,
					Size:   linebot.FlexTextSizeTypeSm,
					Wrap:   true,
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
					Text:   to,
					Weight: linebot.FlexTextWeightTypeRegular,
					Flex:   &flexLabel,
					Size:   linebot.FlexTextSizeTypeSm,
					Wrap:   true,
				},
				&linebot.TextComponent{
					Type:   linebot.FlexComponentTypeText,
					Text:   trip.To,
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
					Text:   durationLabel,
					Weight: linebot.FlexTextWeightTypeRegular,
					Flex:   &flexLabel,
					Size:   linebot.FlexTextSizeTypeSm,
				},
				&linebot.TextComponent{
					Type:   linebot.FlexComponentTypeText,
					Text:   travelMin,
					Weight: linebot.FlexTextWeightTypeRegular,
					Flex:   &flexDesc,
					Size:   linebot.FlexTextSizeTypeSm,
				},
			},
		},
		&linebot.ButtonComponent{
			Height: linebot.FlexButtonHeightTypeSm,
			Style:  linebot.FlexButtonStyleTypeLink,
			Action: linebot.NewPostbackAction(
				"⭐️",
				fmt.Sprintf("star-feedback:%d:1", tripID), "", ""),
		},
		&linebot.ButtonComponent{
			Height: linebot.FlexButtonHeightTypeSm,
			Style:  linebot.FlexButtonStyleTypeLink,
			Action: linebot.NewPostbackAction(
				"⭐️⭐️",
				fmt.Sprintf("star-feedback:%d:2", tripID), "", ""),
		},
		&linebot.ButtonComponent{
			Height: linebot.FlexButtonHeightTypeSm,
			Style:  linebot.FlexButtonStyleTypeLink,
			Action: linebot.NewPostbackAction(
				"⭐️⭐️⭐️",
				fmt.Sprintf("star-feedback:%d:3", tripID), "", ""),
		},
		&linebot.ButtonComponent{
			Height: linebot.FlexButtonHeightTypeSm,
			Style:  linebot.FlexButtonStyleTypeLink,
			Action: linebot.NewPostbackAction(
				"⭐️⭐️⭐️⭐️",
				fmt.Sprintf("star-feedback:%d:4", tripID), "", ""),
		},
		&linebot.ButtonComponent{
			Height: linebot.FlexButtonHeightTypeSm,
			Style:  linebot.FlexButtonStyleTypeLink,
			Action: linebot.NewPostbackAction(
				"⭐️⭐️⭐️⭐️⭐️",
				fmt.Sprintf("star-feedback:%d:5", tripID), "", ""),
		},
	}

	contents := &linebot.BubbleContainer{
		Type: linebot.FlexContainerTypeBubble,
		Body: &linebot.BoxComponent{
			Type:     linebot.FlexComponentTypeBox,
			Layout:   linebot.FlexBoxLayoutTypeVertical,
			Contents: elements,
		},
		Footer: &linebot.BoxComponent{
			Type:   linebot.FlexComponentTypeBox,
			Layout: linebot.FlexBoxLayoutTypeVertical,
			Contents: []linebot.FlexComponent{
				&linebot.SpacerComponent{
					Type: linebot.FlexComponentTypeSeparator,
					Size: linebot.FlexSpacerSizeTypeSm,
				},
			},
		},
	}

	return linebot.NewFlexMessage("StarFeedback", contents)
}

// EstimatedTravelTimeFlex shows alternative travel time, but continue asking
// 		if customer want to use the service, when?
func (app *HailingApp) EstimatedTravelTimeFlex(record *ReservationRecord, btnAction linebot.TemplateAction, localizer *i18n.Localizer) linebot.SendingMessage {
	// secondaryColor := "#AAAAAA"

	confirm := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "ReservationConfirmation",
			Other: "Your reservation: confirmation",
		},
	})

	elements := []linebot.FlexComponent{
		&linebot.TextComponent{
			Type:   linebot.FlexComponentTypeText,
			Text:   confirm,
			Weight: linebot.FlexTextWeightTypeBold,
			Size:   linebot.FlexTextSizeTypeXl,
			Wrap:   true,
		},
	}
	elements = append(elements, RecordInformationFlexArray(record, localizer)...)
	estTimeElements, err := app.TravelTimeFlexArray(record, localizer)
	if err == nil {
		elements = append(elements, estTimeElements...)
	} else {
		log.Printf("Adding TravelTime failed: %v", err)
	}

	// question
	// question := "If you'd like to continue with our services, when do you want us to pick you up?"
	// elements = append(elements, &linebot.TextComponent{
	// 	Type:  linebot.FlexComponentTypeText,
	// 	Text:  question,
	// 	Size:  linebot.FlexTextSizeTypeMd,
	// 	Wrap:  true,
	// 	Color: secondaryColor,
	// })

	walkInstead := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "WalkInstead",
			Other: "I'll walk instead",
		},
	})

	contents := &linebot.BubbleContainer{
		Type: linebot.FlexContainerTypeBubble,
		Body: &linebot.BoxComponent{
			Type:     linebot.FlexComponentTypeBox,
			Layout:   linebot.FlexBoxLayoutTypeVertical,
			Contents: elements,
		},
		Footer: &linebot.BoxComponent{
			Type:   linebot.FlexComponentTypeBox,
			Layout: linebot.FlexBoxLayoutTypeVertical,
			Contents: []linebot.FlexComponent{
				&linebot.SpacerComponent{
					Type: linebot.FlexComponentTypeSeparator,
					Size: linebot.FlexSpacerSizeTypeSm,
				},
				&linebot.ButtonComponent{
					Height: linebot.FlexButtonHeightTypeMd,
					Style:  linebot.FlexButtonStyleTypePrimary,
					Color:  "#679AF0",
					Action: btnAction,
				},
				&linebot.ButtonComponent{
					Height: linebot.FlexButtonHeightTypeSm,
					Style:  linebot.FlexButtonStyleTypeLink,
					Action: linebot.NewPostbackAction(
						walkInstead,
						"cancel", "cancel", ""),
				},
			},
		},
	}

	return linebot.NewFlexMessage("EstTravelTime", contents)
}

// TravelTimeFlexArray returns estimated travel time in flex component array
func (app *HailingApp) TravelTimeFlexArray(record *ReservationRecord, localizer *i18n.Localizer) ([]linebot.FlexComponent, error) {
	title := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "EstTravelTime",
			Other: "Estimated travel time",
		},
	})
	// NOTE: alternative options..
	walkRoute, err := GetTravelTime("walk", *record)
	if err != nil {
		msg := fmt.Sprintf("err: %v", err)
		return nil, errors.New(msg)
	}
	carSource := "google"
	carRoute, err := GetGoogleTravelTime(*record)
	if err != nil {
		// Still need to report Error
		log.Printf("[TravelTimeFlexArray] Err: %v", err)
		// Try our own GetTravelTime if we have a problem with Google
		carRoute, err = GetTravelTime("car", *record)
		carSource = "osrm"
		if err != nil {
			msg := fmt.Sprintf("err: %v", err)
			return nil, errors.New(msg)
		}
		// msg := fmt.Sprintf("err: %v", err)
		// return nil, errors.New(msg)
	}

	// save polyline from Google's travel time to record
	record.Polyline = carRoute.Geometry
	err = app.SaveRecordToRedis(record)
	if err != nil {
		msg := fmt.Sprintf("err: %v", err)
		return nil, errors.New(msg)
	}

	log.Printf("[EstTravelTimeFlex] Walk: %v", walkRoute)
	log.Printf("[EstTravelTimeFlex] Car: %v", carRoute)

	elements := []linebot.FlexComponent{}
	// title
	elements = append(elements, &linebot.TextComponent{
		Type:   linebot.FlexComponentTypeText,
		Text:   title,
		Weight: linebot.FlexTextWeightTypeRegular,
		Size:   linebot.FlexTextSizeTypeLg,
		Margin: linebot.FlexComponentMarginTypeMd,
	})

	// travel options :: walk
	if walkRoute.Duration > 0 {
		travelLength := localizer.MustLocalize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{
				ID:    "TravelMeter",
				Other: "{{.Meter}} m",
			},
			TemplateData: map[string]string{
				"Meter": fmt.Sprintf("%.0f", walkRoute.Distance),
			},
		})
		travelMin := localizer.MustLocalize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{
				ID:    "TravelMinute",
				Other: "{{.Min}} min",
			},
			TemplateData: map[string]string{
				"Min": fmt.Sprintf("%.0f", walkRoute.Duration/60),
			},
		})
		elements = append(elements, app.travelOption("walk", travelLength, travelMin))
	}
	// travel options :: car
	if carRoute.Duration > 0 && carSource == "google" {
		travelLength := localizer.MustLocalize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{
				ID:    "TravelMeter",
				Other: "{{.Meter}} m",
			},
			TemplateData: map[string]string{
				"Meter": fmt.Sprintf("%.0f", carRoute.Distance),
			},
		})
		if carRoute.DurationInTraffic > carRoute.Duration {
			travelLength = localizer.MustLocalize(&i18n.LocalizeConfig{
				DefaultMessage: &i18n.Message{
					ID:    "TravelMeter",
					Other: "{{.Meter}} m\n{{.FreeFlowMinute}} min w/o traffic",
				},
				TemplateData: map[string]string{
					"Meter":          fmt.Sprintf("%.0f", carRoute.Distance),
					"FreeFlowMinute": fmt.Sprintf("%.0f", carRoute.Duration/60),
				},
			})
		}
		travelMin := localizer.MustLocalize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{
				ID:    "TravelMinute",
				Other: "{{.Min}} min",
			},
			TemplateData: map[string]string{
				"Min": fmt.Sprintf("%.0f", carRoute.DurationInTraffic/60),
			},
		})
		elements = append(elements, app.travelOption("taxi", travelLength, travelMin))
	} else if carRoute.Duration > 0 {
		travelLength := localizer.MustLocalize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{
				ID:    "TravelMeter",
				Other: "{{.Meter}} m",
			},
			TemplateData: map[string]string{
				"Meter": fmt.Sprintf("%.0f", carRoute.Distance),
			},
		})
		travelMin := localizer.MustLocalize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{
				ID:    "TravelMinute",
				Other: "{{.Min}} min",
			},
			TemplateData: map[string]string{
				"Min": fmt.Sprintf("%.0f", carRoute.Duration/60),
			},
		})
		elements = append(elements, app.travelOption("taxi", travelLength, travelMin))
	}
	return elements, nil
}

// RecordInformationFlexArray returns array of record information
func RecordInformationFlexArray(record *ReservationRecord, localizer *i18n.Localizer) []linebot.FlexComponent {
	flexLabel := 2
	flexDesc := 8
	bkk, _ := time.LoadLocation("Asia/Bangkok")
	bkkReservedTime := record.ReservedAt.In(bkk)

	pickup := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "Pickup",
			Other: "Pickup",
		},
	})
	to := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "To",
			Other: "To",
		},
	})
	timeLabel := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "Time",
			Other: "Time",
		},
	})

	flexComponents := []linebot.FlexComponent{
		&linebot.BoxComponent{
			Type:    linebot.FlexComponentTypeBox,
			Layout:  linebot.FlexBoxLayoutTypeBaseline,
			Spacing: linebot.FlexComponentSpacingTypeXs,
			Margin:  linebot.FlexComponentMarginTypeXl,
			Contents: []linebot.FlexComponent{
				&linebot.TextComponent{
					Type:   linebot.FlexComponentTypeText,
					Text:   pickup,
					Weight: linebot.FlexTextWeightTypeRegular,
					Flex:   &flexLabel,
					Size:   linebot.FlexTextSizeTypeSm,
				},
				&linebot.TextComponent{
					Type:   linebot.FlexComponentTypeText,
					Text:   fmt.Sprintf("%v (%d 👤)", record.From, record.NumOfPassengers),
					Weight: linebot.FlexTextWeightTypeRegular,
					Flex:   &flexDesc,
					Size:   linebot.FlexTextSizeTypeSm,
					Wrap:   true,
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
					Text:   to,
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
					Wrap:   true,
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
					Text:   timeLabel,
					Weight: linebot.FlexTextWeightTypeRegular,
					Flex:   &flexLabel,
					Size:   linebot.FlexTextSizeTypeSm,
				},
				&linebot.TextComponent{
					Type:   linebot.FlexComponentTypeText,
					Text:   bkkReservedTime.Format(time.Kitchen),
					Weight: linebot.FlexTextWeightTypeRegular,
					Flex:   &flexDesc,
					Size:   linebot.FlexTextSizeTypeSm,
				},
			},
		},
	}
	return flexComponents
}

// RecordConfirmFlex to return information in form of FLEX
func (record *ReservationRecord) RecordConfirmFlex(title string, localizer *i18n.Localizer, customButtons ...linebot.ButtonComponent) linebot.SendingMessage {
	flexBtnLeft := 3
	flexBtnRight := 6

	pickupTimeChange := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "ChangePickupTime",
			Other: "Change pickup time",
		},
	})
	cancel := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "Cancel",
			Other: "Cancel",
		},
	})

	var successButton linebot.ButtonComponent
	if len(customButtons) == 0 {
		successButton = linebot.ButtonComponent{
			Height: linebot.FlexButtonHeightTypeMd,
			Style:  linebot.FlexButtonStyleTypeSecondary,
			Flex:   &flexBtnRight,
			Action: linebot.NewDatetimePickerAction(
				pickupTimeChange, "datetime-change", "datetime",
				"", "", ""),
		}
	} else {
		successButton = customButtons[0]
	}

	elements := []linebot.FlexComponent{
		&linebot.TextComponent{
			Type:   linebot.FlexComponentTypeText,
			Text:   title,
			Weight: linebot.FlexTextWeightTypeBold,
			Size:   linebot.FlexTextSizeTypeXl,
		},
	}
	elements = append(elements, RecordInformationFlexArray(record, localizer)...)

	contents := &linebot.BubbleContainer{
		Type: linebot.FlexContainerTypeBubble,
		Body: &linebot.BoxComponent{
			Type:     linebot.FlexComponentTypeBox,
			Layout:   linebot.FlexBoxLayoutTypeVertical,
			Contents: elements,
		},
		Footer: &linebot.BoxComponent{
			Type:   linebot.FlexComponentTypeBox,
			Layout: linebot.FlexBoxLayoutTypeVertical,
			Contents: []linebot.FlexComponent{
				// &linebot.ButtonComponent{
				// 	Height: linebot.FlexButtonHeightTypeMd,
				// 	Style:  linebot.FlexButtonStyleTypeLink,
				// 	Action: linebot.NewPostbackAction("Call driver", "call", "", ""),
				// },
				&linebot.BoxComponent{
					Type:   linebot.FlexComponentTypeBox,
					Layout: linebot.FlexBoxLayoutTypeHorizontal,
					Contents: []linebot.FlexComponent{
						&linebot.ButtonComponent{
							Height: linebot.FlexButtonHeightTypeMd,
							Style:  linebot.FlexButtonStyleTypeLink,
							Flex:   &flexBtnLeft,
							Action: linebot.NewPostbackAction(cancel, "cancel", "", ""),
						},
						&successButton,
					},
				},
			},
		},
	}
	return linebot.NewFlexMessage("Record confirmation", contents)
}

// PushNotification handle pushing messages to our user
func (app *HailingApp) PushNotification(lineUserID string, messages ...linebot.SendingMessage) error {
	_, err := app.bot.PushMessage(lineUserID, messages...).Do()
	if err != nil {
		// Do something when some bad happened
		log.Printf("[PushNoti] %v", lineUserID)
	}
	return nil
}

// LanguageOptionFlex push Flex message for language options
func (app *HailingApp) LanguageOptionFlex(localizer *i18n.Localizer) linebot.SendingMessage {
	title := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "LanguagePickerTitle",
			Other: "Language selector",
		},
	})
	question := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "WhichOne",
			Other: "Which one do you prefer?",
		},
	})
	en := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "English",
			Other: "🇺🇸 English",
		},
	})
	ja := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "Japanese",
			Other: "🇯🇵 Japanese",
		},
	})
	th := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "Thai",
			Other: "🇹🇭 Thai",
		},
	})

	elements := []linebot.FlexComponent{
		&linebot.TextComponent{
			Type:   linebot.FlexComponentTypeText,
			Text:   title,
			Weight: linebot.FlexTextWeightTypeBold,
			Size:   linebot.FlexTextSizeTypeLg,
		},
		&linebot.TextComponent{
			Type:   linebot.FlexComponentTypeText,
			Text:   question,
			Wrap:   true,
			Weight: linebot.FlexTextWeightTypeRegular,
			Size:   linebot.FlexTextSizeTypeMd,
		},
		&linebot.ButtonComponent{
			Height: linebot.FlexButtonHeightTypeMd,
			Style:  linebot.FlexButtonStyleTypeLink,
			Action: linebot.NewPostbackAction(
				en,
				fmt.Sprintf("/set:language:en"), "", ""),
		},
		&linebot.ButtonComponent{
			Height: linebot.FlexButtonHeightTypeMd,
			Style:  linebot.FlexButtonStyleTypeLink,
			Action: linebot.NewPostbackAction(
				ja,
				fmt.Sprintf("/set:language:ja"), "", ""),
		},
		&linebot.ButtonComponent{
			Height: linebot.FlexButtonHeightTypeMd,
			Style:  linebot.FlexButtonStyleTypeLink,
			Action: linebot.NewPostbackAction(
				th,
				fmt.Sprintf("/set:language:th"), "", ""),
		},
	}

	contents := &linebot.BubbleContainer{
		Type: linebot.FlexContainerTypeBubble,
		Body: &linebot.BoxComponent{
			Type:     linebot.FlexComponentTypeBox,
			Layout:   linebot.FlexBoxLayoutTypeVertical,
			Contents: elements,
		},
		Footer: &linebot.BoxComponent{
			Type:   linebot.FlexComponentTypeBox,
			Layout: linebot.FlexBoxLayoutTypeVertical,
			Contents: []linebot.FlexComponent{
				&linebot.SpacerComponent{
					Type: linebot.FlexComponentTypeSeparator,
					Size: linebot.FlexSpacerSizeTypeSm,
				},
			},
		},
	}
	altText := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "Confirmation",
			Other: "Language",
		},
	})
	return linebot.NewFlexMessage(altText, contents)
}

// HelpMessageFlex push Flex message for language options
func (app *HailingApp) HelpMessageFlex(localizer *i18n.Localizer) linebot.SendingMessage {
	title := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "Help",
			Other: "Help",
		},
	})
	listOfCommands := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "ListOfAvailableCommands",
			Other: "List of available commands",
		},
	})
	elements := []linebot.FlexComponent{
		&linebot.TextComponent{
			Type:   linebot.FlexComponentTypeText,
			Text:   title,
			Weight: linebot.FlexTextWeightTypeBold,
			Size:   linebot.FlexTextSizeTypeLg,
		},
		&linebot.TextComponent{
			Type:   linebot.FlexComponentTypeText,
			Text:   listOfCommands,
			Wrap:   true,
			Weight: linebot.FlexTextWeightTypeRegular,
			Size:   linebot.FlexTextSizeTypeMd,
		},
		&linebot.TextComponent{
			Type:   linebot.FlexComponentTypeText,
			Text:   "/help - this command",
			Wrap:   true,
			Weight: linebot.FlexTextWeightTypeRegular,
			Size:   linebot.FlexTextSizeTypeSm,
		},
		&linebot.TextComponent{
			Type:   linebot.FlexComponentTypeText,
			Text:   "/lang - call language UI picker",
			Wrap:   true,
			Weight: linebot.FlexTextWeightTypeRegular,
			Size:   linebot.FlexTextSizeTypeSm,
		},
	}

	contents := &linebot.BubbleContainer{
		Type: linebot.FlexContainerTypeBubble,
		Body: &linebot.BoxComponent{
			Type:     linebot.FlexComponentTypeBox,
			Layout:   linebot.FlexBoxLayoutTypeVertical,
			Contents: elements,
		},
		Footer: &linebot.BoxComponent{
			Type:   linebot.FlexComponentTypeBox,
			Layout: linebot.FlexBoxLayoutTypeVertical,
			Contents: []linebot.FlexComponent{
				&linebot.SpacerComponent{
					Type: linebot.FlexComponentTypeSeparator,
					Size: linebot.FlexSpacerSizeTypeSm,
				},
			},
		},
	}
	altText := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "Help",
			Other: "Help",
		},
	})
	return linebot.NewFlexMessage(altText, contents)
}
