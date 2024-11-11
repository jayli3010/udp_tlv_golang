package main

import (
	"fmt"
	"net"
	"os"
	"syscall"

	// "time"
	"UDP_TLV_GO/pkg/tlv"
	"UDP_TLV_GO/pkg/udp"
	"encoding/json"
)

func main() {
	server, err := udp.NewUDPServer("localhost:8080")
	if err != nil {
		fmt.Println("Error creating UDP server:", err)
		if opErr, ok := err.(*net.OpError); ok {
			if syscallErr, ok := opErr.Err.(*os.SyscallError); ok {
				if syscallErr.Err == syscall.EADDRINUSE {
					fmt.Println("Port 8080 is already in use. Make sure no other program is using it.")
				}
			}
		}
		return
	}
	defer server.Close()

	// go udp.KeepAliveMonitor("localhost:8080", 5*time.Second)

	fmt.Println("UDP server listening on localhost:8080")

	server.Listen(func(data []byte, addr *net.UDPAddr) {
		msg, err := tlv.Decode[json.RawMessage](data)
		if err != nil {
			fmt.Println("Error decoding TLV message:", err)
			return
		}

		fmt.Printf("Received message from %v: Type=%d, Length=%d, Value=%s\n",
			addr, msg.Type, msg.Length, string(msg.Value))

		// Echo the message back
		response, err := tlv.Encode(msg)
		if err != nil {
			fmt.Println("Error encoding response:", err)
			return
		}
		err = server.Send(response, addr)
		if err != nil {
			fmt.Println("Error sending response:", err)
		}
	})
}
