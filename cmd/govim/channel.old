package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"
)

type chanMsg struct {
	i int
	m interface{}
}

func (c chanMsg) MarshalJSON() ([]byte, error) {
	v := [2]interface{}{c.i, c.m}
	return json.Marshal(v)
}

type handler struct {
	id   string // TODO should probably be a UUIDv4
	conn net.Conn
	log  io.Writer
}

func newHandler(log io.Writer, conn net.Conn) *handler {
	h := sha256.New()
	h.Write([]byte(time.Now().String()))

	return &handler{
		id:   fmt.Sprintf("%x", h.Sum(nil))[:10],
		conn: conn,
		log:  log,
	}
}

func (h *handler) String() string {
	return h.id
}

func (c *chanMsg) UnmarshalJSON(byts []byte) error {
	fmt.Printf("> UnmarshalJSON: %v\n", string(byts))
	var a []json.RawMessage

	if err := json.Unmarshal(byts, &a); err != nil {
		return err
	}

	if err := json.Unmarshal(a[0], &c.i); err != nil {
		return err
	}

	if err := json.Unmarshal(a[1], &c.m); err != nil {
		return err
	}

	return nil
}

func (h *handler) send(i interface{}) {
	h.sendMsg(chanMsg{
		m: i,
	})
}

func (h *handler) sendMsg(m chanMsg) {
	byts, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}

	if _, err := h.conn.Write(byts); err != nil {
		panic(err)
	}

	fmt.Fprintln(h.conn)
	h.debugf("sent response %v", string(byts))
}

func (h *handler) recvMsg() chanMsg {
	var m chanMsg
	dec := json.NewDecoder(h.conn)

	if err := dec.Decode(&m); err != nil {
		if err == io.EOF {
			panic(err)
		}
		fatalf("unexpected error in recvMsg decode: %v", err)
	}
	return m
}

func (h *handler) handle() {
	defer func() {
		err := recover()
		if err == io.EOF {
			// dropped/closed connection
			h.debugf("connection dropped")
		}

	}()
	h.debugf("handling new connection")
	h.send(h.id)
	for {
		h.debugf("waiting for question")

		req := h.recvMsg()

		resp := chanMsg{
			i: req.i,
			m: "hello workd!",
		}

		h.sendMsg(resp)
	}
}

func (h *handler) debugf(format string, args ...interface{}) {
	debugf(h.log, h.id+": "+format, args...)
}
