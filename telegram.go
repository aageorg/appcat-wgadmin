package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
)

type ReplyKeyboardMarkup struct {
	Keyboard        [][]KeyboardButton `json:"keyboard,omitempty"`
	ResizeKeyboard  bool               `json:"resize_keyboard,omitempty"`
	OneTimeKeyboard bool               `json:"one_time_keyboard,omitempty"`
	RemoveKeyboard  bool               `json:"remove_keyboard,omitempty"`
}

type KeyboardButton struct {
	Text string            `json:"text"`
	Lang map[string]string `json:"-"`
}

type Message struct {
	Message_id int
	From       User
	Chat       Chat
	Date       int
	Text       string
}

type Chat struct {
	Id         int64  `json:"id"`
	First_name string `json:"first_name"`
	Username   string `json:"username"`
	Chattype   string `json:"type"`
}

type Update struct {
	Update_id int
	Message   Message `json:"message"`
}

type Chatbot struct {
	Chat_id        int64               `json:"chat_id"`
	User           User                `json:"-"`
	My_new_message string              `json:"text"`
	Message        map[string]string   `json:"-"`
	ParseMode      string              `json:"parse_mode"`
	ReplyMarkup    ReplyKeyboardMarkup `json:"reply_markup,omitempty"`
}

func (c *Chatbot) RemoveMarkup() {
	revoked := new(Chatbot)
	revoked.Chat_id = c.Chat_id
	revoked.User = c.User
	revoked.My_new_message = c.My_new_message
	revoked.Message = c.Message
	c = revoked
}
func escapeChars(symb []string, str string) string {
	res := str
	if len(symb) == 0 {
		return res
	} else {
		for _, s := range symb {
			res = strings.ReplaceAll(res, s, `\`+s)
		}
	}
	return res
}

func CodeBlock(str string) string {
	res := "```\n"
	res += escapeChars([]string{"\\", "`"}, str)
	res += "\n```"
	return res
}

func TgMessage(str string) string {
	res := escapeChars([]string{"_", "*", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!", `\\`}, str)

	return res
}

func (c Chatbot) Reply() {
	if c.Message != nil {
		if val, ok := c.Message["en"]; ok {
			c.My_new_message = val
		}
		for lang, text := range c.Message {
			if lang == c.User.Tg_language_code {
				c.My_new_message = text
			}
		}
	}

	c.ParseMode = "MarkdownV2"
	body, err := json.Marshal(c)
	if err != nil {
		log.Fatal(err)
	}

	requestBody := bytes.NewReader(body)

	res, err := http.Post(cfg.Tg.getUrl()+"/sendMessage",
		"application/json; charset=UTF-8",
		requestBody,
	)
	if err != nil {
		log.Fatal(err)
	}
	data, _ := ioutil.ReadAll(res.Body)
	res.Body.Close()
	fmt.Printf("%s\n", data)
}

func (c Chatbot) Report() {
	admin, err := db.getAdmin()
	if err != nil || admin.Chat == 0 {
		c.Message = make(map[string]string)
		c.Message["en"] = "An error occured. Please write about it to @" + cfg.Tg.Admin
		c.Message["ru"] = "Произошла ошибка, пожалуйста, напишите об этом @" + cfg.Tg.Admin
		c.Reply()
		return
	}
	c.Chat_id = admin.Chat
	c.Reply()

}

func (upd *Update) Process() {
	upd.Message.From.Role = Role{Admin: false, Customer: false, Guest: false}

	if upd.Message.From.Username == cfg.Tg.Admin {
		upd.Message.From.Role.Admin = true
	}
	if db.user(upd.Message.From.Tg_id) {
		upd.Message.From.Role.Customer = true
	} else {
		upd.Message.From.Role.Guest = true
	}
	ch, ok := awaiting[upd.Message.Chat.Id]
	if ok {
		ch <- upd.Message.Text

	}
	upd.Message.From.Chat = upd.Message.Chat.Id

	if strings.HasPrefix(upd.Message.Text, "/") {
		a := Command{
			Name: strings.TrimLeft(upd.Message.Text, "/"),
			User: upd.Message.From,
			Chat: upd.Message.Chat,
		}
		a.exec()
	}

}
