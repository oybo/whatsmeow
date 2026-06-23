package whatsmeow

import (
	"context"
	"fmt"
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

func (cli *Client) SendIQGetCountryCode() (*waBinary.Node, error) {
	if cli == nil {
		return nil, nil
	}

	respCh, err := cli.sendIQ(context.Background(), infoQuery{
		Namespace: "md",
		Type:      "get",
		To:        types.ServerJID,
		Content: []waBinary.Node{
			{Tag: "link_code_companion_reg", Attrs: waBinary.Attrs{
				"stage": "get_country_code",
			}},
		},
	})

	return respCh, err
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

/*
设置用户头像	192 * 192规格
<iq to="s.whatsapp.net" type="set" xmlns="w:profile:picture" id="25948.489-21189">

	<picture type="image">
	   ffd8ffe000104a46494600......
	</picture>

</iq>
*/
func (cli *Client) SetPicture(ctx context.Context, picture []byte) error {
	if cli == nil {
		return ErrClientIsNil
	}
	_, err := cli.sendIQ(ctx, infoQuery{
		Namespace: "w:profile:picture",
		Type:      "set",
		To:        types.ServerJID,
		Content: []waBinary.Node{{
			Tag: "picture",
			Attrs: waBinary.Attrs{
				"type": "image",
			},
			Content: picture,
		}},
	})
	if err != nil {
		return fmt.Errorf("error SetTrustedContact: %w", err)
	}
	return nil
}
