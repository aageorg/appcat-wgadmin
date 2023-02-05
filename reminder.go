package main

import (
	"fmt"
	//	"strconv"
	"time"
)

const FirstAlert = int64(60 * 60 * 24 * 15)
const SecondAlert = int64(60 * 60 * 24 * 3)
const LastAlert = int64(60 * 60 * 24)

func toRemind(w Wg) bool {

	now := time.Now().Unix()
	delta := w.Paidtill - now
	period := w.Paidtill - w.Paidsince
	last, _ := db.GetLastAlert(w.Id)
	if (period/delta) < 4 || delta < 0 {
		return false
	}
	alerts := []int64{FirstAlert, SecondAlert, LastAlert}
	for i := 0; i < len(alerts); i++ {
		if now > w.Paidtill-alerts[i] && last < w.Paidtill-alerts[i] {
			return true
		}
	}

	return false
}

func expired(w Wg) bool {

	now := time.Now().Unix()
	if w.Paidtill != 0 && w.Paidtill-now < 0 {
		return true
	}
	return false
}

func Controller() {

	wgs := db.getAllVpns()
	for _, wg := range wgs {
		if expired(wg) {
			//	fmt.Println("disable expired")
			for i, _ := range wg.Peers {
				wg.Peers[i].Disable()
			}
			if wg.Server.IPAddress == nil {
				wg.Server = cfg.Server
			}
			err := wg.Update()
			if err != nil {
				fmt.Println(err.Error())
				continue
			}
			wg.Paidsince = 0
			_, err = db.addVpn(wg)
			if err != nil {
				fmt.Println(err.Error())
			}
			continue
		}
		if toRemind(wg) {
			bot := Chatbot{Chat_id: wg.Owner.Chat}
			bot.Message = make(map[string]string)
			bot.Message["en"] = TgMessage("Hello, " + wg.Owner.Name + "! Don't forget to pay for your service before " + time.Unix(wg.Paidtill, 0).Format("2 Jan 2006 15:04"))
			bot.Reply()
			db.SetAlert(wg.Id)
		}

	}

}
