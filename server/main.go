package main

import (
	"emregrkan.network.io/server/internal/tcp"
)

func main() {
	conn := tcp.NewConnection(":8080")
	conn.Establish()
}
