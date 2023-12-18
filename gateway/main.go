package main

import (
	"emregrkan.network.io/gateway/internal/server"
)

func main() {
	tcpSrv := &server.TCPServer{
		Port: ":8367",
	}
	udpSrv := &server.UDPServer{
		Port: ":4638",
	}
	go udpSrv.Run()
	tcpSrv.Run()
}
