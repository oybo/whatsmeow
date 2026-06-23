package main

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"fmt"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waAdv"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
	"math/rand"
	"strings"
	"time"
)

func SendLinkType0(cacheDir string, request MessageRequest) waE2E.Message {
	var (
		//link = "https://cutt.ly/9rSryoI4"
		link = request.Link
		//title = "Get up to 7,777 cash and 8,888 gift pack! Download"
		title = request.Title
	)

	// 默认空缩略图（关键点）
	var thumb []byte
	// 尝试下载图片
	if request.ThumbnailUrl != "" {
		metadata, _, err := GetImageAndSave(request.ThumbnailUrl, cacheDir)
		if err != nil {
			// 下载失败 → 使用空缩略图
			thumb = []byte{}
		} else {
			thumb = metadata.ImageThumb
		}
	} else {
		// 没传缩略图
		thumb = []byte{}
	}

	fmt.Println("thumb size:", len(thumb))

	// content添加一些随机表情
	title = AddRandomEmojis(title, 1, 1)

	return waE2E.Message{
		LocationMessage: &waE2E.LocationMessage{
			DegreesLatitude:  proto.Float64(0),
			DegreesLongitude: proto.Float64(0),
			Name:             proto.String(link),
			URL:              proto.String(link),
			Address:          proto.String(title),
			JPEGThumbnail:    thumb,
		}, MessageContextInfo: BuildMessageContextInfo(),
	}
}

func SendLinkType2(waCli *whatsmeow.Client, cacheDir string, request MessageRequest) waE2E.Message {
	ctx := context.Background()

	// 创建扩展文本消息（包含链接）
	var (
		//link        = "https://cutt.ly/9rSryoI4"
		//title       = "Clique aqui para participar"
		//content = "🎯A plataforma de jogos PG mais popular do Brasil já está no ar! \n🐉 Milhares de jogos para você escolher, mais de 5.000 títulos PG \n💵 Depósito mínimo de apenas R$10 \n💰 Saques sem valor mínimo, dinheiro na conta a qualquer hora! \n🎁 Deposite hoje e ganhe 100% de bônus + prêmios de até R$7.777 \n🎲 Baixe o aplicativo e registre-se para receber bônus extras aleatórios! \n💸 Deposite R$10 e receba R$20 \n💸 Deposite R$20 e receba R$40 \n👇 Clique no botão abaixo para participar da pr"

		link    = request.Link
		title   = request.Title
		content = request.Content // + "\n\n" + helpers.GenerateMsgRandomString()
	)
	//testTxt := "000001|000002|000003|000004|000005|000006|000007|000008|000009|000010|000011|000012|000013"
	// btnJsonText := `{"display_text":"Clique aqui para participar","url":"https://cutt.ly/brJU4L9F"}`
	btn := make([]*waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton, 2)
	// 1.URL按钮
	btnJsonText := fmt.Sprintf(`{"display_text":"%s","url":"%s","merchant_url":"%s"}`, title, link, link)
	btn[0] = &waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{
		Name:             proto.String("cta_url"),
		ButtonParamsJSON: proto.String(btnJsonText),
	}
	// 再添加一个快捷回复的
	replyBtnJson := `{"display_text":"Stop receiving","id":"reply_consult"}`
	btn[1] = &waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{
		Name:             proto.String("quick_reply"),
		ButtonParamsJSON: proto.String(replyBtnJson),
	}

	// content添加一些随机表情
	content = AddRandomEmojis(content, 1, 3)

	// 默认空缩略图（关键点）
	var thumb []byte
	// 尝试下载图片
	if request.ThumbnailUrl != "" {
		metadata, _, err := GetImageAndSave(request.ThumbnailUrl, cacheDir)
		if err != nil {
			// 下载失败 → 使用空缩略图
			thumb = []byte{}
		} else {
			thumb = metadata.ImageThumb
		}
	} else {
		// 没传缩略图
		thumb = []byte{}
	}

	// 上传图片
	var ImageMessage *waE2E.ImageMessage
	if len(thumb) > 0 {
		dataWaRecipient, _ := types.ParseJID(request.To + "@s.whatsapp.net")
		ctx, cancel := context.WithTimeout(ctx, time.Second*30)
		defer cancel()
		uploadedThumb, errOrigin := uploadMediaCache(waCli, ctx, whatsmeow.MediaImage, thumb, dataWaRecipient)
		if errOrigin == nil {
			// fmt.Printf("uploadedThumb:%+v\n", uploadedThumb)
			ImageMessage =
				&waE2E.ImageMessage{
					// 1
					URL: proto.String(uploadedThumb.URL),
					// 2
					Mimetype: proto.String("image/jpeg"), // 实际测试写死没问题 TODO 考虑实际的 mime 类型
					// 4
					FileSHA256: uploadedThumb.FileSHA256,
					// 5
					FileLength: proto.Uint64(uploadedThumb.FileLength),
					// 6
					Height: proto.Uint32(400), //proto.Uint32(request.Height), // 展示框大小，建议是一个合理的宽高比
					// 7
					Width: proto.Uint32(520), //proto.Uint32(request.Width),
					// 8
					MediaKey: uploadedThumb.MediaKey,
					// 9
					FileEncSHA256: uploadedThumb.FileEncSHA256,
					// 11
					DirectPath: proto.String(uploadedThumb.DirectPath),
					// 16
					JPEGThumbnail: thumb,
					// 17
					ContextInfo: &waE2E.ContextInfo{},
					// 25
					ViewOnce: proto.Bool(false),
				}
		}
	}

	return waE2E.Message{
		ViewOnceMessage: &waE2E.FutureProofMessage{
			Message: &waE2E.Message{
				InteractiveMessage: &waE2E.InteractiveMessage{
					// 1
					Header: &waE2E.InteractiveMessage_Header{
						Media: &waE2E.InteractiveMessage_Header_ImageMessage{
							ImageMessage: ImageMessage,
						},
						HasMediaAttachment: proto.Bool(true),
					},
					// 2
					Body: &waE2E.InteractiveMessage_Body{
						Text: proto.String(content), // TODO 广告内容
					},
					// 3 先固定该值，后台文案模版没有配置
					Footer: &waE2E.InteractiveMessage_Footer{
						Text: proto.String("Some users found this useful so sharing here 😊 If it’s not relevant for you, no worries — just ignore it 👍"),
					},
					// 6
					InteractiveMessage: &waE2E.InteractiveMessage_NativeFlowMessage_{
						NativeFlowMessage: &waE2E.InteractiveMessage_NativeFlowMessage{
							// 1
							Buttons: btn,
							// 2
							MessageParamsJSON: proto.String("{}"),
							// 3
							MessageVersion: proto.Int32(1),
						},
					},
					// 15
					ContextInfo: &waE2E.ContextInfo{},
				},
			},
		},
	}
}

func SendLinkType11(waCli *whatsmeow.Client, cacheDir string, request MessageRequest) waE2E.Message {
	ctx := context.Background()

	// 创建扩展文本消息（包含链接）
	var (
		//link        = "https://cutt.ly/9rSryoI4"
		//title       = "Clique aqui para participar"
		//content = "🎯A plataforma de jogos PG mais popular do Brasil já está no ar! \n🐉 Milhares de jogos para você escolher, mais de 5.000 títulos PG \n💵 Depósito mínimo de apenas R$10 \n💰 Saques sem valor mínimo, dinheiro na conta a qualquer hora! \n🎁 Deposite hoje e ganhe 100% de bônus + prêmios de até R$7.777 \n🎲 Baixe o aplicativo e registre-se para receber bônus extras aleatórios! \n💸 Deposite R$10 e receba R$20 \n💸 Deposite R$20 e receba R$40 \n👇 Clique no botão abaixo para participar da pr"

		link = groupUrl2WhatsappFormat(request.Link)
		//title   = request.Title
		btnText = "Join group"
		content = request.Content // + "\n\n" + helpers.GenerateMsgRandomString()
	)
	//testTxt := "000001|000002|000003|000004|000005|000006|000007|000008|000009|000010|000011|000012|000013"
	// btnJsonText := `{"display_text":"Clique aqui para participar","url":"https://cutt.ly/brJU4L9F"}`
	btnJsonText := fmt.Sprintf(`{"display_text":"%s","url":"%s"}`, btnText, link)
	btn := make([]*waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton, 1)
	btn[0] = &waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{ // TODO
		Name:             proto.String("cta_url"),
		ButtonParamsJSON: proto.String(btnJsonText),
	}

	// 默认空缩略图（关键点）
	var thumb []byte
	// 尝试下载图片
	if request.ThumbnailUrl != "" {
		metadata, _, err := GetImageAndSave(request.ThumbnailUrl, cacheDir)
		if err != nil {
			// 下载失败 → 使用空缩略图
			thumb = []byte{}
		} else {
			thumb = metadata.ImageThumb
		}
	} else {
		// 没传缩略图
		thumb = []byte{}
	}

	// 上传图片
	var ImageMessage *waE2E.ImageMessage
	if len(thumb) > 0 {
		dataWaRecipient, _ := types.ParseJID(request.To + "@s.whatsapp.net")
		ctx, cancel := context.WithTimeout(ctx, time.Second*30)
		defer cancel()
		uploadedThumb, errOrigin := uploadMediaCache(waCli, ctx, whatsmeow.MediaImage, thumb, dataWaRecipient)
		if errOrigin == nil {
			// fmt.Printf("uploadedThumb:%+v\n", uploadedThumb)
			ImageMessage =
				&waE2E.ImageMessage{
					URL:           proto.String(uploadedThumb.URL),
					Mimetype:      proto.String("image/jpeg"), // 实际测试写死没问题 TODO 考虑实际的 mime 类型
					FileSHA256:    uploadedThumb.FileSHA256,
					FileLength:    proto.Uint64(uploadedThumb.FileLength),
					Height:        proto.Uint32(400), //proto.Uint32(request.Height), // 展示框大小，建议是一个合理的宽高比
					Width:         proto.Uint32(520), //proto.Uint32(request.Width),
					MediaKey:      uploadedThumb.MediaKey,
					FileEncSHA256: uploadedThumb.FileEncSHA256,
					DirectPath:    proto.String(uploadedThumb.DirectPath),
					JPEGThumbnail: thumb,
					ContextInfo:   &waE2E.ContextInfo{},
					ViewOnce:      proto.Bool(false),
				}
		}
	}

	return waE2E.Message{
		ViewOnceMessage: &waE2E.FutureProofMessage{
			Message: &waE2E.Message{
				InteractiveMessage: &waE2E.InteractiveMessage{
					// 1
					Header: &waE2E.InteractiveMessage_Header{
						Media: &waE2E.InteractiveMessage_Header_ImageMessage{
							ImageMessage: ImageMessage,
						},
						HasMediaAttachment: proto.Bool(true),
					},
					// 2
					Body: &waE2E.InteractiveMessage_Body{
						Text: proto.String(content), // TODO 广告内容
					},
					// 6
					InteractiveMessage: &waE2E.InteractiveMessage_NativeFlowMessage_{
						NativeFlowMessage: &waE2E.InteractiveMessage_NativeFlowMessage{
							Buttons:        btn,
							MessageVersion: proto.Int32(0),
						},
					},
					// 15
					ContextInfo: &waE2E.ContextInfo{},
				},
			},
		},
	}
}

func SendLinkType12(cacheDir string, request MessageRequest) waE2E.Message {
	// 默认空缩略图（关键点）
	var thumb []byte
	// 尝试下载图片
	if request.ThumbnailUrl != "" {
		metadata, _, err := GetImageAndSave(request.ThumbnailUrl, cacheDir)
		if err != nil {
			// 下载失败 → 使用空缩略图
			thumb = []byte{}
		} else {
			thumb = metadata.ImageThumb
		}
	} else {
		// 没传缩略图
		thumb = []byte{}
	}

	// 创建扩展文本消息（包含链接）
	var (
		//name    = "666 HHH"
		name = request.Title
		//address = "群聊邀请"
		address = "Group chat invite"
		// url     = "https://chat.whatsapp.com/FsLKt2J6mwE40CGHhUQucW"
		url = request.Link
		//title   = "hello HHH"
		title = request.Title
		//body    = "点击链接以加入我的 WhatsApp 群组：https://chat.whatsapp.com/FsLKt2J6mwE40CGHhUQucW"
		body = request.Content
		//footText  = "footer text 11122"
		thumbnail = thumb
		//sourceURL    = "https://chat.whatsapp.com/FsLKt2J6mwE40CGHhUQucW"
		btnText = "Join group"                 // 加入群组
		btnUrl  = groupUrl2WhatsappFormat(url) // "whatsapp://chat?code=FsLKt2J6mwE40CGHhUQucW"
	)

	// 按钮部分
	btnJsonText := fmt.Sprintf(`{"display_text":"%s","url":"%s"}`, btnText, btnUrl)

	return waE2E.Message{
		ViewOnceMessage: &waE2E.FutureProofMessage{
			Message: &waE2E.Message{
				//45
				InteractiveMessage: &waE2E.InteractiveMessage{
					//1
					Header: &waE2E.InteractiveMessage_Header{
						//Title:              proto.String(title), //1
						Title:              nil,
						Subtitle:           nil,              //2
						HasMediaAttachment: proto.Bool(true), //5
						Media: &waE2E.InteractiveMessage_Header_LocationMessage{
							//8
							LocationMessage: &waE2E.LocationMessage{
								DegreesLatitude:                   proto.Float64(0),      //1
								DegreesLongitude:                  proto.Float64(0),      //2
								Name:                              proto.String(name),    //3
								Address:                           proto.String(address), //4
								URL:                               proto.String(url),     //5
								IsLive:                            proto.Bool(true),      //6
								AccuracyInMeters:                  proto.Uint32(0),       //7
								SpeedInMps:                        proto.Float32(0),      //8
								DegreesClockwiseFromMagneticNorth: proto.Uint32(0),       //9
								Comment:                           proto.String("11"),    //11
								JPEGThumbnail:                     thumbnail,             //16
								ContextInfo: &waE2E.ContextInfo{ //17
									ExternalAdReply: &waE2E.ContextInfo_ExternalAdReplyInfo{ //28
										Title:        proto.String(title),
										Body:         proto.String("Group chat invite"), //群聊邀请
										MediaType:    waE2E.ContextInfo_ExternalAdReplyInfo_IMAGE.Enum(),
										ThumbnailURL: proto.String(url),
										MediaURL:     proto.String(url),
										Thumbnail:    thumbnail,         //6
										SourceURL:    proto.String(url), //9
										//OriginalImageURL: proto.String("https://mmg.whatsapp.net/v/t72.18990-6/553836548_1896917391168968_5112697119583924817_n.enc?ccb=11-4&oh=01_Q5Aa2wEu30rgZj-8-Ka2MreZGaTcqhcs-z6kFKe9dRL7PKu4gA&oe=68EA6176&_nc_sid=5e03e0&ts=1760067188&bind_token=1bbcb4d4fda377d38a868e96d8184cb9143a2b7465737f8de48d04217b719d42"), //22
										// 缺少 27
									},
								},
							},
						},
					},
					//2
					Body: &waE2E.InteractiveMessage_Body{
						Text: proto.String(body), //1
					},
					//3
					//Footer: &waE2E.InteractiveMessage_Footer{
					//	Text: proto.String(footText),
					//},
					InteractiveMessage: &waE2E.InteractiveMessage_NativeFlowMessage_{
						// 6
						NativeFlowMessage: &waE2E.InteractiveMessage_NativeFlowMessage{
							// 1
							Buttons: []*waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{
								{
									Name:             proto.String("cta_url"),
									ButtonParamsJSON: proto.String(btnJsonText),
								},
							},
						},
					},
				},
			},
		},
	}
}

func SendLinkType13(cacheDir string, request MessageRequest) waE2E.Message {
	// 默认空缩略图（关键点）
	var thumb []byte
	// 尝试下载图片
	if request.ThumbnailUrl != "" {
		metadata, _, err := GetImageAndSave(request.ThumbnailUrl, cacheDir)
		if err != nil {
			// 下载失败 → 使用空缩略图
			thumb = []byte{}
		} else {
			thumb = metadata.ImageThumb
		}
	} else {
		// 没传缩略图
		thumb = []byte{}
	}

	// 创建扩展文本消息（包含链接）
	var (
		title     = request.Title
		text      = request.Content     // 真正的body
		body      = "Group chat invite" // 写死，群聊邀请
		thumbnail = thumb
		//thumbnailURL = "https://chat.whatsapp.com/FsLKt2J6mwE40CGHhUQucW"
		thumbnailURL = request.Link
		sourceURL    = request.Link
	)

	return waE2E.Message{
		//6
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			//1
			Text: proto.String(text),
			//17
			ContextInfo: &waE2E.ContextInfo{
				//28
				ExternalAdReply: &waE2E.ContextInfo_ExternalAdReplyInfo{
					Title:                 proto.String(title),                                //1
					Body:                  proto.String(body),                                 //2
					MediaType:             waE2E.ContextInfo_ExternalAdReplyInfo_IMAGE.Enum(), //3
					ThumbnailURL:          proto.String(thumbnailURL),                         //4
					MediaURL:              proto.String(thumbnailURL),                         //5
					Thumbnail:             thumbnail,                                          //6
					SourceURL:             proto.String(sourceURL),                            //9
					RenderLargerThumbnail: proto.Bool(true),                                   //11
					OriginalImageURL:      proto.String(thumbnailURL),                         //22
				},
			},
		},
	}
}

/*
根据hex反序列号构造message
*/
func BuildMessageFromHex(hexStr string) waE2E.Message {
	// 1、把 hex 字符串 → 转成 []byte
	data, err := hex.DecodeString(hexStr)
	if err != nil {
		panic(err)
	}

	// 2、反序列化 protobuf
	var msg waE2E.Message
	err = proto.Unmarshal(data, &msg)
	if err != nil {
		panic(err)
	}

	return msg
}

// https://chat.whatsapp.com/FsLKt2J6mwE40CGHhUQucW  to   whatsapp://chat?code=FsLKt2J6mwE40CGHhUQucW
func groupUrl2WhatsappFormat(link string) string {
	const prefix = "https://chat.whatsapp.com/"
	code := strings.TrimPrefix(link, prefix)
	code = strings.TrimSpace(code)
	return fmt.Sprintf("whatsapp://chat?code=%s", code)
}

func uploadMediaCache(WaCli *whatsmeow.Client, ctx context.Context, mediaType whatsmeow.MediaType, media []byte, recipient types.JID) (uploaded whatsmeow.UploadResponse, err error) {
	// 上传
	uploaded, err = WaCli.Upload(ctx, media, mediaType)
	return
}

var emojis = []string{
	"😀", "😂", "🤣", "😊", "😍",
	"🥰", "😘", "😎", "🤔", "🥳",
	"🔥", "💯", "👍", "👏", "🎉",
	"❤️", "✨", "🌟", "🚀", "🎈",
}

func AddRandomEmojis(text string, minCount, maxCount int) string {
	if len(text) == 0 {
		return text
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// 随机决定插入多少个表情
	count := minCount
	if maxCount > minCount {
		count += r.Intn(maxCount - minCount + 1)
	}

	chars := strings.Split(text, "")

	for i := 0; i < count; i++ {
		emoji := emojis[r.Intn(len(emojis))]
		pos := r.Intn(len(chars) + 1)

		chars = append(chars[:pos],
			append([]string{emoji}, chars[pos:]...)...)
	}

	return strings.Join(chars, "")
}

func BuildMessageContextInfo() *waE2E.MessageContextInfo {
	var senderKeyHash []byte
	var senderTimestamp uint64

	senderKeyHash = nil
	var details waAdv.ADVDeviceIdentity
	if err := proto.Unmarshal(client.Store.Account.Details, &details); err == nil && details.Timestamp != nil {
		senderTimestamp = *details.Timestamp
	}
	// 如果上述解析时间戳失败或为空，提供兜底时间戳
	if senderTimestamp == 0 {
		senderTimestamp = uint64(time.Now().Unix())
	}

	messageSecret := make([]byte, 32)
	_, _ = crand.Reader.Read(messageSecret)

	return &waE2E.MessageContextInfo{
		// 1
		DeviceListMetadata: &waE2E.DeviceListMetadata{
			// 1
			SenderKeyHash: senderKeyHash,
			// 2
			SenderTimestamp: proto.Uint64(senderTimestamp),
			// 4
			SenderAccountType: waAdv.ADVEncryptionType_E2EE.Enum(),
			// 5
			ReceiverAccountType: waAdv.ADVEncryptionType_E2EE.Enum(),
			// 8
			RecipientKeyHash: nil,
			// 9
			RecipientTimestamp: nil,
		},
		// 2
		DeviceListMetadataVersion: proto.Int32(2),
		// 3
		MessageSecret: messageSecret,
	}
}
