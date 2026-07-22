package whatsmeow

import (
	"context"
	"fmt"
	"go.mau.fi/whatsmeow/appstate"
	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/proto/waSyncAction"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
	"time"
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

// SetPicture 设置用户头像，图片建议使用 192x192 规格。
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

// AddContact 添加指定 JID 为联系人。
func (cli *Client) AddContact(jid types.JID) error {
	ctx := context.Background()

	mutations := make([]appstate.MutationInfo, 0, 1)

	// 先从本地映射里获取 LID；没有的话再请求用户信息填充缓存。
	lid, _ := cli.Store.LIDs.GetLIDForPN(ctx, jid)
	if lid.User == "" {
		info, _ := cli.GetUserInfo(ctx, []types.JID{jid})
		if info != nil {
			lid, _ = cli.Store.LIDs.GetLIDForPN(ctx, jid)
		}
	}

	mutation := appstate.MutationInfo{
		// 这里不加额外的 "1"，否则不会同步到主设备通讯录。
		// Index: []string{appstate.IndexContact, jid.String(), "1"},
		Index: []string{appstate.IndexContact, jid.String()},
		Value: &waSyncAction.SyncActionValue{
			Timestamp: proto.Int64(time.Now().UnixMilli()),
			ContactAction: &waSyncAction.ContactAction{
				// 1
				FullName: proto.String(jid.User),
				// 3
				LidJID: proto.String(lid.String()),
				// 5
				PnJID: proto.String(jid.String()),
				// 4 保存到主通讯录
				SaveOnPrimaryAddressbook: proto.Bool(true),
			},
		},
	}

	mutations = append(mutations, mutation)

	patch := appstate.PatchInfo{
		Type:      appstate.WAPatchCriticalUnblockLow,
		Mutations: mutations,
	}

	// 发送 patch 到服务器。
	err := cli.SendAppState(ctx, patch)

	return err

}
