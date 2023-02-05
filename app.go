package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
)

var cfg Config
var db Database
var tg Telegram
var Mu sync.Mutex

func Serve() {

	http.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {

		var secret []string
		secret = req.Header["X-Telegram-Bot-Api-Secret-Token"]
		if len(secret) == 0 || secret[0] != cfg.Tg.WebhookSecret {
			res.Header().Set("Content-Type", "text/html")
			res.WriteHeader(http.StatusForbidden)
			fmt.Fprintf(res, "403! Forbidden")
			return
		}
		b, err := io.ReadAll(req.Body)
		if err != nil {
			log.Fatalln(err)
		}
		var upd Update
		json.Unmarshal(b, &upd)
		if upd.Message.From.Tg_is_bot == true {
			return
		}
		go upd.Process()

	})

	log.Fatal(http.ListenAndServeTLS(":"+cfg.Tg.WebhookPort, "/etc/letsencrypt/live/"+cfg.Tg.WebhookUri+"/fullchain.pem", "/etc/letsencrypt/live/"+cfg.Tg.WebhookUri+"/privkey.pem", nil))
}

func main() {
	err := cfg.Parse()
	if err != nil {
		fmt.Println("Configuration file error: " + err.Error())
		return
	}

	err = db.init()
	if err != nil {
		log.Fatal(err)
	}

	secret := db.getWebhookSecret()
	if secret == "" {
		hash := md5.Sum([]byte(strconv.FormatInt(time.Now().Unix(), 10)))
		s := hex.EncodeToString(hash[:])
		err := cfg.Tg.setWebhook(s)
		if err != nil {
			fmt.Println(err)
			return
		}
		err = db.saveWebhookSecret([]byte(s))
		if err != nil {
			fmt.Println(err)
			return
		}
		cfg.Tg.WebhookSecret = s
	} else {
		cfg.Tg.WebhookSecret = secret
	}

	go func() {
		for {
			Mu.Lock()
			Controller()
			Mu.Unlock()
			time.Sleep(5 * time.Minute)
		}
	}()
	Serve()

}
