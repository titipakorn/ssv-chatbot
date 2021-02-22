package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/line/line-bot-sdk-go/linebot"
)

// BotCommandHandler handle all commands from LIFF
func (app *HailingApp) BotCommandHandler(replyToken string, lineUserID string, reply Reply) error {

	text := strings.TrimSpace(reply.Text)
	cmds := strings.Fields(text)
	if len(cmds) == 0 {
		return nil
	}
	cmd1 := strings.TrimPrefix(cmds[0], "/")
	// https://golang.org/pkg/regexp/syntax/
	// r3 := regexp.MustCompile(`/(?P<cmd>\w+)\s+?(?P<subcmd>\w+)\s+?(?P<val>\w+)`)
	// r2 := regexp.MustCompile(`/(?P<cmd>.+)\s+(?P<subcmd>\w+?)`)
	// text := strings.Replace(reply.Text, "[LIFF]", "", 1)
	// text = strings.TrimSpace(text)
	// text = strings.ToLower(text)

	switch cmd1 {
	case "language":
	case "lang":
		// TODO: return language options as picker
		return app.CancelHandler(replyToken, lineUserID)
	case "help":
		return app.HelpHandler(replyToken, lineUserID)
	}
	msg := fmt.Sprintf("Command unavailable")
	if _, err := app.bot.ReplyMessage(
		replyToken,
		linebot.NewTextMessage(msg),
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
	user, err := app.FindOrCreateUser(lineUserID)
	if err != nil {
		errMsg := fmt.Sprintf("%v", err)
		if _, err := app.bot.ReplyMessage(
			replyToken,
			linebot.NewTextMessage(errMsg),
		).Do(); err != nil {
			return err
		}
	}
	if user.Language == lang {
		errMsg := fmt.Sprintf("Your language is already %s", lang)
		if _, err := app.bot.ReplyMessage(
			replyToken,
			linebot.NewTextMessage(errMsg),
		).Do(); err != nil {
			return err
		}
	}
	// this is supposed to be database-related command
	_, err = app.SetLanguage(user.ID, lang)
	msg := fmt.Sprintf("Your language set to %v", lang)
	if err != nil {
		msg = fmt.Sprintf("%v", err)
	}
	if _, err := app.bot.ReplyMessage(
		replyToken,
		linebot.NewTextMessage(msg),
	).Do(); err != nil {
		return err
	}
	return nil
}

// CancelHandler takes care of the reservation cancellation
func (app *HailingApp) CancelHandler(replyToken string, lineUserID string) error {
	total, err := app.Cancel(lineUserID)
	if err != nil {
		errMsg := fmt.Sprintf("%v", err)
		if _, err := app.bot.ReplyMessage(
			replyToken,
			linebot.NewTextMessage(errMsg),
		).Do(); err != nil {
			return err
		}
	}
	if total > 0 {
		msg := fmt.Sprintf("Your reservation cancelled.")
		if _, err := app.bot.ReplyMessage(
			replyToken,
			linebot.NewTextMessage(msg),
		).Do(); err != nil {
			return err
		}
	}
	return nil

}

// FeedbackHandler takes care of the reservation feedback
func (app *HailingApp) FeedbackHandler(replyToken string, tripID string, rating string) error {
	nRating, _ := strconv.Atoi(rating)
	tID, _ := strconv.Atoi(tripID)
	_, err := app.SaveTripFeedback(tID, nRating)
	if err != nil {
		return err
	}
	msg := fmt.Sprintf("Thank you for your feedback. We hope to see you again.")
	if _, err := app.bot.ReplyMessage(
		replyToken,
		linebot.NewTextMessage(msg),
	).Do(); err != nil {
		return err
	}
	return nil
}
