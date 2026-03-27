package whatsmeow

import (
	"context"
	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

func (cli *Client) sendIQXmppPing(query *infoQuery) (<-chan *waBinary.Node, []byte, error) {
	if cli == nil {
		return nil, nil, ErrClientIsNil
	}
	waiter := cli.waitResponse(query.ID)
	attrs := waBinary.Attrs{
		"type": string(query.Type),
	}
	if !query.To.IsEmpty() {
		attrs["to"] = query.To
	}
	data, err := cli.sendNodeAndGetData(
		context.Background(),
		waBinary.Node{
			Tag:     "iq",
			Attrs:   attrs,
			Content: query.Content,
		})
	if err != nil {
		cli.cancelResponse(query.ID, waiter)
		return nil, data, err
	}
	return waiter, data, nil
}

// 是否聊天的参与者都在拿到的设备列表里
func isAllParticipantsInAllDevices(participants []types.JID, devices []types.JID) bool {
	numParticipants := len(participants)
	numParticipantsInAllDevices := 0
	for _, participant := range participants {
		u := participant.User
		for _, device := range devices {
			if u == device.User {
				numParticipantsInAllDevices++
				break
			}
		}
	}
	if numParticipantsInAllDevices == numParticipants {
		return true
	}
	return false
}
