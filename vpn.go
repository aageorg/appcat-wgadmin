package main

import (
	"errors"
	"fmt"
	"net"
	"net/rpc/jsonrpc"
)

type Ipv4 [5]int

type Server struct {
	Name      string
	Country   string
	Provider  string
	IPAddress net.IP
	WgadminIP net.IP
}

type Peer struct {
	Name      string
	PublicKey string
	IPAddress net.IP
	Active    bool
}

type Wg struct {
	Id         int
	PublicKey  string
	PrivateKey string
	Network    *net.IPNet
	Server     Server
	IPAddress  net.IP
	IPNat      *net.IPNet
	Port       int
	Peers      []Peer
	Owner      User
	Paidtill   int64
	Paidsince  int64
}
type Keypair struct {
	Private string
	Public  string
}

type Response struct {
	Ok    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

func (w *Wg) Addpeer() error {
	return nil
}

func (w *Wg) New() error {
	client, err := jsonrpc.Dial("tcp", w.Server.WgadminIP.String()+":9000")
	if err != nil {
		return err
	}
	defer client.Close()
	err = client.Call("Wg.New", w.Id, &w)
	if err != nil {
		return err
	}
	fmt.Printf("\n\n\n%+v\n\n\n", w)
	return nil
}

func (w *Wg) Get() error {
	client, err := jsonrpc.Dial("tcp", w.Server.WgadminIP.String()+":9000")
	if err != nil {
		return err
	}
	defer client.Close()
	err = client.Call("Wg.Get", w.Id, &w)
	if err != nil {
		return err
	}
	return nil
}

func (w *Wg) Remove() error {
	client, err := jsonrpc.Dial("tcp", w.Server.WgadminIP.String()+":9000")
	if err != nil {
		return err
	}
	defer client.Close()
	err = client.Call("Wg.Remove", w.Id, &w)
	if err != nil {
		return err
	}
	fmt.Printf("\n\n\n%+v\n\n\n", w)
	return nil
}

func (w *Wg) Update() error {
	client, err := jsonrpc.Dial("tcp", w.Server.WgadminIP.String()+":9000")
	if err != nil {
		return err
	}
	var r Response
	defer client.Close()
	err = client.Call("Wg.Update", w, &r)
	if r.Ok != true {
		return errors.New(r.Error)
	}
	fmt.Printf("\n\n\n%+v\n\n\n", w)
	fmt.Printf("\n\n\n%+v\n\n\n", r)
	return nil
}

func (w *Wg) Genkeys() error {
	client, err := jsonrpc.Dial("tcp", w.Server.WgadminIP.String()+":9000")
	if err != nil {
		return err
	}
	defer client.Close()
	var keys Keypair
	err = client.Call("Wg.Keypair", nil, &keys)
	if err != nil {
		return err
	}

	fmt.Printf("\n\n\n%+v\n\n\n", keys)
	w.PublicKey = keys.Public
	w.PrivateKey = keys.Private
	return nil
}

func (p *Peer) Enable() {
	p.Active = true
}

func (p *Peer) Disable() {
	p.Active = false
}
