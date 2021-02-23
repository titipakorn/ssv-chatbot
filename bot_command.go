package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/line/line-bot-sdk-go/linebot"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

// BotCommandHandler handle all commands from LIFF
func (app *HailingApp) BotCommandHandler(replyToken string, lineUserID string, reply Reply) error {

	text := strings.TrimSpace(reply.Text)
	cmds := strings.Fields(text)
	if len(cmds) == 0 {
		return nil
	}
	_, localizer, err := app.Localizer(lineUserID)
	if err != nil {
		return app.replyText(replyToken, err.Error())
	}
	cmd1 := strings.TrimPrefix(cmds[0], "/")
	// https://golang.org/pkg/regexp/syntax/
	// r3 := regexp.MustCompile(`/(?P<cmd>\w+)\s+?(?P<subcmd>\w+)\s+?(?P<val>\w+)`)
	// r2 := regexp.MustCompile(`/(?P<cmd>.+)\s+(?P<subcmd>\w+?)`)
	// text := strings.Replace(reply.Text, "[LIFF]", "", 1)
	// text = strings.TrimSpace(text)
	// text = strings.ToLower(text)
	msgs := []linebot.SendingMessage{}

	switch cmd1 {
	case "get":
		// TODO: this is a different beast which will need to verify subcommand
		// 		 for other steps
		break
	case "set":
		// TODO: this is a different beast which will need to verify subcommand
		// 		 for other steps
		break
	case "language":
	case "lang":
		langFlex := app.LanguageOptionFlex()
		msgs = append(msgs, langFlex)
	case "help":
		helpFlex := app.HelpMessageFlex()
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
			ID:    "LanguageHandlerSame",
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
			ID:    "LanguageHandlerSet",
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
	total, err := app.Cancel(lineUserID)
	if err != nil {
		errMsg := fmt.Sprintf("%v", err)
		return app.replyText(replyToken, errMsg)
	}
	_, localizer, err := app.Localizer(lineUserID)
	cancelText := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "CancelHandlerMessage",
			Other: "Your reservation cancelled.",
		},
	})
	if err != nil {
		return app.replyText(replyToken, err.Error())
	}
	if total > 0 {
		return app.replyText(replyToken, cancelText)
	}
	return nil

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
	feedbackText := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "FeedbackHandlerMessage",
			Other: "Thank you for your feedback. We hope to see you again.",
		},
	})
	return app.replyText(replyToken, feedbackText)
}
