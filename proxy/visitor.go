package proxy

import "net"

type Visitor struct {
	Conn    net.Conn
	Offline bool
}

func NewVisitor(conn net.Conn) *Visitor {
	return &Visitor{
		Conn:    conn,
		Offline: false,
	}
}

func (this *Visitor) Close() {
	if this.Offline {
		return
	}

	_ = this.Conn.Close()
	this.Offline = true
}
