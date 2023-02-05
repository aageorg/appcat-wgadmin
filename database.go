package main

//import "errors"
import (
	"boltdb/bolt"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"
)

var awaiting = make(map[int64]chan string)
var adminActivities = make(map[int64]chan string)

type Role struct {
	Admin    bool
	Customer bool
	Guest    bool
}

type User struct {
	Name             string `json:"first_name"`
	Username         string
	Phone            string
	Plan             int
	Tg_id            int64  `json:"id"`
	Tg_is_bot        bool   `json:"is_bot"`
	Tg_language_code string `json:"language_code"`
	Role             Role
	Chat             int64
}

type BotCommand struct {
	command     string
	description string
}

type Database struct {
	mu sync.Mutex
}

func (b *Database) init() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	d, err := bolt.Open(cfg.Database, 0644, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		fmt.Println(cfg.Database)
		return err
	}
	defer d.Close()

	err = d.Update(func(tx *bolt.Tx) error {
		_, err = tx.CreateBucketIfNotExists([]byte("config"))
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists([]byte("alerts"))
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists([]byte("users"))
		if err != nil {
			return err
		}
		_, err := tx.CreateBucketIfNotExists([]byte("wg"))
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists([]byte("servers"))
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists([]byte("wg_bindings"))
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists([]byte("admin"))
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists([]byte("invitations"))
		if err != nil {
			return err
		}

		//
		//	_, err = tx.CreateBucketIfNotExists([]byte("device_bindings"))
		//	if err != nil {
		//		return err
		//	}
		return nil
	})
	if err != nil {
		return err
	}
	return nil

}

func (b *Database) saveWebhookSecret(secret []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	d, err := bolt.Open(cfg.Database, 0644, &bolt.Options{Timeout: 3 * time.Second})
	if err != nil {
		return err
	}
	defer d.Close()
	err = d.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("config"))
		err = b.Put([]byte("webhook_secret"), secret)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (b *Database) getWebhookSecret() string {
	d, err := bolt.Open(cfg.Database, 0644, &bolt.Options{ReadOnly: true})
	if err != nil {
		return ""
	}
	defer d.Close()
	var secret []byte
	d.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("config"))
		secret = b.Get([]byte("webhook_secret"))
		return nil
	})
	if secret == nil {
		return ""
	}
	return string(secret)
}

func (b *Database) SetAlert(id int) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	d, err := bolt.Open(cfg.Database, 0644, &bolt.Options{Timeout: 3 * time.Second})
	if err != nil {
		return err
	}
	defer d.Close()
	t := strconv.FormatInt(time.Now().Unix(), 10)
	err = d.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("alerts"))
		err = b.Put([]byte(strconv.Itoa(id)), []byte(t))
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (b *Database) GetLastAlert(id int) (int64, error) {
	d, err := bolt.Open(cfg.Database, 0644, &bolt.Options{ReadOnly: true})
	if err != nil {
		return 0, err
	}
	defer d.Close()
	var value []byte
	d.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("alerts"))
		value = b.Get([]byte(strconv.Itoa(id)))
		return nil
	})
	if value == nil {
		return 0, nil
	}
	result, _ := strconv.ParseInt(string(value), 10, 64)
	return result, nil
}

func (b *Database) addVpn(wg Wg) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if wg.Id == 0 {
		wg.Id = b.newwgid()
	}
	d, err := bolt.Open(cfg.Database, 0644, &bolt.Options{Timeout: 3 * time.Second})
	if err != nil {
		return 0, err
	}
	defer d.Close()
	err = d.Update(func(tx *bolt.Tx) error {
		userid := []byte(strconv.FormatInt(wg.Owner.Tg_id, 10))
		wgid := []byte(strconv.Itoa(wg.Id))
		w, err := json.Marshal(wg)
		if err != nil {
			return err
		}
		wgs := tx.Bucket([]byte("wg"))

		_, err = wgs.CreateBucketIfNotExists(userid)
		if err != nil {
			return err
		}
		err = wgs.Bucket(userid).Put(wgid, w)
		if err != nil {
			return err
		}
		err = tx.Bucket([]byte("wg_bindings")).Put(wgid, userid)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return 0, err
	}

	return wg.Id, nil
}

func (b *Database) removeVpn(wg Wg) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	userid := []byte(strconv.FormatInt(wg.Owner.Tg_id, 10))
	wgid := []byte(strconv.Itoa(wg.Id))
	d, err := bolt.Open(cfg.Database, 0644, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return err
	}
	defer d.Close()
	var w []byte
	d.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("wg")).Bucket(userid)
		if b != nil {
			w = b.Get([]byte(wgid))
		}
		return nil
	})
	if w != nil {
		err = d.Update(func(tx *bolt.Tx) error {

			err = tx.Bucket([]byte("wg")).Bucket(userid).Delete(wgid)
			if err != nil {
				return err
			}
			err = tx.Bucket([]byte("wg_bindings")).Delete(wgid)
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			return err
		}
	} else {
		return errors.New("Wg" + string(wgid) + " does not exist")

	}
	return nil
}

func (b *Database) addUser(u User) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	d, err := bolt.Open(cfg.Database, 0644, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return err
	}
	defer d.Close()

	err = d.Update(func(tx *bolt.Tx) error {
		userid := []byte(strconv.FormatInt(u.Tg_id, 10))
		u, err := json.Marshal(u)
		if err != nil {
			return err
		}
		err = tx.Bucket([]byte("users")).Put(userid, u)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil

}

func (b *Database) addInvitation(i Invitation) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	d, err := bolt.Open(cfg.Database, 0644, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return err
	}
	defer d.Close()

	err = d.Update(func(tx *bolt.Tx) error {
		id := []byte(i.Code)
		i, err := json.Marshal(i)
		if err != nil {
			return err
		}
		err = tx.Bucket([]byte("invitations")).Put(id, i)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil

}

func (b *Database) dropInvitation(code string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	d, err := bolt.Open(cfg.Database, 0644, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return err
	}
	defer d.Close()

	err = d.Update(func(tx *bolt.Tx) error {
		id := []byte(code)
		err = tx.Bucket([]byte("invitations")).Delete(id)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil

}

func (b *Database) getInvitation(code string) error {
	d, err := bolt.Open(cfg.Database, 0644, &bolt.Options{ReadOnly: true})
	if err != nil {
		return err
	}
	defer d.Close()
	var i []byte
	d.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("invitations"))
		if b != nil {
			i = b.Get([]byte(code))
		}
		return nil
	})

	if i == nil {
		return errors.New("Invitation code not found")
	}

	fmt.Println("\n\nresult\n\n")
	fmt.Println(i)
	var inv Invitation
	err = json.Unmarshal(i, &inv)
	if err != nil {
		return err
	}
	if !b.user(inv.User.Tg_id) {
		//	b.dropInvitation(code)
		return errors.New("The inviter's account removed")
	}
	if time.Now().Unix() > inv.Expire {
		//	b.dropInvitation(code)
		return errors.New("The invitation code expired")
	}

	return nil

}

func (b *Database) checkUserInvitations(uid int64) (int, error) {
	d, err := bolt.Open(cfg.Database, 0644, &bolt.Options{ReadOnly: true})
	if err != nil {
		return -1, err
	}
	defer d.Close()
	counter := 0
	d.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("invitations"))
		if b != nil {
			b.ForEach(func(k, v []byte) error {
				var i Invitation
				err := json.Unmarshal(v, &i)
				if err != nil {
					return err
				}
				if i.User.Tg_id == uid {
					counter++
				}
				return nil
			})

		}
		return nil
	})

	return counter, nil

}

func (b *Database) setAdmin(u User) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	d, err := bolt.Open(cfg.Database, 0644, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return err
	}
	defer d.Close()

	err = d.Update(func(tx *bolt.Tx) error {
		userid := []byte(strconv.FormatInt(u.Tg_id, 10))
		u, err := json.Marshal(u)
		if err != nil {
			return err
		}
		err = tx.Bucket([]byte("admin")).Put(userid, u)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil

}
func (b *Database) getAdmin() (User, error) {
	d, err := bolt.Open(cfg.Database, 0644, &bolt.Options{ReadOnly: true})
	if err != nil {
		return User{}, err
	}
	defer d.Close()
	var u User
	d.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("admin"))
		if b != nil {
			b.ForEach(func(_, v []byte) error {
				err := json.Unmarshal(v, &u)
				if err != nil {
					return err
				}
				return nil
			})

		}
		return nil
	})

	if u.Tg_id == 0 || cfg.Tg.Admin != u.Username {
		return User{}, errors.New("Admin not found or admin username was changed")
	}
	return u, nil

}

func (b *Database) user(id int64) bool {
	d, err := bolt.Open(cfg.Database, 0644, &bolt.Options{ReadOnly: true})
	if err != nil {
		log.Fatal(err)
	}
	defer d.Close()

	var u []byte
	d.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("users"))
		u = b.Get([]byte(strconv.FormatInt(id, 10)))
		return nil
	})
	if u == nil {
		return false
	}
	return true
}

func (b *Database) newwgid() int {
	d, err := bolt.Open(cfg.Database, 0644, &bolt.Options{ReadOnly: true})
	if err != nil {
		log.Fatal(err)
	}
	defer d.Close()
	wgid := 2000
	d.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("wg_bindings"))
		if bucket != nil {
			c := bucket.Cursor()
			for k, _ := c.First(); k != nil; k, _ = c.Next() {
				i, err := strconv.Atoi(string(k))

				if err != nil {
					log.Fatal(err)
				}
				if i > wgid {
					wgid = i
				}
			}
		}
		return nil

	})

	return wgid + 1
}

func (b *Database) getChatByUser(id int64) int64 {
	d, err := bolt.Open(cfg.Database, 0644, &bolt.Options{ReadOnly: true})
	if err != nil {
		log.Fatal(err)
	}
	defer d.Close()

	var u []byte
	d.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("users"))
		u = b.Get([]byte(strconv.FormatInt(id, 10)))
		return nil
	})
	if u == nil {
		return 0
	} else {
		user := User{}
		err := json.Unmarshal(u, &user)
		if err != nil {
			log.Fatal(err)
		}
		return user.Chat
	}

}
func (b *Database) getVpns(id int64) []Wg {
	d, err := bolt.Open(cfg.Database, 0644, &bolt.Options{ReadOnly: true})
	if err != nil {
		log.Fatal(err)
	}
	defer d.Close()
	var wg []Wg
	d.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("wg")).Bucket([]byte(strconv.FormatInt(id, 10)))
		if b != nil {
			b.ForEach(func(_, v []byte) error {
				var w Wg
				err := json.Unmarshal(v, &w)
				if err != nil {
					return err
				}
				wg = append(wg, w)
				return nil
			})
		}
		return nil
	})
	return wg

}

func (b *Database) getAllVpns() []Wg {
	d, err := bolt.Open(cfg.Database, 0644, &bolt.Options{ReadOnly: true})
	if err != nil {
		log.Fatal(err)
	}
	defer d.Close()
	var wg []Wg
	d.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("wg"))
		c := b.Cursor()
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			wgs := b.Bucket(k)
			wgs.ForEach(func(k, v []byte) error {
				var w Wg
				err := json.Unmarshal(v, &w)
				if err != nil {
					return err
				}
				wg = append(wg, w)

				return nil
			})

		}
		return nil
	})
	return wg

}

func (b *Database) getVpn(wgid int) Wg {
	d, err := bolt.Open(cfg.Database, 0644, &bolt.Options{ReadOnly: true})
	if err != nil {
		log.Fatal(err)
	}
	defer d.Close()
	var wg Wg
	var u []byte
	d.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("wg_bindings"))
		u = b.Get([]byte(strconv.Itoa(wgid)))
		return nil
	})

	d.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("wg")).Bucket(u)
		w := b.Get([]byte(strconv.Itoa(wgid)))
		err := json.Unmarshal(w, &wg)
		if err != nil {
			return err
		}
		return nil
	})

	return wg

}

func (b *Database) getUsers() []User {
	d, err := bolt.Open(cfg.Database, 0644, &bolt.Options{ReadOnly: true})
	if err != nil {
		log.Fatal(err)
	}
	defer d.Close()
	var users []User

	d.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("users"))

		if b != nil {
			var u User
			b.ForEach(func(_, v []byte) error {
				err := json.Unmarshal(v, &u)
				if err != nil {
					return err
				}
				users = append(users, u)
				return nil
			})
		}
		return nil
	})
	return users

}

func (b *Database) getUser(id int64) User {
	d, err := bolt.Open(cfg.Database, 0644, &bolt.Options{ReadOnly: true})
	if err != nil {
		log.Fatal(err)
	}
	defer d.Close()
	var user User

	d.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("users"))
		if b != nil {
			u := b.Get([]byte(strconv.FormatInt(id, 10)))
			err := json.Unmarshal(u, &user)
			if err != nil {
				return err
			}

		}
		return nil
	})
	return user

}
