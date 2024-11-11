# TLV over UDP Project

This project implements a simple server that uses Type-Length-Value (TLV) encoding over UDP communication.

## Building and Running

To build and run the project:

1. Ensure you have Go installed on your system.
2. Clone this repository.
3. Navigate to the project directory.
4. Run `go run cmd/main.go` to start the server.

The server will listen on localhost:8080 for incoming UDP packets, decode them as TLV messages, and echo them back to the sender.

## Project Structure

- `cmd/main.go`: Main application entry point
- `pkg/tlv/tlv.go`: TLV encoding and decoding functions
- `pkg/udp/udp.go`: UDP server implementation and keep-alive monitor

## TODO

- Implement error handling and logging
- Create a command-line interface for easy interaction
- Implement sequence checking
- Optimize performance
- Add more detailed documentation
# udp_tlv_golang
