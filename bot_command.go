package main

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/line/line-bot-sdk-go/linebot"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

// BotCommandHandler handle all commands from LIFF
func (app *HailingApp) BotCommandHandler(replyToken string, lineUserID string, reply Reply) error {

	text := strings.TrimSpace(reply.Text)
	cmds := strings.Split(text, ":")
	if len(cmds) == 0 {
		return nil
	}
	_, localizer, err := app.Localizer(lineUserID)
	if err != nil {
		return app.replyText(replyToken, err.Error())
	}
	cmd1 := strings.TrimPrefix(cmds[0], "/")
	cmd1 = strings.ToLower(cmd1)
	msgs := []linebot.SendingMessage{}
	log.Printf("[BotCmd] %v", cmds)

	switch cmd1 {
	case "get":
		// TODO: this is a different beast which will need to verify subcommand
		// 		 for other steps
		break
	case "set":
		if len(cmds) < 3 {
			return errors.New("missing arguments")
		}
		if cmds[1] == "language" {
			return app.LanguageHandler(replyToken, lineUserID, cmds[2])
		} else if cmds[1] == "cancel-reason" {
			// TODO: [cancel] Store reason to trip record
			tripID := cmds[2]
			cancelReason := cmds[3]
			cancelID := cmds[4]
			log.Printf("Trip #%s cancelled because of %s", tripID, cancelReason)
			_, err := app.UpdateCancellationReason(tripID, cancelReason,cancelID)
			if err != nil {
				return errors.New("Updating cancellation reason failed")
			}
			return app.EndOfCancellation(replyToken, localizer)
		}
	case "language":
	case "lang":
		langFlex := app.LanguageOptionFlex(localizer)
		msgs = append(msgs, langFlex)
	case "help":
		helpFlex := app.HelpMessageFlex(localizer)
		msgs = append(msgs, helpFlex)
	}
	if len(msgs) == 0 {
		// if no other command, then return this
		msgCommandUnavailable := localizer.MustLocalize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{
				ID:    "CommandUnavailable",
				Other: "Command unavailable",
			},
		})
		txtMsg := linebot.NewTextMessage(msgCommandUnavailable)
		msgs = append(msgs, txtMsg)
	}
	if _, err := app.bot.ReplyMessage(
		replyToken,
		msgs...,
	).Do(); err != nil {
		return err
	}
	return nil
}

// HelpHandler shows the available command
func (app *HailingApp) HelpHandler(replyToken string, lineUserID string) error {
	// TODO: implement this
	return nil
}

// LanguageHandler shows the available command
func (app *HailingApp) LanguageHandler(replyToken string, lineUserID string, lang string) error {
	user, localizer, err := app.Localizer(lineUserID)
	if err != nil {
		return app.replyText(err.Error())
	}
	msgLanguageTheSame := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "LanguageIsTheSame",
			Other: "Your language is already {{.Lang}}.",
		},
		TemplateData: map[string]string{
			"Lang": lang,
		},
	})

	if user.Language == lang {
		if _, err := app.bot.ReplyMessage(
			replyToken,
			linebot.NewTextMessage(msgLanguageTheSame),
		).Do(); err != nil {
			return err
		}
	}
	msgLanguageSet := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "LanguageSetTo",
			Other: "Your language set to {{.Lang}}.",
		},
		TemplateData: map[string]string{
			"Lang": lang,
		},
	})
	// this is supposed to be database-related command
	_, err = app.SetLanguage(user.ID, lang)
	msg := msgLanguageSet
	if err != nil {
		msg = fmt.Sprintf("%v", err)
	}
	return app.replyText(replyToken, msg)
}

// CancelHandler takes care of the reservation cancellation
func (app *HailingApp) CancelHandler(replyToken string, lineUserID string) error {
	tripID, err := app.Cancel(lineUserID)
	if err != nil {
		errMsg := fmt.Sprintf("%v", err)
		return app.replyText(replyToken, errMsg)
	}
	_, localizer, err := app.Localizer(lineUserID)
	if err != nil {
		return app.replyText(replyToken, err.Error())
	}
	// NOTE: Should a cancellation before "confirm" be logged?
	if tripID > 0 {
		msg := app.CancellationFeedback(localizer, tripID)
		return app.replyMessage(replyToken, msg)
		// return app.replyText(replyToken, cancelText)
	}
	return nil

}

func (app *HailingApp) EndOfCancellation(replyToken string, localizer *i18n.Localizer) error {
	cancelText := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "ReservationCancelled",
			Other: "Your reservation cancelled.",
		},
	})
	return app.replyText(replyToken, cancelText)
}

// FeedbackHandler takes care of the reservation feedback
func (app *HailingApp) FeedbackHandler(replyToken string, lineUserID string, tripID string, rating string) error {
	nRating, _ := strconv.Atoi(rating)
	tID, _ := strconv.Atoi(tripID)
	_, err := app.SaveTripFeedback(tID, nRating)
	if err != nil {
		return err
	}
	_, localizer, err := app.Localizer(lineUserID)
	if err != nil {
		return err
	}
	feedbackText := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "ThankYouSeeYouAgain",
			Other: "Thank you for your feedback. We hope to see you again.",
		},
	})
	return app.replyText(replyToken, feedbackText)
}

func Find(a []Questionaire, x int) int {
    for i, n := range a {
        if x == n.ID {
            return i
        }
    }
    return len(a)
}


// QuestionaireHandler takes care of the questionaire feedback
func (app *HailingApp) AskQuestionaire(replyToken string, record *ReservationRecord) error {
	user, _, err := app.Localizer(record.LineUserID)
	if err != nil {
		return err
	}
	if _, err := app.bot.ReplyMessage(
		replyToken,
		app.QuestionFlex(record.QList[record.QState],record.TripID,user.Language),
	).Do(); err != nil {
		return err
	}
	return nil
}



// QuestionaireHandler takes care of the questionaire feedback
func (app *HailingApp) QuestionaireHandler(replyToken string, lineUserID string, tripID string, questionID string ,rating string) error {
	var record *ReservationRecord
	nRating, _ := strconv.Atoi(rating)
	qID, _ := strconv.Atoi(questionID)
	tID, _ := strconv.Atoi(tripID)
	_, err := app.SaveTripQuestionaire(tID, qID, nRating)
	if err != nil {
		return err
	}
	_, localizer, err := app.Localizer(lineUserID)
	if err != nil {
		return err
	}

	record, err = app.FindRecord(lineUserID)
	if err != nil {
		return err
	}
	nextQuestion := record.QState + 1
	// nextQuestion := Find(record.QList, record.QState) + 1
	feedbackText := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "ThankYouSeeYouAgain",
			Other: "Thank you for your feedback. We hope to see you again.",
		},
	})
	if(nextQuestion>=len(record.QList)){
		app.Cleanup(lineUserID)
	}else{
		record.QState = nextQuestion
		// record.QState = record.QList[nextQuestion]
		err = app.SaveRecordToRedis(record)
		if err != nil {
			return err
		}
		return app.AskQuestionaire(replyToken, record)
	}
	return app.replyText(replyToken, feedbackText)
}

