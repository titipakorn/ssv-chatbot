package main

import (
	"strings"
)

/* Available commands
1. [LIFF] Cancel
2. [LIFF] feedback on trip <int> => <rating>
	this is user's feedback
*/

// LIFFHandler handle all commands from LIFF
func (app *HailingApp) LIFFHandler(replyToken string, lineUserID string, reply Reply) error {
	text := strings.Replace(reply.Text, "[LIFF]", "", 1)
	text = strings.TrimSpace(text)
	text = strings.ToLower(text)
	cmds := strings.Split(text, " ")

	if len(cmds) == 0 {
		return nil
	}

	switch cmds[0] {
	case "cancel":
		return app.CancelHandler(replyToken, lineUserID)
	case "feedback":
		// [0: "feedback" 1:"on" 2:"trip" 3:"<int>" 4:"=>" 5:"<rating>"]
		if len(cmds) != 6 {
			return nil
		}
		tripID := cmds[3]
		rating := cmds[5]
		return app.FeedbackHandler(replyToken, tripID, rating)
	}
	return nil
}
