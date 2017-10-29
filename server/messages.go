package server

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"os"

	"github.com/cashshuffle/cashshuffle/message"
	"github.com/golang/protobuf/proto"
)

var (
	// breakBytes are the bytes that delimit each protobuf message
	// This represents the character ⏎
	breakBytes = []byte{226, 143, 142}
)

// startSignedChan starts a loop reading messages.
func startSignedChan(c chan *signedConn) {
	for {
		sc := <-c
		err := sc.processReceivedMessage()
		if err != nil {
			sc.conn.Close()
			fmt.Fprintf(os.Stderr, "[Error] %s\n", err.Error())
		}
	}
}

// processReceivedMessage reads the message and processes it.
func (sc *signedConn) processReceivedMessage() error {
	// If we are not tracking the connection yet, the user must be
	// registering with the server.
	if sc.tracker.getTrackerData(sc.conn) == nil {
		if err := sc.registerClient(); err != nil {
			return err
		}

		return nil
	}

	if err := sc.verifyMessage(); err != nil {
		return err
	}

	if err := sc.broadcastMessage(); err != nil {
		return err
	}

	return nil
}

// processMessages reads messages from the connection and begins processing.
func processMessages(conn net.Conn, c chan *signedConn, t *tracker) {
	scanner := bufio.NewScanner(conn)
	scanner.Split(bufio.ScanRunes)

	var b bytes.Buffer

	for {
		for scanner.Scan() {
			scanBytes := scanner.Bytes()

			if breakScan(scanBytes) {
				break
			}

			b.Write(scanBytes)
		}

		if err := scanner.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "[Error] %s\n", err.Error())
			break
		}

		// We should not receive empty messages.
		if b.String() == "" {
			break
		}

		if err := sendToSignedChan(&b, conn, c, t); err != nil {
			fmt.Fprintf(os.Stderr, "[Error] %s\n", err.Error())
			break
		}
	}
}

// sendToSignedChannel takes a byte buffer containing a protobuf message,
// converts it to message.Signed and sends it over signedChan.
func sendToSignedChan(b *bytes.Buffer, conn net.Conn, c chan *signedConn, t *tracker) error {
	defer b.Reset()

	pdata := new(message.Signed)

	err := proto.Unmarshal(b.Bytes(), pdata)
	if err != nil {
		return err
	}

	data := &signedConn{
		message: pdata,
		conn:    conn,
		tracker: t,
	}

	c <- data

	return nil
}

// breakScan checks if a byte sequence is the break point on the scanner.
func breakScan(bs []byte) bool {
	if len(bs) == 3 {
		for i := range bs {
			if bs[i] != breakBytes[i] {
				return false
			}
		}

		return true
	}

	return false
}
