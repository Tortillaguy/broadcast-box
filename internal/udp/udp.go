package udp

import (
	"fmt"
	"net"
	"sync"
)


var payload = ""
var payloadLock sync.Mutex

func UpdatePayload(data string){
	payloadLock.Lock()
	defer payloadLock.Unlock()
	payload = data
}

func GetPayload() string {
	payloadLock.Lock()
	defer payloadLock.Unlock()
	return payload
}

func Start() {
	// Define the port to listen on
	port := 8001

	// Resolve UDP address
	address, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", port))
	if err != nil {
		fmt.Println("Error resolving address:", err)
		return
	}

	// Create UDP connection
	conn, err := net.ListenUDP("udp", address)
	if err != nil {
		fmt.Println("Error creating UDP connection:", err)
		return
	}
	defer conn.Close()

	// Create a buffer to hold incoming data
	buffer := make([]byte, 1024)

	fmt.Printf("Listening for UDP packets on port %d...\n", port)

	for {
		// Read data into the buffer
		n, addr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Println("Error reading from UDP connection:", err)
			return
		}

		// Print out the received data
		fmt.Printf("Received %d bytes from %s: %s\n", n, addr, string(buffer[:n]))
		UpdatePayload(string(buffer[:n]))
	}
}
