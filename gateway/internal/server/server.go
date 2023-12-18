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
}

type udpReq struct {
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

func (tr *tcpReader) readConnection(reqChan chan []byte, errChan chan error) {
	scanner := bufio.NewScanner(tr)
	for scanner.Scan() {
		reqChan <- scanner.Bytes()
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
	if _, err := fmt.Fprint(srv.sensConn, status); err != nil {
		log.Println("Error sending response")
	}
	log.Println("Response status", status, "sent for request:", req)
}

func (srv *TCPServer) processRequest(req []byte) {
	reqString := string(req)
	var reqType string
	var timestamp, val int

	parsed, err := fmt.Sscanf(reqString, "%s %d:%d", &reqType, &timestamp, &val)
	if parsed < 3 || err != nil || reqType != "TEMP" {
		go srv.sendResponse(400, reqString)
		return
	}

	go srv.sendResponse(200, reqString)

	log.Println("Request:", reqString)

	// Critical
	srv.buffer = append(srv.buffer, string(req)[5:])
	if len(srv.buffer) == 10 {
		// TODO: Send the buffer to the server
		log.Println(srv.buffer)
		// Clear the slice, keep the allocated memory
		srv.buffer = srv.buffer[:0]
	}
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

	// Accept connection attempts
	// Note: TCP Server accepts a single active connection
	for srv.sensConn == nil {
		srv.sensConn, err = ln.Accept()
		if err != nil {
			log.Println("Failed to connect to the client:", err)
		}

		tr := &tcpReader{Conn: srv.sensConn}

		reqChan := make(chan []byte)
		errChan := make(chan error)

		go tr.readConnection(reqChan, errChan)

		for srv.sensConn != nil {
			select {
			// Received request successfully
			case req := <-reqChan:
				srv.processRequest(req)
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

func (srv *UDPServer) listenRequests(reqChan chan *udpReq, errChan chan error) {
	// Keep accepting packets as long as there is a client (sensor)
	for srv.packetConn != nil {
		// 128 byte long packets should be enough
		buff := make([]byte, 128)
		n, addr, err := srv.packetConn.ReadFrom(buff)
		if n == 0 || err != nil {
			errChan <- err
			// To leave the desicion to the caller
			continue
		}
		// Len is to get rid of NULLs
		// I'm not exactly sure about n-1 is the best way to get rid of LF
		reqChan <- &udpReq{Addr: addr, Len: n - 1, Body: buff}
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
		// To terminate `listenRequests` goroutine
		srv.packetConn = nil
	}()

	// Client is nil when there is no sensor
	srv.sensAddr = nil

	reqChan := make(chan *udpReq)
	errChan := make(chan error)

	go srv.listenRequests(reqChan, errChan)

	// I'm not very happy about this
	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		// Received a request
		case req := <-reqChan:
			// Initial request set up
			if srv.sensAddr == nil {
				srv.sensAddr = req.Addr
				// Timer starts here
				timer.Reset(7 * time.Second)
			}
			reqStr := string(req.Body[:req.Len])
			if reqStr == "ALIVE" {
				// `ALIVE` message received reset the timer
				timer.Reset(7 * time.Second)
			}
			log.Println(reqStr)
		// An error occured during processing request
		case err := <-errChan:
			log.Println(err)
			return
		// Possible (almost certain) time out
		case <-timer.C:
			// I don't know if this check is necessary
			if srv.sensAddr != nil {
				log.Println("Humidty sensor down")
				// Reset for the next inital set up
				srv.sensAddr = nil
			}
		}
	}
}
