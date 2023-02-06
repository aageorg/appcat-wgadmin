package main

import (
	"encoding/base64"
	"errors"
	"math/rand"
	"net"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)

func (w *Wg) AddPeer() error {

	isValidKey := func(key string) bool {
		data, err := base64.StdEncoding.DecodeString(key)
		if err != nil {
			return false
		}
		if len(data) != 32 {
			return false
		}
		return true

	}

	copyip := func(ip net.IP) (net.IP, error) {
		if ip.To4 == nil {
			return nil, errors.New("IP address " + ip.String() + " is not a valid IPv4 address") 
		}
		var newip net.IP
		return append(newip, ip[len(ip)-4:]...), nil
	}

	unic := func(ip net.IP, addr []net.IPNet) bool {
		for _, net := range addr {
			if net.Contains(ip) {
				return false
			}
		}
		return true
	}

	incr := func(ip *net.IP) {
		for i := len(*ip) - 1; i >= len(*ip)-4; i-- {
			if (*ip)[i] < 254 {
				(*ip)[i]++
				break
			}
			(*ip)[i] = 1
			continue
		}

	}
	bot := Chatbot{Chat_id: w.Owner.Chat}
	bot.Message = make(map[string]string)
	bot.Message["en"] = TgMessage("Let's create a new peer for wg" + strconv.Itoa(w.Id) + ". Send me please your peer's public key.")
	bot.Reply()

	ch := make(chan string)
	awaiting[w.Owner.Chat] = ch
	key := <-ch
	delete(awaiting, w.Owner.Chat)
	close(ch)

	if !isValidKey(key) {
		return errors.New("This key is not valid wireguard public key.")

	}
	var nets []net.IPNet
	if w.IPNat == nil {
		fromIP, err := copyip(w.IPAddress)
	if err != nil {
		return err
}

		fromIP[3] = 128
		w.IPNat = &net.IPNet{net.IP(fromIP).Mask(net.CIDRMask(25, 32)), net.IPMask{255, 255, 255, 128}}
	}
	nets = append(nets, net.IPNet{net.IP(w.IPNat.IP).Mask(net.CIDRMask(32, 32)), net.IPMask{255, 255, 255, 255}})
	for _, p := range w.Peers {
		if p.PublicKey == key {
			return errors.New("The peer with given public key is already exist")
		}
		net := net.IPNet{net.IP(p.IPAddress).Mask(net.CIDRMask(32, 32)), net.IPMask{255, 255, 255, 255}}
		nets = append(nets, net)
	}
	ip, err := copyip(w.IPNat.IP)
	if err != nil {
		return err
}
	for !unic(ip, nets) {
		incr(&ip)
	}
	if !w.IPNat.Contains(ip) {
		return errors.New("No availible addresses in nated range for new peers")
	}

	var peer Peer
	peer.PublicKey = key
	peer.IPAddress = ip
	peer.Active = true
	bot.Message["en"] = TgMessage("Give a short name to your peer. For example, \"My Iphone\" or \"Домашний комп\"")
	bot.Reply()
	ch = make(chan string)
	awaiting[w.Owner.Chat] = ch
	name := <-ch
	delete(awaiting, w.Owner.Chat)
	close(ch)
	if len(name) > 30 {
		name = name[:30]
	}

	peer.Name = strings.ReplaceAll(name, "\n", " ")
	// next logic
	bot.Message["en"] = TgMessage("Now you can configure your device:\n\n")
	var config string
	config = "[Interface]\n"
	config += "PrivateKey = <your private key>\n"
	config += "Address = " + peer.IPAddress.String() + "/24\n# Optional, you can choose your own\n# DNS servers or remove next string:\n"
	config += "DNS = 77.88.8.8,77.88.8.1\n\n"
	config += "[Peer]\n"
	config += "PublicKey = " + w.PublicKey + "\n"
	config += "AllowedIPs = 0.0.0.0/0\n"
	config += "Endpoint = " + w.Server.IPAddress.String() + ":" + strconv.Itoa(w.Port) + "\n"
	bot.Message["en"] += CodeBlock(config)
	bot.Reply()
	w.Peers = append(w.Peers, peer)

	return nil
}

type Invitation struct {
	Code   string
	User   User
	Create int64
	Expire int64
}

type Command struct {
	Name        string
	Description map[string]string
	Permitted   []string
	User        User
	Chat        Chat
	mu          sync.Mutex
}

var Commands = map[string]Command{
	"start": Command{
		Name:        "start",
		Description: map[string]string{"en": "Show list of availible commands", "ru": "Показать список доступных команд"},
		Permitted:   []string{"Admin", "Customer", "Guest"},
	},
	"register": Command{
		Name:        "register",
		Description: map[string]string{"en": "Create a new user account", "ru": "Зарегистрироваться в сервисе"},
		Permitted:   []string{"Admin", "Guest"},
	},
	"newvpn": Command{
		Name:        "newvpn",
		Description: map[string]string{"en": "Create a new wireguard connection", "ru": "Создать новое wireguard подключение"},
		Permitted:   []string{"Admin", "Customer"},
	},
	"removevpn": Command{
		Name:        "removevpn",
		Description: map[string]string{"en": "Remove a wireguard connection", "ru": "Удалить wireguard подключение"},
		Permitted:   []string{"Admin", "Customer"},
	},
	"removevpnadmin": Command{
		Name:        "removevpnadmin",
		Description: map[string]string{"en": "Remove a wireguard connection", "ru": "Удалить wireguard подключение"},
		Permitted:   []string{"Admin"},
	},
	"addpeer": Command{
		Name:        "addpeer",
		Description: map[string]string{"en": "Add a new peer to existing Wireguarg VPN", "ru": "Добавить устройство (peer) к wireguard подключению"},
		Permitted:   []string{"Admin", "Customer"},
	},
	"getusers": Command{
		Name:        "getusers",
		Description: map[string]string{"en": "List of users", "ru": "Список пользователей"},
		Permitted:   []string{"Admin"},
	},
	"getvpns": Command{
		Name:        "getvpns",
		Description: map[string]string{"en": "List of vpn with owners", "ru": "Список vpn по пользователям"},
		Permitted:   []string{"Admin"},
	},
	"myvpns": Command{
		Name:        "myvpns",
		Description: map[string]string{"en": "List of yours vpn", "ru": "Список vpn пользователя"},
		Permitted:   []string{"Admin", "Customer"},
	},
	"setpaid": Command{
		Name:        "setpaid",
		Description: map[string]string{"en": "Set new payment period", "ru": "Установить оплаченный период"},
		Permitted:   []string{"Admin"},
	},
	"keypair": Command{
		Name:        "keypair",
		Description: map[string]string{"en": "Generates random wireguard keypair", "ru": "Генерирует случайную пару Wireguard ключей"},
		Permitted:   []string{"Admin", "Customer"},
	},
	"setplan": Command{
		Name:        "setplan",
		Description: map[string]string{"en": "Set a tarif plan for user", "ru": "Установить тарифный план для пользователя"},
		Permitted:   []string{"Admin"},
	},
	"rate": Command{
		Name:        "rate",
		Description: map[string]string{"en": "Shows your rate plan", "ru": "Узнать тарифный план"},
		Permitted:   []string{"Admin", "Customer"},
	},
	"payment": Command{
		Name:        "payment",
		Description: map[string]string{"en": "Shows your billing info", "ru": "Узнать информацию по своим услугам"},
		Permitted:   []string{"Admin", "Customer"},
	},
	"invite": Command{
		Name:        "invite",
		Description: map[string]string{"en": "Invite somebody", "ru": "Сделать приглашение"},
		Permitted:   []string{"Admin", "Customer"},
	},
	"adoptvpn": Command{
		Name:        "adoptvpn",
		Description: map[string]string{"en": "Adopt preconfigured VPN", "ru": "Добавить уже настроенный на сервере VPN"},
		Permitted:   []string{"Admin"},
	},
}

func (a Command) exec() {
	var bot Chatbot
	_, ok := Commands[a.Name]

	if ok {
		if a.granted() {
			reflect.ValueOf(&a).MethodByName(strings.Title(a.Name)).Call([]reflect.Value{})
		} else {
			bot = Chatbot{
				Chat_id:        a.Chat.Id,
				My_new_message: TgMessage("You are not permitted to run /" + a.Name),
			}
			bot.Reply()
		}
	} else {
		bot = Chatbot{
			Chat_id: a.Chat.Id,
			Message: map[string]string{
				"en": TgMessage("Unknown command /" + a.Name),
				"ru": TgMessage("Команда " + a.Name + " не найдена"),
			},
		}
		bot.Reply()
	}
}

func (r *Role) getField(field string) bool {
	reflect := reflect.ValueOf(r).Elem()
	return reflect.FieldByName(field).Interface().(bool)
}

func (a Command) granted() bool {

	for _, wanted_role := range Commands[a.Name].Permitted {
		if a.User.Role.getField(wanted_role) == true {
			return true
		}
	}
	return false
}

func (a Command) Start() {
	bot := Chatbot{Chat_id: a.Chat.Id}
	bot.Message = make(map[string]string)
	for _, cmd := range Commands {
		cmd.User = a.User
		if cmd.granted() {
			for lang, desc := range cmd.Description {
				bot.Message[lang] += TgMessage("/" + cmd.Name + " - " + desc + "\n")
			}
		}
	}
	bot.Reply()
}

func (a Command) Getusers() {
	bot := Chatbot{Chat_id: a.Chat.Id}
	bot.Message = make(map[string]string)
	users := db.getUsers()
	var output string
	for i, u := range users {
		username := ""
		if len(u.Username) != 0 {
			username = " (@" + u.Username + ")\n"
		}
		output += strconv.Itoa(i+1) + ". " + u.Name + username
	}
	//bot.Message["en"] = TgMessage("Registered users:")
	bot.Message["en"] += CodeBlock(output)
	bot.Reply()
}

func (a Command) Getvpns() {
	bot := Chatbot{Chat_id: a.Chat.Id}
	bot.Message = make(map[string]string)
	users := db.getUsers()
	var output string
	for _, u := range users {
		wgs := db.getVpns(u.Tg_id)

		if len(wgs) > 0 {
			output = u.Name + " (@" + u.Username + ")\n"
			for i, w := range wgs {
				output += strconv.Itoa(i+1) + ". wg" + strconv.Itoa(w.Id) + " (" + w.Network.String() + ")\n"
			}
			bot.Message["en"] += CodeBlock(output)
		}
	}
	if len(output) == 0 {
		bot.Message["en"] = "The list is empty"
	}

	bot.Reply()
}

func (a Command) Myvpns() {
	bot := Chatbot{Chat_id: a.Chat.Id}
	bot.Message = make(map[string]string)
	wgs := db.getVpns(a.User.Tg_id)
	if len(wgs) > 0 {
		for i, w := range wgs {
			var output string
			output += strconv.Itoa(i+1) + ". Interface: wg" + strconv.Itoa(w.Id)
			if len(w.Peers) > 0 {
				output += "\n   Peers:"
			}
			for _, peer := range w.Peers {
				output += "\n   - " + peer.Name + " (" + peer.IPAddress.String() + ")"
			}
			bot.Message["en"] += CodeBlock(output)

		}
		//	bot.Message["en"] += output
	} else {
		bot.Message["en"] = "The list is empty"
	}

	bot.Reply()
}

func (a Command) Register() {

	bot := Chatbot{Chat_id: a.Chat.Id}
	bot.Message = make(map[string]string)

	if a.User.Username != cfg.Tg.Admin {

		bot.Message["ru"] = TgMessage("Прекрасно, " + a.User.Name + ", пришлите ваш код приглашения")
		bot.Message["en"] = TgMessage("Ok, " + a.User.Name + ", please, type the invitation code")
		bot.Reply()

		ch := make(chan string)
		awaiting[a.Chat.Id] = ch
		code := <-ch
		delete(awaiting, a.Chat.Id)
		close(ch)
		err := db.getInvitation(code)
		if err != nil {
			bot.Message["en"] = TgMessage(err.Error())
			bot.Reply()
			return

		}
		db.dropInvitation(code)

		a.mu.Lock()
		admin, err := db.getAdmin()
		if err != nil {
			bot.Message["ru"] = TgMessage("В настоящее время регистрация невозможна")
			bot.Message["en"] = TgMessage("Registration is temporary unavailible")
			bot.Reply()
			return
		}
		awaiting[admin.Chat] = ch
		bot.Message["ru"] = TgMessage("Пользователь " + a.User.Name + " (@" + a.User.Username + ") запрашивает регистрацию. Разрешить?")
		bot.Message["en"] = TgMessage("User " + a.User.Name + " (@" + a.User.Username + ") requests registration. Allow?")
		bot.ReplyMarkup = ReplyKeyboardMarkup{
			Keyboard: [][]KeyboardButton{{
				KeyboardButton{
					Text: "Yes",
				},
				KeyboardButton{
					Text: "No",
				},
			}},

			ResizeKeyboard:  true,
			OneTimeKeyboard: true,
		}
		bot.Report()
		bot.RemoveMarkup()

		//		admin = db.getAdmin()

		ch = make(chan string)
		awaiting[admin.Chat] = ch
		adminResponse := <-ch
		delete(awaiting, admin.Chat)
		close(ch)

		bot.Message["ru"] = "Ok"
		bot.Message["en"] = "Ok"
		bot.ReplyMarkup = ReplyKeyboardMarkup{RemoveKeyboard: true}
		bot.Report()
		bot.RemoveMarkup()
		if adminResponse == "Yes" {

			err := db.addUser(a.User)
			if err != nil {
			bot.Message["en"] = TgMessage("Register() error on writing to db: "+err.Error())
			bot.Report()
			return
			}
			bot.Message["ru"] = TgMessage("Вы зарегистрировались")
			bot.Message["en"] = TgMessage("Done!")
			bot.Reply()
		} else {
			bot.Message["ru"] = TgMessage("Адимн не одобрил вашу заявку на регистрацию")
			bot.Message["en"] = TgMessage("Your registration request has been rejected")
			bot.Reply()

		}

		a.mu.Unlock()
	} else {

		err := db.addUser(a.User)
		if err != nil {
			bot.Message["en"] = TgMessage("Register() error on writing to db: "+err.Error())
			bot.Report()
			return
		}
		err = db.setAdmin(a.User)
		if err != nil {
			bot.Message["en"] = TgMessage("Register() error on writing to db: "+err.Error())
			bot.Report()
			return
		}

		bot.RemoveMarkup()
		bot.ReplyMarkup = ReplyKeyboardMarkup{RemoveKeyboard: true}
		bot.Message["ru"] = TgMessage("Вы зарегистрировались как администратор")
		bot.Message["en"] = TgMessage("You are registered as administraror")
		bot.Reply()

	}
	delete(awaiting, a.Chat.Id)

}

func (a Command) Newvpn() {
	bot := Chatbot{Chat_id: a.Chat.Id}
	wg := Wg{Owner: a.User}
	wgid, err := db.addVpn(wg)
	if err != nil {
			bot.Message["en"] = TgMessage("Newvpn() error on writing to db: "+err.Error())
			bot.Report()
			return
	}
	wg.Id = wgid
	wg.Server = cfg.Server
	wg.New()
	bot.My_new_message = TgMessage("Interface " + strconv.Itoa(wgid) + " created")
	bot.Reply()
	err = wg.AddPeer()
	if err != nil {
		bot.My_new_message = TgMessage(err.Error() + "\n")
		bot.My_new_message += TgMessage("No peers associated with an interface. You can add peer later with /addpeer")
		bot.Reply()
	}
	wg.Update()
	now := time.Now()
	wg.Paidtill = now.AddDate(0, 0, 7).Unix()

	wgid, err = db.addVpn(wg)
	if err != nil {
					bot.Message["en"] = TgMessage("Newvpn() error on writing to db: "+err.Error())
			bot.Report()
			return
	}
}

func (a Command) Keypair() {
	bot := Chatbot{Chat_id: a.Chat.Id}
	var wg Wg
	wg.Server = cfg.Server
	err := wg.Genkeys()
	if err != nil {
		bot.My_new_message = TgMessage(err.Error())
		bot.Reply()
		return
	}

	bot.My_new_message = TgMessage("The first is private, the second is public:")
	bot.Reply()

	bot.My_new_message = CodeBlock(wg.PrivateKey)
	bot.Reply()

	bot.My_new_message = CodeBlock(wg.PublicKey)
	bot.Reply()
}

func (a Command) Addpeer() {
	wgs := db.getVpns(a.User.Tg_id)
	bot := Chatbot{Chat_id: a.Chat.Id}
	bot.Message = make(map[string]string)

	addpeer := func(wgid int) {
		Mu.Lock()
		defer Mu.Unlock()
		var wg Wg
		wg = db.getVpn(wgid)
		err := wg.AddPeer()
		if err != nil {
			bot.Message["en"] = TgMessage("You can not create such peer for this VPN because of error: \n\"" + err.Error() + "\"\nTry again or use another VPN for the peer")
			bot.Reply()
			return

		}
		if wg.Server.IPAddress == nil {
			wg.Server = cfg.Server
		}
		wg.Update()
		_, err = db.addVpn(wg)
		if err != nil {
			bot.Message["en"] = TgMessage("Newvpn() error on writing to db: "+err.Error())
			bot.Report()
			return
		}
	}

	if len(wgs) == 0 {
		bot.Message["en"] = TgMessage("You have no Wireguard VPN connections. Create one wirh /newvpn, then you'll be able to add a new peer")
		bot.Reply()
		return

	}

	if len(wgs) == 1 {
		addpeer(wgs[0].Id)
		return
	}
	if len(wgs) > 1 {
		bot.Message["en"] = TgMessage("Please select a Wireguard VPN where we'll add a new peer device.\n\n")
		for k, v := range wgs {
			bot.Message["en"] += TgMessage(strconv.Itoa(k) + " - wg" + strconv.Itoa(v.Id) + "\n")
		}

		bot.Reply()
		ch := make(chan string)
		awaiting[a.Chat.Id] = ch
		wgid, err := strconv.Atoi(<-ch)
		delete(awaiting, a.Chat.Id)
		close(ch)
		if err != nil || wgid > len(wgs)-1 || wgid < 0 {
			bot.Message["en"] = TgMessage("Wrong VPN number. If you still want add peer, run /addpeer again")
			bot.Reply()
			return
		}
		addpeer(wgs[wgid].Id)
		return
	}

}

func (a Command) Removevpn() {
	bot := Chatbot{Chat_id: a.Chat.Id}
	bot.Message = make(map[string]string)
	bot.Message["en"] = TgMessage("Note, when you remove VPN, all linked devices will be removed too. If you want ro remove the device, use /removepeer\n\n")
	wgs := db.getVpns(a.User.Tg_id)
	if len(wgs) > 0 {
		bot.Message["en"] += TgMessage("Which one VPN would you like to remove (enter the number)?\n")
		for k, v := range wgs {
			bot.Message["en"] += TgMessage(strconv.Itoa(k) + " - wg" + strconv.Itoa(v.Id) + "\n")
		}
		bot.Reply()
		ch := make(chan string)
		awaiting[a.Chat.Id] = ch
		wgid, err := strconv.Atoi(<-ch)
		delete(awaiting, a.Chat.Id)
		close(ch)
		if err != nil || wgid > len(wgs)-1 || wgid < 0 {
			bot.Message["en"] = TgMessage("Wrong VPN number. If you still want to remove VPN, run /removevpn again")
			bot.Reply()
			return
		}

		wg := Wg{Id: wgs[wgid].Id, Owner: a.User}
		err = db.removeVpn(wg)
		if err != nil {
			bot.Message["en"] = TgMessage(err.Error())
			bot.Reply()
			bot.Message["en"] = TgMessage("An error occured while user " + a.User.Name + " (@" + a.User.Username + ") tried to remove a VPN connection: " + err.Error())
			bot.Report()
			return
		}
		if wg.Server.IPAddress == nil {
			wg.Server = cfg.Server
		}
		err = wg.Remove()
		if err != nil {
			bot.Message["en"] = TgMessage("An error occured while user " + a.User.Name + " (@" + a.User.Username + ") tried to remove a VPN connection: " + err.Error())
			bot.Report()
			return
		}

		bot.Message["en"] = TgMessage("VPN wg" + strconv.Itoa(wg.Id) + " removed")
		bot.Reply()
	} else {
		bot.Message["en"] = TgMessage("You have no VPN connections. Create one with /newvpn")
		bot.Reply()

	}
}

func (a Command) Setpaid() {
	Mu.Lock()
	defer Mu.Unlock()
	bot := Chatbot{Chat_id: a.Chat.Id}
	bot.Message = make(map[string]string)
	wgs := db.getAllVpns()
	if len(wgs) == 0 {
		bot.Message["en"] = TgMessage("The list is empty\n")
		bot.Reply()
		return

	}
	bot.Message["en"] += TgMessage("For which VPN would you like to set a paid period (enter the number)?\n")
	var listmenu string
	for k, v := range wgs {
		username := ""
		if v.Owner.Username != "" {
			username = " (@" + v.Owner.Username + ")"
		}
		listmenu += strconv.Itoa(k+1) + " - wg" + strconv.Itoa(v.Id) + " user: " + v.Owner.Name + username + "\n"
	}
	bot.Message["en"] += CodeBlock(listmenu)
	bot.Reply()
	ch := make(chan string)
	awaiting[a.Chat.Id] = ch
	wgid, err := strconv.Atoi(<-ch)
	delete(awaiting, a.Chat.Id)
	close(ch)
	if err != nil || wgid > len(wgs) || wgid < 1 {
		bot.Message["en"] = TgMessage("Wrong VPN number. Run /setpaid again")
		bot.Reply()
		return
	}
	wgid = wgid - 1
	bot.Message["en"] = TgMessage("Enter the number of paid months\n")
	bot.Reply()
	ch = make(chan string)
	awaiting[a.Chat.Id] = ch
	dur, err := strconv.Atoi(<-ch)
	delete(awaiting, a.Chat.Id)
	close(ch)
	if err != nil || dur < 0 || dur > 100 {
		bot.Message["en"] = TgMessage("Wrong period. Run /setpaid again")
		bot.Reply()
		return
	}
	now := time.Now()
	if wgs[wgid].Paidsince == 0 {
		wgs[wgid].Paidsince = now.Unix()
	}
	wgs[wgid].Paidtill = now.AddDate(0, dur, 0).Unix()
	for i, _ := range wgs[wgid].Peers {

		wgs[wgid].Peers[i].Enable()
	}
	if wgs[wgid].Server.IPAddress == nil {
		wgs[wgid].Server = cfg.Server
	}
	_, err = db.addVpn(wgs[wgid])
	if err != nil {
		bot.Message["en"] = TgMessage("Setpaid() error on writing to db: "+err.Error())
		bot.Report()
		return
	}
	err = wgs[wgid].Update()
	if err != nil {
		bot.Message["en"] = TgMessage("Setpaid() error on calling RPC: "+err.Error())
		bot.Report()
		return
	}

	bot.Message["en"] = TgMessage("A new paid period is set for vpn wg" + strconv.Itoa(wgs[wgid].Id) + " till " + time.Unix(wgs[wgid].Paidtill, 0).Format("02.01.2006 15:04"))
	bot.Reply()
	bot.Chat_id = wgs[wgid].Owner.Chat
	bot.Message["en"] = TgMessage("Thanks for your payment, " + wgs[wgid].Owner.Name + ". The service wg" + strconv.Itoa(wgs[wgid].Id) + " is paid till " + time.Unix(wgs[wgid].Paidtill, 0).Format("02.01.2006 15:04"))
	bot.Reply()

}

func (a Command) Removevpnadmin() {
	Mu.Lock()
	defer Mu.Unlock()
	bot := Chatbot{Chat_id: a.Chat.Id}
	bot.Message = make(map[string]string)
	wgs := db.getAllVpns()
	if len(wgs) == 0 {
		bot.Message["en"] = TgMessage("The list is empty\n")
		bot.Reply()
		return

	}
	bot.Message["en"] += TgMessage("Which VPN would you like to remove (enter the number)?\n")
	var listmenu string
	for k, v := range wgs {
		username := ""
		if v.Owner.Username != "" {
			username = " (@" + v.Owner.Username + ")"
		}
		listmenu += strconv.Itoa(k+1) + " - wg" + strconv.Itoa(v.Id) + " user: " + v.Owner.Name + username + "\n"
	}
	bot.Message["en"] += CodeBlock(listmenu)
	bot.Reply()
	ch := make(chan string)
	awaiting[a.Chat.Id] = ch
	wgid, err := strconv.Atoi(<-ch)
	delete(awaiting, a.Chat.Id)
	close(ch)
	if err != nil || wgid > len(wgs) || wgid < 1 {
		bot.Message["en"] = TgMessage("Wrong VPN number. Run /setpaid again")
		bot.Reply()
		return
	}
	wgid = wgid - 1
	err = db.removeVpn(wgs[wgid])
	if err != nil {
		bot.Message["en"] = TgMessage(err.Error())
		bot.Reply()
		return
	}
	bot.Message["en"] = TgMessage("VPN wg" + strconv.Itoa(wgs[wgid].Id) + " sucessfully removed from db. Remove it manual on server's side")
	bot.Reply()

}

func (a Command) Setplan() {
	bot := Chatbot{Chat_id: a.Chat.Id}
	bot.Message = make(map[string]string)
	users := db.getUsers()
	if len(users) == 0 {
		bot.Message["en"] = TgMessage("No users found")
		bot.Reply()
		return
	}
	bot.Message["en"] = TgMessage("For which user would you like to set a rate plan (enter the number)?\n")
	var listmenu string
	for i, v := range users {
		username := ""
		if len(v.Username) != 0 {
			username = " (@" + v.Username + ")\n"
		}
		listmenu += strconv.Itoa(i+1) + ". " + v.Name + username
	}
	bot.Message["en"] += CodeBlock(listmenu)
	bot.Reply()
	ch := make(chan string)
	awaiting[a.Chat.Id] = ch
	userid, err := strconv.Atoi(<-ch)
	delete(awaiting, a.Chat.Id)
	close(ch)
	if err != nil || userid > len(users) || userid < 1 {
		bot.Message["en"] = TgMessage("Wrong number. Run /setplan again")
		bot.Reply()
		return
	}
	userid = userid - 1
	bot.Message["en"] = TgMessage("Enter the monthly amount per device\n")
	bot.Reply()
	ch = make(chan string)
	awaiting[a.Chat.Id] = ch
	plan, err := strconv.Atoi(<-ch)
	delete(awaiting, a.Chat.Id)
	close(ch)
	if err != nil || plan < 0 {
		bot.Message["en"] = TgMessage("Wrong amount. Run /setplan again")
		bot.Reply()
		return
	}
	users[userid].Plan = plan
	err = db.addUser(users[userid])
	if err != nil {
		bot.Message["en"] = TgMessage("Setplan() error on writting to db: "+err.Error())
		bot.Report()
		return
	}
	bot.Message["en"] = TgMessage("A new rate is set for user " + users[userid].Name + "  -  " + strconv.Itoa(plan) + " rub/month per device")
	bot.Reply()

}

func (a Command) Rate() {
	bot := Chatbot{Chat_id: a.Chat.Id}
	bot.Message = make(map[string]string)
	user := db.getUser(a.User.Tg_id)
	if user.Plan > 0 {
		bot.Message["en"] = TgMessage("Your rate plan is " + strconv.Itoa(user.Plan) + " rub/month per device")
		bot.Reply()
		return
	}
	bot.Message["en"] = TgMessage("The rate plan is not set")
	bot.Reply()

}

func (a Command) Payment() {
	bot := Chatbot{Chat_id: a.Chat.Id}
	bot.Message = make(map[string]string)
	user := db.getUser(a.User.Tg_id)
	wgs := db.getVpns(user.Tg_id)
	if len(wgs) == 0 {
		bot.Message["en"] = TgMessage("You have no any VPN. Configure new one using /newvpn")
		bot.Reply()
		return
	}
	bot.Message["en"] = TgMessage("Your billing information:\n\n")
	for _, w := range wgs {
		var peers string
		if len(w.Peers) != 0 {
			peers += "\nPeers:\n"
			for i, peer := range w.Peers {
				peers += strconv.Itoa(i+1) + ". " + peer.Name + " (" + peer.IPAddress.String() + ")\n"
			}
			var amount string
			if w.Paidtill == 0 {
				amount = "0"
			} else {
				amount = strconv.Itoa(user.Plan * len(w.Peers))
			}

			peers += "Amount: " + amount + " rub/month"
		}

		if w.Paidtill > time.Now().Unix() && w.Paidtill != 0 {
			bot.Message["en"] += CodeBlock("Wg" + strconv.Itoa(w.Id) + " is paid till " + time.Unix(w.Paidtill, 0).Format("02.01.2006 15:04") + "." + peers + "\n")
		} else if w.Paidtill == 0 {
			bot.Message["en"] += CodeBlock("Wg" + strconv.Itoa(w.Id) + " is free of charge." + peers + "\n")
		} else {
			bot.Message["en"] += CodeBlock("Wg" + strconv.Itoa(w.Id) + " is expired since " + time.Unix(w.Paidtill, 0).Format("02.01.2006 15:04") + "." + peers + "\n")

		}
	}

	bot.Reply()

}

func (a Command) Invite() {
	bot := Chatbot{Chat_id: a.Chat.Id}
	bot.Message = make(map[string]string)
	bot.Message["en"] = TgMessage("By inviting someone, you vouch for his good intentions. If the invitee violates the laws protecting person and property, or the rules of our service, the consequences may affect the inviter.\n\nDo you want to continue?")
	bot.ReplyMarkup = ReplyKeyboardMarkup{
		Keyboard: [][]KeyboardButton{{
			KeyboardButton{
				Text: "Yes",
			},
			KeyboardButton{
				Text: "No",
			},
		}},

		ResizeKeyboard:  true,
		OneTimeKeyboard: true,
	}

	bot.Reply()
	ch := make(chan string)
	awaiting[a.Chat.Id] = ch
	response := <-ch
	delete(awaiting, a.Chat.Id)
	close(ch)
	bot.ReplyMarkup = ReplyKeyboardMarkup{RemoveKeyboard: true}

	if response != "Yes" {
		bot.Message["en"] = TgMessage("Thank you for being responsible")
		bot.Reply()
		return
	}

	rand.Seed(time.Now().UnixNano())
	num := rand.Intn(9000000) + 1000000
	now := time.Now().Unix()
	var inv Invitation
	inv.Code = strconv.Itoa(num)
	inv.User = a.User
	inv.Create = now
	inv.Expire = now + (60 * 60 * 24)
	err := db.addInvitation(inv)
	if err != nil {
		bot.Message["en"] = TgMessage("Sorry, but something went wrong. You can not create invitation code at the moment")
		bot.Reply()
		bot.Message["en"] = TgMessage("Invite() error on writting to db: " + err.Error())
		bot.Report()
		return
	}
	bot.Message["en"] = TgMessage("Here is the invitation code. Forward it to the friend, who will use it when run /register. The code will expire in 24h\n\n")
	bot.Message["en"] += CodeBlock(inv.Code)
	bot.Reply()

}

func (a Command) Adoptvpn() {

	bot := Chatbot{Chat_id: a.Chat.Id}
	bot.Message = make(map[string]string)
	bot.Message["en"] = TgMessage("Enter the number-name (for wg5 type 5) of the wireguard interface you'd like to adopt")
	bot.Reply()

	ch := make(chan string)
	awaiting[a.Chat.Id] = ch
	response := <-ch
	delete(awaiting, a.Chat.Id)
	close(ch)

	source_id, err := strconv.Atoi(response)
	if err != nil {
		bot.Message["en"] = TgMessage(err.Error())
		bot.Reply()
		return
	}
	var wg Wg
	wg.Id = source_id
	wg.Server = cfg.Server
	wg.Owner, err = db.getAdmin()
	if err != nil {
		bot.Message["en"] = TgMessage(err.Error())
		bot.Reply()
		return
	}
	wg.Get()

	users := db.getUsers()
	bot.Message["en"] = TgMessage("Select an owner: \n\n")
	var output string
	for i, u := range users {
		username := ""
		if len(u.Username) != 0 {
			username = " (@" + u.Username + ")\n"
		}
		output += strconv.Itoa(i+1) + ". " + u.Name + username
	}
	bot.Message["en"] += CodeBlock(output)
	bot.Reply()

	ch = make(chan string)
	awaiting[a.Chat.Id] = ch
	response = <-ch
	delete(awaiting, a.Chat.Id)
	close(ch)
	i, err := strconv.Atoi(response)
	if err != nil || i < 1 || i > len(users) {
		bot.Message["en"] = "Wrong user number. Run /adoptvpn again"
		bot.Reply()
		return
	}
	i = i - 1
	wg.Owner = users[i]
	_, err = db.addVpn(wg)
	if err != nil {
		bot.Message["en"] = TgMessage(err.Error())
		bot.Reply()
		return
	}
	err = wg.Update()
	if err != nil {
		bot.Message["en"] = TgMessage(err.Error())
		bot.Reply()
		return
	}
	bot.Message["en"] = TgMessage("Wg" + response + " was succesfully adopted to wgadmin. Owner: " + wg.Owner.Name)
	bot.Reply()

}
