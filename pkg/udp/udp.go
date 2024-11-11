package udp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"golang.org/x/net/ipv4"
)

type UDPServer struct {
	conn        *net.UDPConn
	reassembler *Reassembler
}

type Fragment struct {
	SourcePort      uint16
	DestinationPort uint16
	SequenceNumber  uint32
	TotalFragments  uint16
	FragmentNumber  uint16
	Flags           uint16 // 可选，用于自定义标志
	Data            []byte
	Timestamp       time.Time
}

type Reassembler struct {
	fragments map[uint32]map[uint16]Fragment
	mutex     sync.Mutex
	timeout   time.Duration
}

func NewReassembler(timeout time.Duration) *Reassembler {
	return &Reassembler{
		fragments: make(map[uint32]map[uint16]Fragment),
		timeout:   timeout,
	}
}

func (r *Reassembler) AddFragment(data []byte) ([]byte, error) {
	if len(data) < 20 { // 8 (UDP header) + 12 (custom header)
		return nil, errors.New("fragment too short")
	}

	frag := Fragment{
		SourcePort:      binary.BigEndian.Uint16(data[0:2]),
		DestinationPort: binary.BigEndian.Uint16(data[2:4]),
		// 跳过 Length 和 Checksum
		SequenceNumber:  binary.BigEndian.Uint32(data[8:12]),
		TotalFragments:  binary.BigEndian.Uint16(data[12:14]),
		FragmentNumber:  binary.BigEndian.Uint16(data[14:16]),
		Flags:           binary.BigEndian.Uint16(data[16:18]),
		Data:            data[20:],
		Timestamp:       time.Now(),
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	key := frag.SequenceNumber
	if _, exists := r.fragments[key]; !exists {
		r.fragments[key] = make(map[uint16]Fragment)
	}
	r.fragments[key][frag.FragmentNumber] = frag

	return r.tryReassemble(key)
}

func (r *Reassembler) tryReassemble(seqNum uint32) ([]byte, error) {
	frags := r.fragments[seqNum]
	if len(frags) == 0 {
		return nil, nil // No fragments yet
	}

	var totalFrags uint16
	for _, frag := range frags {
		totalFrags = frag.TotalFragments
		break
	}

	if uint16(len(frags)) < totalFrags {
		return nil, nil // Not all fragments received yet
	}

	// Check for missing fragments
	for i := uint16(0); i < totalFrags; i++ {
		if _, exists := frags[i]; !exists {
			return nil, fmt.Errorf("missing fragment %d", i)
		}
	}

	// Reassemble
	reassembled := make([]byte, 0)
	for i := uint16(0); i < totalFrags; i++ {
		reassembled = append(reassembled, frags[i].Data...)
	}

	// Clean up
	delete(r.fragments, seqNum)

	return reassembled, nil
}

func (r *Reassembler) CleanupOldFragments() {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	now := time.Now()
	for seqNum, frags := range r.fragments {
		for fragNum, frag := range frags {
			if now.Sub(frag.Timestamp) > r.timeout {
				delete(frags, fragNum)
			}
		}
		if len(frags) == 0 {
			delete(r.fragments, seqNum)
		}
	}
}

func NewUDPServer(address string) (*UDPServer, error) {
	addr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}

	return &UDPServer{
		conn:        conn,
		reassembler: NewReassembler(30 * time.Second), // 30 seconds timeout
	}, nil
}

func (s *UDPServer) Listen(handler func([]byte, *net.UDPAddr)) {
	buffer := make([]byte, 1024)
	go s.cleanupRoutine()

	for {
		n, remoteAddr, err := s.conn.ReadFromUDP(buffer)
		if err != nil {
			log.Printf("Error reading from UDP: %v", err)
			continue
		}

		go s.handlePacket(buffer[:n], remoteAddr, handler)
	}
}

func (s *UDPServer) handlePacket(data []byte, remoteAddr *net.UDPAddr, handler func([]byte, *net.UDPAddr)) {
	// 验证校验和
	if !ValidateUDPChecksum(remoteAddr.IP, s.conn.LocalAddr().(*net.UDPAddr).IP, data) {
		log.Println("Invalid UDP checksum, discarding packet")
		return
	}

	reassembled, err := s.reassembler.AddFragment(data)
	if err != nil {
		log.Printf("Error reassembling fragment: %v", err)
		return
	}

	if reassembled != nil {
		handler(reassembled, remoteAddr)
	}
}

func (s *UDPServer) cleanupRoutine() {
	ticker := time.NewTicker(5 * time.Second)
	for range ticker.C {
		s.reassembler.CleanupOldFragments()
	}
}

func (s *UDPServer) Send(data []byte, addr *net.UDPAddr) error {
	// 计算并设置校验和
	checksum := CalculateUDPChecksum(s.conn.LocalAddr().(*net.UDPAddr).IP, addr.IP, data)
	binary.BigEndian.PutUint16(data[6:8], checksum)

	_, err := s.conn.WriteToUDP(data, addr)
	return err
}

func (s *UDPServer) Close() {
	s.conn.Close()
}

func KeepAliveMonitor(address string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		conn, err := net.Dial("udp", address)
		if err != nil {
			log.Printf("Keep-alive check failed: %v", err)
			continue
		}
		conn.Close()
		log.Println("Keep-alive check successful")
	}
}

func CalculateUDPChecksum(srcIP, dstIP net.IP, udpPayload []byte) uint16 {
	pseudoHeader := make([]byte, 12)
	copy(pseudoHeader[0:4], srcIP.To4())
	copy(pseudoHeader[4:8], dstIP.To4())
	pseudoHeader[8] = 0
	pseudoHeader[9] = 17 // UDP protocol number
	binary.BigEndian.PutUint16(pseudoHeader[10:12], uint16(len(udpPayload)))

	csum := ipv4.Checksum(udpPayload, 0)
	csum = ipv4.Checksum(pseudoHeader, csum)

	return ^csum
}

func ValidateUDPChecksum(srcIP, dstIP net.IP, udpPacket []byte) bool {
	originalChecksum := binary.BigEndian.Uint16(udpPacket[6:8])
	binary.BigEndian.PutUint16(udpPacket[6:8], 0)

	calculatedChecksum := CalculateUDPChecksum(srcIP, dstIP, udpPacket)

	binary.BigEndian.PutUint16(udpPacket[6:8], originalChecksum)

	return calculatedChecksum == originalChecksum
}
