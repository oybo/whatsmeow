package whatsmeow

import (
	"context"
	"testing"

	"go.mau.fi/whatsmeow/socket"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// Both fork handshake paths finish here. Each connection must get its own
// upstream handler queue, including an XX retry after a failed IK handshake.
func TestFinishHandshakeCreatesPerConnectionQueue(t *testing.T) {
	cli := &Client{Log: waLog.Noop}
	firstSocket := socket.NewFrameSocket(waLog.Noop, nil, "")
	firstHandshake := socket.NewNoiseHandshake()
	firstHandshake.Start(socket.NoiseXXStartPattern, firstSocket.Header)
	firstQueue, err := cli.finishHandshake(context.Background(), firstHandshake, firstSocket)
	if err != nil {
		t.Fatal(err)
	}
	firstNoiseSocket := cli.socket
	t.Cleanup(func() { firstNoiseSocket.Stop(false, false) })
	if firstQueue == nil || cap(firstQueue) != handlerQueueSize || firstNoiseSocket == nil {
		t.Fatal("handshake did not initialize the socket and handler queue")
	}

	secondSocket := socket.NewFrameSocket(waLog.Noop, nil, "")
	secondHandshake := socket.NewNoiseHandshake()
	secondHandshake.Start(socket.NoiseIKStartPattern, secondSocket.Header)
	secondQueue, err := cli.finishHandshake(context.Background(), secondHandshake, secondSocket)
	if err != nil {
		t.Fatal(err)
	}
	secondNoiseSocket := cli.socket
	t.Cleanup(func() { secondNoiseSocket.Stop(false, false) })
	if secondQueue == nil || firstQueue == secondQueue || firstNoiseSocket == secondNoiseSocket {
		t.Fatal("reconnected socket reused the previous connection's queue or socket")
	}
}
