package proxy

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"trp/pack"
)

type Proxy struct {
	// which client bind this proxy
	Client net.Conn
	// listener
	Listener net.Listener
	// listener port
	ListenerPort int
	// visitors
	Visitors    map[int]*Visitor
	VisitorLock sync.RWMutex
}

func NewProxy(client net.Conn) *Proxy {
	return &Proxy{
		Client:   client,
		Visitors: make(map[int]*Visitor),
	}
}

func (this *Proxy) Close() {
	_ = this.Client.Close()

	// close all connections
	this.VisitorLock.Lock()
	defer this.VisitorLock.Unlock()
	for k, v := range this.Visitors {
		_ = v.Conn.Close()
		delete(this.Visitors, k)
	}

	// stop listening
	if this.Listener != nil {
		_ = this.Listener.Close()
	}
}

func (this *Proxy) ServerStart(portS, portC int) {
	fmt.Println("Mapping port:", portS, "to", this.Client.RemoteAddr().String(), portC)

	remote, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", portS))
	if err != nil {
		fmt.Println("Error listening port:", portS, ", ", err)
		return
	}
	this.Listener = remote
	this.ListenerPort = portS

	go func() {
		var connId int
		for {
			conn, e := remote.Accept()
			if e != nil {
				var netErr net.Error
				if e == io.EOF || errors.As(e, &netErr) {
					fmt.Println("Proxy closed:", this.ListenerPort)
					return
				}

				fmt.Println("Error accepting connection: ", err)
				continue
			}

			this.VisitorLock.Lock()
			this.Visitors[connId] = NewVisitor(conn)
			this.VisitorLock.Unlock()
			// notify client new visitor
			_ = pack.SendPickyData(this.Client, []byte{}, connId, pack.TYPE_VISITOR_NEW)
			go this.HandleVisitor(connId)
			connId++
		}
	}()
}

func (this *Proxy) HandleVisitor(connId int) {
	visitor := this.Visitors[connId]

	for {
		buf := make([]byte, pack.PACK_MAX_SIZE)
		cnt, err := visitor.Conn.Read(buf)
		if err != nil {
			var netErr net.Error
			if err == io.EOF || errors.As(err, &netErr) {
				fmt.Println("Visitor disconnected: ", visitor.Conn.RemoteAddr().String())
				// close visitor
				this.CloseVisitor(connId)
				// notify opposite to close visitor
				_ = pack.SendPickyData(this.Client, []byte{}, connId, pack.TYPE_VISITOR_CLOSE)
				return
			}

			fmt.Println("Error reading from visitor: ", err)
			continue
		}

		// forward request/response
		_ = pack.SendPickyData(this.Client, buf[:cnt], connId, pack.TYPE_DATA)
	}
}

func (this *Proxy) CloseVisitor(connId int) {
	// close conn
	if visitor, ok := this.Visitors[connId]; ok {
		_ = visitor.Conn.Close()
	}

	// delete from map
	this.VisitorLock.Lock()
	defer this.VisitorLock.Unlock()
	delete(this.Visitors, connId)
}
