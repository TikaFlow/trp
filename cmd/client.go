package cmd

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"
	"trp/pack"
	"trp/proxy"
)

var (
	serverHost string
	serverPort int
	remotePort int
	localHost  string
	localPort  int
)

func initClient() {
	flag.StringVar(&serverHost, "h", "127.0.0.1", "server host")
	flag.IntVar(&serverPort, "r", 7777, "server port")
	flag.IntVar(&remotePort, "p", 7080, "remote port")
	flag.StringVar(&localHost, "lh", "127.0.0.1", "local host")
	flag.IntVar(&localPort, "l", 9090, "local port")

	flag.Parse()
}

func StartClient() {
	initClient()
	client, err := net.Dial("tcp", fmt.Sprintf("%s:%d", serverHost, serverPort))
	if err != nil {
		fmt.Println("Error connecting: ", err)
		return
	}
	defer func() {
		_ = client.Close()
	}()

	fmt.Println("Connected to server: ", client.RemoteAddr().String())
	_ = pack.SendData(client, []byte(fmt.Sprintf("%d:%d", remotePort, localPort)), pack.TYPE_PORTS)
	// heartbeat
	go func() {
		for {
			_ = pack.SendData(client, []byte{}, pack.TYPE_HEARTBEAT)
			time.Sleep(pack.HEARTBEAT_INTERVAL * time.Second)
		}
	}()

	prx := proxy.NewProxy(client)
	for {
		pkg, err := pack.ReadPackage(client)

		if err != nil {
			var netErr net.Error
			if err == io.EOF || errors.As(err, &netErr) {
				fmt.Println("Client disconnected: ", client.RemoteAddr().String())
				prx.Close()
				return
			}

			fmt.Println("Error reading package: ", err)
			continue
		}

		// we get a package
		handleServerPackage(prx, pkg)
	}
}

func handleServerPackage(prx *proxy.Proxy, pkg *pack.Package) {
	if pkg.Type == pack.TYPE_VISITOR {
		visitorId, _ := strconv.Atoi(string(pkg.Data))
		// VISITOR package must follow a package
		p, e := pack.ReadPackage(prx.Client)
		if e != nil {
			fmt.Println("Error reading package: ", e)
			return
		}

		if p.Type == pack.TYPE_VISITOR_NEW {
			conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", localHost, localPort))
			if err != nil {
				fmt.Println("Error connecting to visitor: ", err)
			}
			prx.Visitors[visitorId] = proxy.NewVisitor(conn)
			go prx.HandleVisitor(visitorId)
			return
		} else if p.Type == pack.TYPE_VISITOR_CLOSE {
			prx.CloseVisitor(visitorId)
			return
		} else if p.Type != pack.TYPE_DATA {
			fmt.Println("Unknown package type: ", p.Type)
			return
		}
		// forward request to visitor
		_, _ = prx.Visitors[visitorId].Conn.Write(p.Data)
	} else {
		// ignore unknown data
		fmt.Println("Unknown package type: ", pkg.Type)
		return
	}
}
