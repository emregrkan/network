package main

import "emregrkan.network.io/gateway/internal/server"

func main() {
	srv := &server.TCPServer{
		Port: ":8367",
	}
	srv.Run()
}
