package cmd

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"trp/pack"
	"trp/proxy"
)

var (
	listenAddr string
	listenPort int
)

func initServer() {
	flag.StringVar(&listenAddr, "h", "0.0.0.0", "listen address")
	flag.IntVar(&listenPort, "p", 7777, "listen port")
}

func StartServer() {
	initServer()
	ser, err := net.Listen("tcp", fmt.Sprintf("%s:%d", listenAddr, listenPort))
	if err != nil {
		fmt.Println("[Server] Error listening: ", err)
		return
	}
	defer func() {
		_ = ser.Close()
	}()

	fmt.Println("Server started on 0.0.0.0:7777")

	for {
		client, err := ser.Accept()
		if err != nil {
			fmt.Println("Error accepting connection: ", err)
			continue
		}
		fmt.Println("Client connected: ", client.RemoteAddr().String())

		go handleClient(client)
	}
}

func handleClient(client net.Conn) {
	// for every client, get settings and listen
	prx := proxy.NewProxy(client)

	for {
		pkg, err := pack.ReadPackage(client)

		if err != nil {
			var netErr net.Error
			if err == io.EOF || errors.As(err, &netErr) {
				fmt.Println("Client disconnected: ", client.RemoteAddr().String())
				// close the proxy
				prx.Close()
				return
			}

			fmt.Println("Error reading package: ", err)
			continue
		}

		// we get a package
		handleClientPackage(prx, pkg)
	}
}

func handleClientPackage(prx *proxy.Proxy, pkg *pack.Package) {
	if pkg.Type == pack.TYPE_PORTS {
		ports := strings.Split(string(pkg.Data), ":")
		// Client ensures that the ports sent are legitimate, so we ignore the error
		portS, _ := strconv.Atoi(ports[0])
		portC, _ := strconv.Atoi(ports[1])
		prx.ServerStart(portS, portC)
	} else if pkg.Type == pack.TYPE_HEARTBEAT {
		fmt.Println("Client heart beat: ")
	} else if pkg.Type == pack.TYPE_VISITOR {
		visitorId, _ := strconv.Atoi(string(pkg.Data))
		// VISITOR package must follow a DATA package
		p, err := pack.ReadPackage(prx.Client)
		if err != nil {
			fmt.Println("Error reading package: ", err)
			return
		}
		if p.Type == pack.TYPE_VISITOR_CLOSE {
			prx.CloseVisitor(visitorId)
			return
		} else if p.Type != pack.TYPE_DATA {
			fmt.Println("Error: VISITOR package must follow a DATA package")
			return
		}
		// forward response to visitor
		_, _ = prx.Visitors[visitorId].Conn.Write(p.Data)
	} else {
		// ignore unknown data
		fmt.Println("Unknown package type: ", pkg.Type)
	}
	// TODO if no heartbeat for a while, close the connection
	// so we need refresh timer for client, and any package is also a heartbeat
}
