package server

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"syscall"
	"time"
)

type Server interface {
	Run()
}

type UDPServer struct {
	Port string
	// To process and send response to the client
	sensAddr   net.Addr
	packetConn net.PacketConn
	timer      *time.Timer
	buffer     []string
}

type udpPacket struct {
	net.Addr
	Len  int
	Body []byte
}

type TCPServer struct {
	Port     string
	sensConn net.Conn
	buffer   []string
	// server connection
}

type tcpPacket []byte

type tcpReader struct {
	net.Conn
}

func (tr *tcpReader) Read(p []byte) (n int, err error) {
	// Read bytes
	n, err = tr.Conn.Read(p)

	// Set timeout value after each message
	// Ignore the error check
	tr.Conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	// Check if client timed out
	var timeout net.Error
	if errors.As(err, &timeout) && timeout.Timeout() {
		err = io.EOF
	}

	return
}

func (tr *tcpReader) readConnection(packetChan chan tcpPacket, errChan chan error) {
	scanner := bufio.NewScanner(tr)
	for scanner.Scan() {
		packetChan <- scanner.Bytes()
	}
	if err := scanner.Err(); err != nil {
		errChan <- err
		return
	}
	// Explicit EOF since scanner.Err() filters it
	errChan <- io.EOF
}

func (srv *TCPServer) sendResponse(status int, req string) {
	// Use with valid connection
	if _, err := fmt.Fprintln(srv.sensConn, status); err != nil {
		log.Println("Error sending response")
	}
	log.Println("Response status", status, "sent for request:", req)
}

func (srv *TCPServer) processPacket(packet tcpPacket) {
	req := string(packet)
	var reqType string
	var timestamp, val int

	parsed, err := fmt.Sscanf(req, "%s %d:%d", &reqType, &timestamp, &val)
	if parsed < 3 || err != nil || reqType != "TEMP" {
		srv.sendResponse(400, req)
		return
	}

	// Critical
	srv.buffer = append(srv.buffer, req[5:])
	if len(srv.buffer) == 10 {
		// TODO: Send the buffer to the server
		log.Println(srv.buffer)
		// Clear the slice, keep the allocated memory
		srv.buffer = srv.buffer[:0]
	}

	srv.sendResponse(200, req)
}

func (srv *TCPServer) Run() {
	// Broadcast port
	ln, err := net.Listen("tcp", srv.Port)
	if err != nil {
		log.Fatal("Failed to create TCP listener:", err)
	}

	// Close the listener and connection if not closed
	defer func() {
		ln.Close()
		if srv.sensConn != nil {
			srv.sensConn.Close()
		}
	}()

	packetChan := make(chan tcpPacket)
	errChan := make(chan error)

	// Accept connection attempts
	// Note: TCP Server accepts a single active connection
	for srv.sensConn == nil {
		srv.sensConn, err = ln.Accept()
		if err != nil {
			log.Println("Failed to connect to the client:", err)
		}

		tr := &tcpReader{Conn: srv.sensConn}

		go tr.readConnection(packetChan, errChan)

		for srv.sensConn != nil {
			select {
			// Received a packet successfully
			case packet := <-packetChan:
				// Should be faster than 1s
				log.Println("Received a TCP packet:", string(packet))
				srv.processPacket(packet)
			case err := <-errChan:
				// TODO: Notify the server that the TEMP Sensor down
				// Check if the client timed out or connection resetted
				if errors.Is(err, io.EOF) || errors.Is(err, syscall.ECONNRESET) {
					log.Println("Connection reset")
					// Closing the connection
					if err = srv.sensConn.Close(); err != nil {
						log.Fatal(err)
					}
					// Set the connection to nil so can server accept a new connection
					srv.sensConn = nil
				} else {
					// Possible unrecoverable error
					// TODO: might send status 500
					log.Println(err)
					// Shuts down the business
					return
				}
			}
		}
	}
}

func (srv *UDPServer) receivePackets(packetChan chan *udpPacket, errChan chan error) {
	// Keep accepting packets as long as there is a client (sensor)
	for srv.packetConn != nil {
		// 128 byte long packets should be enough
		buff := make([]byte, 512)
		n, addr, err := srv.packetConn.ReadFrom(buff)
		if n == 0 || err != nil {
			errChan <- err
			// To leave the desicion to the caller
			continue
		}
		// Len is to get rid of NULLs
		// I'm not exactly sure about n-2 is the best way to get rid of LF
		packetChan <- &udpPacket{Addr: addr, Len: n - 2, Body: buff}
	}
}

func (srv *UDPServer) processPacket(packet *udpPacket) {
	req := string(packet.Body[:packet.Len])
	// Set up server or continue
	if srv.sensAddr == nil || req == "REQUEST TRANSMISSION" {
		// I don't know
		// srv.timer.Reset(0)

		// Handshake check
		if req != "REQUEST TRANSMISSION" {
			log.Printf("Expected handshake, got: %s. Sending response status 400", req)
			_, err := srv.packetConn.WriteTo([]byte("400\r\n"), packet.Addr)
			if err != nil {
				log.Println("Failed to send response status 400")
			}
			return
		}

		// Successful handshake
		log.Println("Handshake successful, sending response status 200")
		_, err := srv.packetConn.WriteTo([]byte("200\r\n"), packet.Addr)
		if err != nil {
			log.Println("Failed to send response status 200")
		}

		// Assign sensor address to server
		srv.sensAddr = packet.Addr
		// Timer starts here
		srv.timer.Reset(7 * time.Second)
		return
	}

	// Request decode
	switch req[:5] {
	case "ALIVE":
		log.Println("Resetting timer")
		srv.timer.Reset(7 * time.Second)
	case "HUMID":
		srv.buffer = append(srv.buffer, req[5:])
		if len(srv.buffer) == 10 {
			// TODO: Send the buffer to the server
			log.Println(srv.buffer)
			// Clear the slice, keep the allocated memory
			srv.buffer = srv.buffer[:0]
		}
	}
}

func (srv *UDPServer) Run() {
	var err error
	// Listen UDP packets
	srv.packetConn, err = net.ListenPacket("udp", srv.Port)
	if err != nil {
		log.Fatal("Failed to create UDP listener")
	}

	// Close UDP server
	defer func() {
		srv.packetConn.Close()
		// To terminate `receivePackets` goroutine
		srv.packetConn = nil
	}()

	// Client is nil when there is no sensor
	srv.sensAddr = nil

	packetChan := make(chan *udpPacket)
	errChan := make(chan error)

	go srv.receivePackets(packetChan, errChan)

	// I'm not very happy about this
	srv.timer = time.NewTimer(0)
	defer srv.timer.Stop()

	for {
		select {
		// Received a request
		case packet := <-packetChan:
			log.Println("Received a UDP packet:", string(packet.Body[:packet.Len]))
			srv.processPacket(packet)
		// An error occured during processing request
		case err := <-errChan:
			log.Println(err)
			return
		// Possible (almost certain) time out
		case <-srv.timer.C:
			// I don't know if this check is necessary
			if srv.sensAddr != nil {
				log.Println("Humidity sensor down")
				// Reset for the next inital set up
				srv.sensAddr = nil
			}
		}
	}
}
