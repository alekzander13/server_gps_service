package main

import (
	"net"
	"time"
)

type conn struct {
	net.Conn

	IdleTimeout   time.Duration
	MaxReadBuffer int64
}

func (c *conn) Close() (err error) {
	err = c.Conn.Close()
	return
}

func (c *conn) UpdateDeadline() {
	idleDeadline := time.Now().Add(c.IdleTimeout)
	c.Conn.SetDeadline(idleDeadline)
}

func (c *conn) Send(b []byte) error {
	_, err := c.Conn.Write(b)
	if err == nil {
		c.UpdateDeadline()
	}
	return err
}
