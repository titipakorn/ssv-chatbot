package main

import (
	"fmt"
	"log"
	"net/mail"
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
	rec, _ := app.FindRecord(user.LineUserID)
	if rec == nil {
		rec = &ReservationRecord{}
	}
	log.Printf("[register] rec#1: %v / %v", rec, reply.Text)
	if rec.Title != "register" || reply.Text == "" {
		log.Printf("[register] rec step 1: %v", rec)
		return app.initRegistrationProcess(replyToken, user, localizer)
	}
	// if rec.Waiting == "first_name" {
	// 	log.Printf("[register] rec step 2: %v", reply.Text)
	// 	rec.State = "first_name"
	// 	rec.Waiting = "last_name"
	// 	app.SaveRecordToRedis(rec)
	// 	q := fmt.Sprintf(`"first_name" = '%s'`, reply.Text)
	// 	_, err := app.UpdateUserInfo(user.ID, q)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	msg := localizer.MustLocalize(&i18n.LocalizeConfig{
	// 		DefaultMessage: &i18n.Message{
	// 			ID:    "WhatIsYourLastName",
	// 			Other: "What's your last name?",
	// 		},
	// 	})
	// 	return app.replyText(replyToken, msg)
	// }
	// if rec.Waiting == "last_name" {
	// 	log.Printf("[register] rec step 3: %v", reply.Text)
	// 	q := fmt.Sprintf(`"last_name" = '%s'`, reply.Text)
	// 	_, err := app.UpdateUserInfo(user.ID, q)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	// Done with registration then asking to init
	// 	app.Cancel(user.LineUserID)
	// 	msg := localizer.MustLocalize(&i18n.LocalizeConfig{
	// 		DefaultMessage: &i18n.Message{
	// 			ID:    "WhatIsYourEmail",
	// 			Other: "What's email?",
	// 		},
	// 	})
	// 	return app.replyText(replyToken, msg)
	// }
	if rec.Waiting == "user_type" {
		log.Printf("[register] rec step 2: %v", reply.Text)
		checked_error := false
		switch reply.Text {
			case
				"Professor",
				"Staff of University",
				"Student",
				"Public General":
				checked_error = false
			default:
				checked_error = true
		}
		
		if checked_error {
			return app.replyBack(replyToken, Question{
				Text: localizer.MustLocalize(&i18n.LocalizeConfig{
					DefaultMessage: &i18n.Message{
						ID:    "WhatIsYourUserType",
						Other: "Who are you?",
					},
				}),
				Buttons: []QuickReplyButton{
					{
						Label: localizer.MustLocalize(&i18n.LocalizeConfig{
							DefaultMessage: &i18n.Message{
								ID:    "A_Professor",
								Other: "Professor",
							},
						}),
						Text:  "Professor",
					},
					{
						Label:localizer.MustLocalize(&i18n.LocalizeConfig{
							DefaultMessage: &i18n.Message{
								ID:    "A_UniversityStaff",
								Other: "Staff of University",
							},
						}),
						Text:  "Staff of University",
					},
					{
						Label:localizer.MustLocalize(&i18n.LocalizeConfig{
							DefaultMessage: &i18n.Message{
								ID:    "A_Student",
								Other: "Student",
							},
						}),
						Text:  "Student",
					},
					{
						Label:localizer.MustLocalize(&i18n.LocalizeConfig{
							DefaultMessage: &i18n.Message{
								ID:    "A_PublicGeneral",
								Other: "Public General",
							},
						}),
						Text:  "Public General",
					},
				},
			})
		}
		rec.State = "user_type"
		rec.Waiting = "gender"
		app.SaveRecordToRedis(rec)
		q := fmt.Sprintf(`"user_type" = '%s'`, reply.Text)
		_, err := app.UpdateUserInfo(user.ID, q)
		if err != nil {
			return err
		}
		msg := Question{
			Text: localizer.MustLocalize(&i18n.LocalizeConfig{
				DefaultMessage: &i18n.Message{
					ID:    "WhatIsYourgender",
					Other: "What's your gender",
				},
			}),
			Buttons: []QuickReplyButton{
				{
					Label: localizer.MustLocalize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "A_Male",
							Other: "Male",
						},
					}),
					Text:  "Male",
				},
				{
					Label: localizer.MustLocalize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "A_Female",
							Other: "Female",
						},
					}),
					Text:  "Female",
				},
				{
					Label:localizer.MustLocalize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "A_NotSpecified",
							Other: "Not Specified",
						},
					}),
					Text:  "Not specified",
				},
			},
		}
		return app.replyBack(replyToken, msg)
	}
	if rec.Waiting == "gender" {
		checked_error := false
		switch reply.Text {
			case
				"Male",
				"Female",
				"Not specified":
				checked_error = false
			default:
				checked_error = true
		}
		
		if checked_error {
			return app.replyBack(replyToken, Question{
				Text: localizer.MustLocalize(&i18n.LocalizeConfig{
					DefaultMessage: &i18n.Message{
						ID:    "WhatIsYourgender",
						Other: "What's your gender",
					},
				}),
				Buttons: []QuickReplyButton{
					{
						Label: localizer.MustLocalize(&i18n.LocalizeConfig{
							DefaultMessage: &i18n.Message{
								ID:    "A_Male",
								Other: "Male",
							},
						}),
						Text:  "Male",
					},
					{
						Label: localizer.MustLocalize(&i18n.LocalizeConfig{
							DefaultMessage: &i18n.Message{
								ID:    "A_Female",
								Other: "Female",
							},
						}),
						Text:  "Female",
					},
					{
						Label:localizer.MustLocalize(&i18n.LocalizeConfig{
							DefaultMessage: &i18n.Message{
								ID:    "A_NotSpecified",
								Other: "Not Specified",
							},
						}),
						Text:  "Not specified",
					},
				},
			})
		}
		rec.State = "gender"
		rec.Waiting = "age"
		app.SaveRecordToRedis(rec)
		log.Printf("[register] rec step 3: %v", reply.Text)
		q := fmt.Sprintf(`"gender" = '%s'`, reply.Text)
		_, err := app.UpdateUserInfo(user.ID, q)
		if err != nil {
			return err
		}
		// Done with registration then asking to init
		// app.Cancel(user.LineUserID)
		msg := Question{
			Text: localizer.MustLocalize(&i18n.LocalizeConfig{
				DefaultMessage: &i18n.Message{
					ID:    "WhatIsYourAge",
					Other: "What's your age range?",
				},
			}),
			Buttons: []QuickReplyButton{
				{
					Label: "1-19",
					Text:  "1-19",
				},
				{
					Label: "20-24",
					Text:  "20-24",
				},
				{
					Label: "25-29",
					Text:  "25-29",
				},
				{
					Label: "30-34",
					Text:  "30-34",
				},
				{
					Label: "35-39",
					Text:  "35-39",
				},
				{
					Label: "40-44",
					Text:  "40-44",
				},
				{
					Label: "45-49",
					Text:  "45-49",
				},
				{
					Label: "50-54",
					Text:  "50-54",
				},
				{
					Label: "55-59",
					Text:  "55-59",
				},
				{
					Label: "60-64",
					Text:  "60-64",
				},
				{
					Label:localizer.MustLocalize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "A_65Up",
							Other: "65 Up",
						},
					}),
					Text:  "65 Up",
				},
			},
		}
		return app.replyBack(replyToken, msg)
	}
	if rec.Waiting == "age" {
		checked_error := false
		switch reply.Text {
			case
				"1-19",
				"20-24",
				"25-29",
				"30-34",
				"35-39",
				"40-44",
				"45-49",
				"50-54",
				"55-59",
				"60-64",
				"65 Up":
				checked_error = false
			default:
				checked_error = true
		}
		
		if checked_error {
			return app.replyBack(replyToken, Question{
				Text: localizer.MustLocalize(&i18n.LocalizeConfig{
					DefaultMessage: &i18n.Message{
						ID:    "WhatIsYourAge",
						Other: "What's your age range?",
					},
				}),
				Buttons: []QuickReplyButton{
					{
						Label: "1-19",
						Text:  "1-19",
					},
					{
						Label: "20-24",
						Text:  "20-24",
					},
					{
						Label: "25-29",
						Text:  "25-29",
					},
					{
						Label: "30-34",
						Text:  "30-34",
					},
					{
						Label: "35-39",
						Text:  "35-39",
					},
					{
						Label: "40-44",
						Text:  "40-44",
					},
					{
						Label: "45-49",
						Text:  "45-49",
					},
					{
						Label: "50-54",
						Text:  "50-54",
					},
					{
						Label: "55-59",
						Text:  "55-59",
					},
					{
						Label: "60-64",
						Text:  "60-64",
					},
					{
						Label:localizer.MustLocalize(&i18n.LocalizeConfig{
							DefaultMessage: &i18n.Message{
								ID:    "A_65Up",
								Other: "65 Up",
							},
						}),
						Text:  "65 Up",
					},
				},
			})
		}
		log.Printf("[register] rec step 4: %v", reply.Text)
		rec.State = "age"
		rec.Waiting = "primary_mode"
		app.SaveRecordToRedis(rec)
		q := fmt.Sprintf(`"age" = '%s'`, reply.Text)
		_, err := app.UpdateUserInfo(user.ID, q)
		if err != nil {
			return err
		}
		// Done with registration then asking to init
		// app.Cancel(user.LineUserID)
		msg := Question{
			Text: localizer.MustLocalize(&i18n.LocalizeConfig{
				DefaultMessage: &i18n.Message{
					ID:    "WhatIsYourPrimaryMode",
					Other: "What's your primary transporation mode?",
				},
			}),
			Buttons: []QuickReplyButton{
				{
					Label: localizer.MustLocalize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "A_Walk",
							Other: "Walk",
						},
					}),
					Text:  "Walk",
				},
				{
					Label:localizer.MustLocalize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "A_EvBus",
							Other: "EV Bus",
						},
					}),
					Text:  "EV Bus",
				},
				{
					Label:localizer.MustLocalize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "A_Bicycle",
							Other: "Bicycle",
						},
					}),
					Text:  "Bicycle",
				},
				{
					Label:localizer.MustLocalize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "A_Scooter",
							Other: "Scooter",
						},
					}),
					Text:  "Scooter",
				},
				{
					Label:localizer.MustLocalize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "A_Taxi",
							Other: "Taxi(Including Grab)",
						},
					}),
					Text:  "Taxi",
				},
				{
					Label:localizer.MustLocalize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "A_PrivateCar",
							Other: "Private Car",
						},
					}),
					Text:  "Private Car",
				},
				{
					Label:localizer.MustLocalize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "A_Motorbike",
							Other: "Motorbike",
						},
					}),
					Text:  "Motorbike",
				},
				{
					Label:localizer.MustLocalize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "A_ShuttleBus",
							Other: "Shuttle Bus",
						},
					}),
					Text:  "Shuttle Bus",
				},
				{
					Label:localizer.MustLocalize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "A_MotorbikeTaxi",
							Other: "Motorbike Taxi",
						},
					}),
					Text:  "Motorbike Taxi",
				},
				{
					Label:localizer.MustLocalize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "A_Songthaew",
							Other: "Songthaew",
						},
					}),
					Text:  "Songthaew",
				},
			},
		}
		return app.replyBack(replyToken, msg)
	} 
	if rec.Waiting == "primary_mode" {
		checked_error := false
		switch reply.Text {
			case
				"Walk",
				"EV Bus",
				"Bicycle",
				"Scooter",
				"Taxi",
				"Private Car",
				"Motorbike",
				"Shuttle Bus",
				"Motorbike Taxi",
				"Songthaew":
				checked_error = false
			default:
				checked_error = true
		}
		
		if checked_error {
			return app.replyBack(replyToken, Question{
				Text: localizer.MustLocalize(&i18n.LocalizeConfig{
					DefaultMessage: &i18n.Message{
						ID:    "WhatIsYourPrimaryMode",
						Other: "What's your primary transporation mode?",
					},
				}),
				Buttons: []QuickReplyButton{
					{
						Label: localizer.MustLocalize(&i18n.LocalizeConfig{
							DefaultMessage: &i18n.Message{
								ID:    "A_Walk",
								Other: "Walk",
							},
						}),
						Text:  "Walk",
					},
					{
						Label:localizer.MustLocalize(&i18n.LocalizeConfig{
							DefaultMessage: &i18n.Message{
								ID:    "A_EvBus",
								Other: "EV Bus",
							},
						}),
						Text:  "EV Bus",
					},
					{
						Label:localizer.MustLocalize(&i18n.LocalizeConfig{
							DefaultMessage: &i18n.Message{
								ID:    "A_Bicycle",
								Other: "Bicycle",
							},
						}),
						Text:  "Bicycle",
					},
					{
						Label:localizer.MustLocalize(&i18n.LocalizeConfig{
							DefaultMessage: &i18n.Message{
								ID:    "A_Scooter",
								Other: "Scooter",
							},
						}),
						Text:  "Scooter",
					},
					{
						Label:localizer.MustLocalize(&i18n.LocalizeConfig{
							DefaultMessage: &i18n.Message{
								ID:    "A_Taxi",
								Other: "Taxi(Including Grab)",
							},
						}),
						Text:  "Taxi",
					},
					{
						Label:localizer.MustLocalize(&i18n.LocalizeConfig{
							DefaultMessage: &i18n.Message{
								ID:    "A_PrivateCar",
								Other: "Private Car",
							},
						}),
						Text:  "Private Car",
					},
					{
						Label:localizer.MustLocalize(&i18n.LocalizeConfig{
							DefaultMessage: &i18n.Message{
								ID:    "A_Motorbike",
								Other: "Motorbike",
							},
						}),
						Text:  "Motorbike",
					},
					{
						Label:localizer.MustLocalize(&i18n.LocalizeConfig{
							DefaultMessage: &i18n.Message{
								ID:    "A_ShuttleBus",
								Other: "Shuttle Bus",
							},
						}),
						Text:  "Shuttle Bus",
					},
					{
						Label:localizer.MustLocalize(&i18n.LocalizeConfig{
							DefaultMessage: &i18n.Message{
								ID:    "A_MotorbikeTaxi",
								Other: "Motorbike Taxi",
							},
						}),
						Text:  "Motorbike Taxi",
					},
					{
						Label:localizer.MustLocalize(&i18n.LocalizeConfig{
							DefaultMessage: &i18n.Message{
								ID:    "A_Songthaew",
								Other: "Songthaew",
							},
						}),
						Text:  "Songthaew",
					},
				},
			})
		}
		log.Printf("[register] rec step 5: %v", reply.Text)
		rec.State = "primary_mode"
		rec.Waiting = "first_impression"
		app.SaveRecordToRedis(rec)
		q := fmt.Sprintf(`"primary_mode" = '%s'`, reply.Text)
		_, err := app.UpdateUserInfo(user.ID, q)
		if err != nil {
			return err
		}
		// Done with registration then asking to init
		// app.Cancel(user.LineUserID)
		msg := Question{
			Text: localizer.MustLocalize(&i18n.LocalizeConfig{
				DefaultMessage: &i18n.Message{
					ID:    "HowDidYouFeel",
					Other: "Please tell us your first impression for choosing SSVS",
				},
			}),
			Buttons: []QuickReplyButton{
				{
					Label: localizer.MustLocalize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "A_Innovative",
							Other: "Innovative",
						},
					}),
					Text:  "Innovative",
				},
				{
					Label:localizer.MustLocalize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "A_Comfortable",
							Other: "Comfortable",
						},
					}),
					Text:  "Comfortable",
				},
				{
					Label:localizer.MustLocalize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "A_Ecofriendly",
							Other: "Eco-friendly",
						},
					}),
					Text:  "Eco-friendly",
				},
				{
					Label:localizer.MustLocalize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "A_Convenient",
							Other: "Convenient",
						},
					}),
					Text:  "Convenient",
				},
				{
					Label:localizer.MustLocalize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "A_SafeAndTrustworthy",
							Other: "Safe & Trustworthy",
						},
					}),
					Text:  "Safe and Trustworthy",
				},
				{
					Label:localizer.MustLocalize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "A_FastTravelSpeed",
							Other: "Fast Travel Speed",
						},
					}),
					Text:  "Fast Travel Speed",
				},
				{
					Label:localizer.MustLocalize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "A_LessWaitingTime",
							Other: "Less Waiting Time",
						},
					}),
					Text:  "Less Waiting Time",
				},
			},
		}
		return app.replyBack(replyToken, msg)
	}
	if rec.Waiting == "first_impression" {
		checked_error := false
		switch reply.Text {
			case
				"Innovative",
				"Comfortable",
				"Eco-friendly",
				"Convenient",
				"Safe and Trustworthy",
				"Fast Travel Speed",
				"Less Waiting Time":
				checked_error = false
			default:
				checked_error = true
		}
		
		if checked_error {
			return app.replyBack(replyToken, Question{
				Text: localizer.MustLocalize(&i18n.LocalizeConfig{
					DefaultMessage: &i18n.Message{
						ID:    "HowDidYouFeel",
						Other: "Please tell us your first impression for choosing SSVS",
					},
				}),
				Buttons: []QuickReplyButton{
					{
						Label: localizer.MustLocalize(&i18n.LocalizeConfig{
							DefaultMessage: &i18n.Message{
								ID:    "A_Innovative",
								Other: "Innovative",
							},
						}),
						Text:  "Innovative",
					},
					{
						Label:localizer.MustLocalize(&i18n.LocalizeConfig{
							DefaultMessage: &i18n.Message{
								ID:    "A_Comfortable",
								Other: "Comfortable",
							},
						}),
						Text:  "Comfortable",
					},
					{
						Label:localizer.MustLocalize(&i18n.LocalizeConfig{
							DefaultMessage: &i18n.Message{
								ID:    "A_Ecofriendly",
								Other: "Eco-friendly",
							},
						}),
						Text:  "Eco-friendly",
					},
					{
						Label:localizer.MustLocalize(&i18n.LocalizeConfig{
							DefaultMessage: &i18n.Message{
								ID:    "A_Convenient",
								Other: "Convenient",
							},
						}),
						Text:  "Convenient",
					},
					{
						Label:localizer.MustLocalize(&i18n.LocalizeConfig{
							DefaultMessage: &i18n.Message{
								ID:    "A_SafeAndTrustworthy",
								Other: "Safe & Trustworthy",
							},
						}),
						Text:  "Safe and Trustworthy",
					},
					{
						Label:localizer.MustLocalize(&i18n.LocalizeConfig{
							DefaultMessage: &i18n.Message{
								ID:    "A_FastTravelSpeed",
								Other: "Fast Travel Speed",
							},
						}),
						Text:  "Fast Travel Speed",
					},
					{
						Label:localizer.MustLocalize(&i18n.LocalizeConfig{
							DefaultMessage: &i18n.Message{
								ID:    "A_LessWaitingTime",
								Other: "Less Waiting Time",
							},
						}),
						Text:  "Less Waiting Time",
					},
				},
			})
		}
		log.Printf("[register] rec step 6: %v", reply.Text)
		rec.State = "first_impression"
		rec.Waiting = "email"
		app.SaveRecordToRedis(rec)
		q := fmt.Sprintf(`"first_impression" = '%s'`, reply.Text)
		_, err := app.UpdateUserInfo(user.ID, q)
		if err != nil {
			return err
		}
		// Done with registration then asking to init
		// app.Cancel(user.LineUserID)
		msg := localizer.MustLocalize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{
				ID:    "WhatIsYourEmail",
				Other: "What's your email address?",
			},
		})
		return app.replyText(replyToken, msg)
	}
	if rec.Waiting == "email" {
		// if(user.Email!=""){
		// 	// Done with registration then asking to init
		// 	app.Cancel(user.LineUserID)
		// 	return app.registratonDone(replyToken, localizer)
		// }
		checked_error := true
		// emailRegex := regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,4}$`)
		// checked_error = emailRegex.MatchString(reply.Text)
		_, email_parse_err := mail.ParseAddress(reply.Text)
		checked_error = email_parse_err == nil
		if checked_error==false {
			msg := localizer.MustLocalize(&i18n.LocalizeConfig{
				DefaultMessage: &i18n.Message{
					ID:    "WhatIsYourEmail",
					Other: "What's your email address?",
				},
			})
			return app.replyText(replyToken, msg)
		}
		log.Printf("[register] rec step 7: %v", reply.Text)
		q := fmt.Sprintf(`"email" = '%s'`, reply.Text)
		_, err := app.UpdateUserInfo(user.ID, q)
		if err != nil {
			return err
		}
		// Done with registration then asking to init
		app.Cancel(user.LineUserID)
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
	// check which is the step we required
	// we required: first_name & last_name & email
	// Old
	/*
	nextStep := "first_name"
	msg2 := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "WhatIsYourFirstName",
			Other: "What's your first name?",
		},
	})
	if user.FirstName != "" && user.LastName != "" {
		nextStep = "email"
		msg2 = localizer.MustLocalize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{
				ID:    "WhatIsYourEmail",
				Other: "What's email?",
			},
		})
	} else if user.FirstName != "" {
		nextStep = "last_name"
		msg2 = localizer.MustLocalize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{
				ID:    "WhatIsYourLastName",
				Other: "What's your last name?",
			},
		})
	}
	*/
	// New
	nextStep := "user_type"
	msg2 := Question{
		Text: localizer.MustLocalize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{
				ID:    "WhatIsYourUserType",
				Other: "Who are you?",
			},
		}),
		Buttons: []QuickReplyButton{
			{
				Label: localizer.MustLocalize(&i18n.LocalizeConfig{
					DefaultMessage: &i18n.Message{
						ID:    "A_Professor",
						Other: "Professor",
					},
				}),
				Text:  "Professor",
			},
			{
				Label:localizer.MustLocalize(&i18n.LocalizeConfig{
					DefaultMessage: &i18n.Message{
						ID:    "A_UniversityStaff",
						Other: "Staff of University",
					},
				}),
				Text:  "Staff of University",
			},
			{
				Label:localizer.MustLocalize(&i18n.LocalizeConfig{
					DefaultMessage: &i18n.Message{
						ID:    "A_Student",
						Other: "Student",
					},
				}),
				Text:  "Student",
			},
			{
				Label:localizer.MustLocalize(&i18n.LocalizeConfig{
					DefaultMessage: &i18n.Message{
						ID:    "A_PublicGeneral",
						Other: "Public General",
					},
				}),
				Text:  "Public General",
			},
		},
	}
	if user.UserType!=""{
		nextStep = "gender"
		msg2 = Question{
			Text: localizer.MustLocalize(&i18n.LocalizeConfig{
				DefaultMessage: &i18n.Message{
					ID:    "WhatIsYourgender",
					Other: "What's your gender?",
				},
			}),
			Buttons: []QuickReplyButton{
				{
					Label: localizer.MustLocalize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "A_Male",
							Other: "Male",
						},
					}),
					Text:  "Male",
				},
				{
					Label:localizer.MustLocalize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "A_Female",
							Other: "Female",
						},
					}),
					Text:  "Female",
				},
				{
					Label:localizer.MustLocalize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "A_NotSpecified",
							Other: "Not Specified",
						},
					}),
					Text:  "Not specified",
				},
			},
		}
	} else if user.Gender!="" {
		nextStep = "age"
		msg2 = Question{
			Text: localizer.MustLocalize(&i18n.LocalizeConfig{
				DefaultMessage: &i18n.Message{
					ID:    "WhatIsYourAge",
					Other: "What's your age range?",
				},
			}),
			Buttons: []QuickReplyButton{
				{
					Label: "1-19",
					Text:  "1-19",
				},
				{
					Label: "20-24",
					Text:  "20-24",
				},
				{
					Label: "25-29",
					Text:  "25-29",
				},
				{
					Label: "30-34",
					Text:  "30-34",
				},
				{
					Label: "35-39",
					Text:  "35-39",
				},
				{
					Label: "40-44",
					Text:  "40-44",
				},
				{
					Label: "45-49",
					Text:  "45-49",
				},
				{
					Label: "50-54",
					Text:  "50-54",
				},
				{
					Label: "55-59",
					Text:  "55-59",
				},
				{
					Label: "60-64",
					Text:  "60-64",
				},
				{
					Label:localizer.MustLocalize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "A_65Up",
							Other: "65 Up",
						},
					}),
					Text:  "65 Up",
				},
			},
		}
	} else if user.Age!="" {
		nextStep = "primary_mode"
		msg2 = Question{
			Text: localizer.MustLocalize(&i18n.LocalizeConfig{
				DefaultMessage: &i18n.Message{
					ID:    "WhatIsYourPrimaryMode",
					Other: "What's your primary transporation mode?",
				},
			}),
			Buttons: []QuickReplyButton{
				{
					Label: localizer.MustLocalize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "A_Walk",
							Other: "Walk",
						},
					}),
					Text:  "Walk",
				},
				{
					Label:localizer.MustLocalize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "A_EvBus",
							Other: "EV Bus",
						},
					}),
					Text:  "EV Bus",
				},
				{
					Label:localizer.MustLocalize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "A_Bicycle",
							Other: "Bicycle",
						},
					}),
					Text:  "Bicycle",
				},
				{
					Label:localizer.MustLocalize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "A_Scooter",
							Other: "Scooter",
						},
					}),
					Text:  "Scooter",
				},
				{
					Label:localizer.MustLocalize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "A_Taxi",
							Other: "Taxi (Including Grab)",
						},
					}),
					Text:  "Taxi",
				},
				{
					Label:localizer.MustLocalize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "A_PrivateCar",
							Other: "Private Car",
						},
					}),
					Text:  "Private Car",
				},
				{
					Label:localizer.MustLocalize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "A_Motorbike",
							Other: "Motorbike",
						},
					}),
					Text:  "Motorbike",
				},
				{
					Label:localizer.MustLocalize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "A_ShuttleBus",
							Other: "Shuttle Bus",
						},
					}),
					Text:  "Shuttle Bus",
				},
				{
					Label:localizer.MustLocalize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "A_MotorbikeTaxi",
							Other: "Motorbike Taxi",
						},
					}),
					Text:  "Motorbike Taxi",
				},
				{
					Label:localizer.MustLocalize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "A_Songthaew",
							Other: "Songthaew",
						},
					}),
					Text:  "Songthaew",
				},
			},
		}
	} else if user.PrimaryMode!="" {
		nextStep = "first_impression"
		msg2 = Question{
			Text: localizer.MustLocalize(&i18n.LocalizeConfig{
				DefaultMessage: &i18n.Message{
					ID:    "HowDidYouFeel",
					Other: "Please tell us your first impression for choosing SSVS",
				},
			}),
			Buttons: []QuickReplyButton{
				{
					Label: localizer.MustLocalize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "A_Innovative",
							Other: "Innovative",
						},
					}),
					Text:  "Innovative",
				},
				{
					Label:localizer.MustLocalize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "A_Comfortable",
							Other: "Comfortable",
						},
					}),
					Text:  "Comfortable",
				},
				{
					Label:localizer.MustLocalize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "A_Ecofriendly",
							Other: "Eco-friendly",
						},
					}),
					Text:  "Eco-friendly",
				},
				{
					Label:localizer.MustLocalize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "A_Convenient",
							Other: "Convenient",
						},
					}),
					Text:  "Convenient",
				},
				{
					Label:localizer.MustLocalize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "A_SafeAndTrustworthy",
							Other: "Safe & Trustworthy",
						},
					}),
					Text:  "Safe and Trustworthy",
				},
				{
					Label:localizer.MustLocalize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "A_FastTravelSpeed",
							Other: "Fast Travel Speed",
						},
					}),
					Text:  "Fast Travel Speed",
				},
				{
					Label:localizer.MustLocalize(&i18n.LocalizeConfig{
						DefaultMessage: &i18n.Message{
							ID:    "A_LessWaitingTime",
							Other: "Less Waiting Time",
						},
					}),
					Text:  "Less Waiting Time",
				},
			},
		}
	} else if user.FirstImpression!="" {
		nextStep = "email"
	}

	rec := ReservationRecord{
		UserID:     user.ID,
		LineUserID: user.LineUserID,
		State:      "init",
		Waiting:    nextStep,
		TripID:     -1,
		Title:      "register",
	}
	log.Printf("[register] init rec: %v", rec)
	err := app.SaveRecordToRedis(&rec)
	if err != nil {
		log.Printf("SAVE to Redis FAILED: %v", err)
		return err
	}
	if nextStep!="email"{
	return app.replyBack(replyToken, msg2)
	} else {
	msg_q := localizer.MustLocalize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{
				ID:    "WhatIsYourEmail",
				Other: "What's your email address?",
			},
		})
	replies := []string{msg1, msg_q}
	return app.replyText(replyToken, replies...)
	}
}
