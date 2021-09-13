package main

import (
	"fmt"
	"log"

	"github.com/line/line-bot-sdk-go/linebot"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

func (app *HailingApp) CompleteRegistration(replyToken string, user *User, reply Reply) error {
	/* Step
	- both FirstName & LastName are blank
		> Ask FirstName
	- step: "FirstName" wait for reply
		> Ask LastName next
	- final - asking if user wanna init reservation process
	*/
	_, localizer, err := app.Localizer(user.LineUserID)
	if err != nil {
		return err
	}
	if reply.Text == "" || (user.FirstName == "" && user.LastName == "") {
		return app.initRegistrationProcess(replyToken, user, localizer)
	}
	rec, _ := app.FindRecord(user.LineUserID)
	if rec.Waiting == "first_name" {
		rec.State = "first_name"
		rec.Waiting = "last_name"
		app.SaveRecordToRedis(rec)
		q := fmt.Sprintf(`"first_name" = '%s'`, reply.Text)
		_, err := app.UpdateUserInfo(user.ID, q)
		if err != nil {
			return err
		}
		msg := localizer.MustLocalize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{
				ID:    "WhatIsYourLastName",
				Other: "What's your last name?",
			},
		})
		return app.replyText(replyToken, msg)
	}
	if rec.Waiting == "last_name" {
		q := fmt.Sprintf(`"last_name" = '%s'`, reply.Text)
		_, err := app.UpdateUserInfo(user.ID, q)
		if err != nil {
			return err
		}
		// Done with registration then asking to init
		return app.registratonDone(replyToken, localizer)
	}
	return nil
}

func (app *HailingApp) registratonDone(replyToken string, localizer *i18n.Localizer) error {
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

	done := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "RegistrationComplete",
			Other: "Your account is ready",
		},
	})
	if _, err := app.bot.ReplyMessage(
		replyToken,
		linebot.NewTextMessage(done),
		ConfirmDialog(initLine, yes, "init"),
	).Do(); err != nil {
		return err
	}
	return nil
}

func (app *HailingApp) initRegistrationProcess(replyToken string, user *User, localizer *i18n.Localizer) error {
	msg1 := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "AskToCompleteRegistration",
			Other: "Hello there, we still need some information to serve you better.",
		},
	})
	msg2 := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "WhatIsYourFirstName",
			Other: "What's your first name?",
		},
	})
	replies := []string{msg1, msg2}
	rec := ReservationRecord{
		UserID:     user.ID,
		LineUserID: user.LineUserID,
		State:      "register",
		Waiting:    "first_name",
		TripID:     -1,
	}
	err := app.SaveRecordToRedis(&rec)
	if err != nil {
		log.Printf("SAVE to Redis FAILED: %v", err)
		return err
	}
	return app.replyText(replyToken, replies...)
}
