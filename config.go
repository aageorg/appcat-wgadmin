package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"reflect"
	"strings"
)

const CfgPath = "appcat.conf"

type Telegram struct {
	Apikey        string `json:"apikey"`
	Secret        string `json:"secret"`
	WebhookUri    string `json:"webhook_url"`
	WebhookPort   string `json:"webhook_port"`
	WebhookSecret string `json:"-"`
	Admin         string `json:"admin"`
	Url           string `json:"url"`
}

func (t *Telegram) getUrl() string {

	return "https://" + t.Url + "/bot" + t.Apikey

}

func (t *Telegram) setWebhook(secret string) error {
	reqBody := strings.NewReader("{\"url\" : \"" + t.WebhookUri + ":" + t.WebhookPort + "\",\"secret_token\" : \"" + secret + "\"}")
	res, err := http.Post(t.getUrl()+"/setWebhook", "application/json; charset=UTF-8", reqBody)
	if err != nil {
		return err
	}
	data, _ := ioutil.ReadAll(res.Body)
	res.Body.Close()
	var r Response
	err = json.Unmarshal(data, &r)
	if err != nil {
		return err
	}
	if r.Ok == false {
		return errors.New("Cannot set webhook")
	}
	return nil
}

type Regru struct {
	Username string `json:"login"`
	Password string `json:"passwd"`
}

type Config struct {
	Tg       Telegram `json:"telegram"`
	Reg      Regru    `json:"regru"`
	Database string   `json:"database"`
	Server   Server   `json:"server"`
}

func removeEsc(slice []byte) []byte {
	esc := [...]byte{' ', '\n', '\t'}
	var result []byte
	value := false
	for i, j := range slice {
		keep := true
		if j == '"' {
			if value == false {
				value = true
			} else {
				if slice[i-1] != '\\' {
					value = false
				} else {
					value = true
				}
			}
		}
		for _, s := range esc {
			if s == j && value == false {
				keep = false
				break
			}
		}
		if keep == true {
			result = append(result, j)
		}
	}
	return result
}

func readconf() []byte {
	var result []byte
	f, err := os.Open(CfgPath)
	if err != nil {
		log.Fatal(err)
	}

	defer f.Close()

	scanner := bufio.NewScanner(f)
	var buf []byte
	for scanner.Scan() {
		buf = removeEsc(scanner.Bytes())
		value := false
		for i := 0; i < len(buf); i++ {
			if buf[i] == '"' {
				if value == false {
					value = true
				} else {
					if buf[i-1] != '\\' {
						value = false
					} else {
						value = true
					}
				}
			}
			if buf[i] == '#' && value == false {
				buf = buf[:i]
				break
			}
		}
		if len(buf) == 0 {
			continue
		}
		result = append(result, buf...)
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	return result

}

func isValidS(t interface{}, s string) bool {

	rt := reflect.TypeOf(t)

	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if f.Name == s {

		}
	}
	return false
}

func (c *Config) Parse() error {
	cfg := readconf()
	err := json.Unmarshal(cfg, &c)
	if err != nil {
		return err
	}

	return nil
}
