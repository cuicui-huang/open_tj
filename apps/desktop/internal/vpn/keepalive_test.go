package vpn

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"
)

func TestSetupKeepaliveSendsHeartbeatsAndStops(t *testing.T) {
	writer, reader := net.Pipe()
	defer reader.Close()
	defer writer.Close()

	stop := startSetupKeepalive(context.Background(), writer, 2*time.Second)
	defer stop()

	_ = reader.SetReadDeadline(time.Now().Add(2 * time.Second))
	packet := make([]byte, 20)
	if _, err := io.ReadFull(reader, packet); err != nil {
		t.Fatalf("no heartbeat: %v", err)
	}
	if packet[9] != 255 || binary.BigEndian.Uint16(packet[12:]) != 1 {
		t.Fatalf("not a heartbeat: %v", packet)
	}

	stop()
	stop() // idempotent
}
