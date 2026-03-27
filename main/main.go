package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	_ "github.com/mattn/go-sqlite3" // 使用官方 SQLite 驱动，支持 CGO
	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/message"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var (
	client *whatsmeow.Client
	ctx    = context.Background()
	lastQR string
)

func main() {

	// HTTP API
	http.HandleFunc("/login", pairLoginHandler)
	http.HandleFunc("/sendMessage", sendMessageHandler)
	http.HandleFunc("/devices", getDevicesHandler)

	fmt.Println("🚀 HTTP API 服务启动，监听 :9090")
	go http.ListenAndServe(":9090", nil)

	// 阻塞直到 Ctrl+C
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
}

// 请求配对码登录
func pairLoginHandler(w http.ResponseWriter, r *http.Request) {
	type Req struct {
		PHONE string `json:"phone"`
	}
	var req Req
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	// 数据库日志
	dbLog := waLog.Stdout("Database", "DEBUG", true)

	// 如果要新扫码账户登录，改个数据库名就行
	var userDbName = "whatsmeow_" + req.PHONE + ".db"
	// 创建 SQLite 数据库存储
	container, err := sqlstore.New(ctx, "sqlite3", "file:"+userDbName+"?_foreign_keys=on", dbLog)
	if err != nil {
		panic(err)
	}

	// 获取设备
	deviceStore, err := container.GetFirstDevice(ctx)
	if err != nil {
		panic(err)
	}

	// 客户端日志
	clientLog := waLog.Stdout("Client", "DEBUG", true)

	// 创建 WhatsApp 客户端
	client = whatsmeow.NewClient(deviceStore, clientLog)

	//// 罗拉美国ip
	//client.SetProxyAddress(fmt.Sprintf("socks5://proxy35_dc_%d-country-us:HPQnGB@proxyus.rola.vip:2000", 666))

	// 添加事件处理器
	client.AddEventHandler(func(evt interface{}) {
		switch v := evt.(type) {
		case *events.Connected:
			fmt.Println("✅ 已连接 WhatsApp")
			// 发送同步regular_high的协议， 每次连接成功都发一次。
			//_ = client.FetchAppState(ctx, "regular_high", false, false)

			//jid, _ := types.ParseJID("923288387163@s.whatsapp.net")
			//realJID, _ := client.Store.GetAltJID(ctx, jid)
			//if realJID.IsEmpty() {
			//info, _ := client.GetUserInfo(ctx, []types.JID{jid})
			//fmt.Printf("userInfo: %+v\n", info)
			//	realJID, _ = client.Store.GetAltJID(ctx, jid)
			//}
			//fmt.Printf("realJID: %+v\n", realJID)

			//devices_, _ := client.GetUserDevices(ctx, []types.JID{jid})
			//fmt.Printf("userDevices: %+v\n", devices_)

			//// 加上一些新协议，测试有没有效果
			//go mewXml()

			//// 获取群邀请链接，必须是管理员才能获取
			//groupJid := types.NewJID("120363406210968859", "g.us")
			//link, err := client.GetGroupInviteLink(ctx, groupJid, false)
			//if err != nil {
			//	log.Fatal(err)
			//}
			//fmt.Println("invite link:", link)
		case *events.AppStateSyncComplete:
			// 同步完成，但是同步事件有多个，所以这里也就会调用多次
			fmt.Println("AppStateSyncComplete")
		case *events.Message:
			fmt.Println("收到消息==============")
			raw, err := proto.Marshal(v.Message)
			if err == nil {
				fmt.Println("message protobuf hex:", hex.EncodeToString(raw))
			}
		case *events.Contact:
			fmt.Printf("=== Contact %v \n", v.JID.String())
		case *events.Disconnected:
			// 断开连接了，需要考虑重连
			fmt.Println("===Disconnected")
		case *events.KeepAliveTimeout:
			// 重要回调，心跳超时：此种状态不应该执行发送消息等动作，需要重连
			fmt.Printf("client.IsConnected() %v \n", client.IsConnected())
			fmt.Println("===KeepAliveTimeout")
		case *events.StreamReplaced:
			// 被顶号
			fmt.Println("===被顶号")
			fmt.Printf("client.IsConnected() %v \n", client.IsConnected())
		case *events.ConnectFailureReason:
			fmt.Println("===ConnectFailureReason=" + v.NumberString())
		case *events.GroupInfo:
			group := v.JID.String()
			fmt.Println("===创建事件" + group)

			// 群被风控
			if len(v.UnknownChanges) > 0 {
				for _, node := range v.UnknownChanges {
					if node.Tag == "suspended" {
						fmt.Println("⚠️ 群被 suspended（风控冻结）:", group)
					}
				}
			}

			// 有人被设置为管理员
			if len(v.Promote) > 0 {
				for _, phoneNumber := range v.Promote {
					log.Printf(
						"[group promote] group=%s user=%s",
						group,
						phoneNumber,
					)
				}
			}

			// 有人被取消管理员
			if len(v.Demote) > 0 {
				for _, phoneNumber := range v.Demote {
					log.Printf(
						"[group demote] group=%s user=%s",
						group,
						phoneNumber,
					)
				}
			}

			// 有人被加进群
			if len(v.Join) > 0 {
				for _, phoneNumber := range v.Join {
					log.Printf(
						"[group join] group=%s user=%s",
						group,
						phoneNumber,
					)
				}
			}

			// 有人离开 / 被移除
			if len(v.Leave) > 0 {
				for _, phoneNumber := range v.Leave {
					log.Printf(
						"[group leave] group=%s user=%s",
						group,
						phoneNumber,
					)
				}
			}
		}
	})

	var msg = ""

	// 判断是否需要扫码登录
	if client.Store.ID == nil {
		if true {
			// -------- 配对码登录
			// 生成一个配对码
			msg = "create pairCode 8888-8888"
			err = client.Connect()
			if err != nil {
				panic(err)
			}
			if client.IsConnected() {
				phoneNumber := req.PHONE
				_, err := client.PairPhone(ctx, phoneNumber, true, whatsmeow.PairClientChrome, "Chrome (Linux)")
				if err != nil {
					fmt.Printf("failed PairPhone %v \n", err)
				}
				//// 模拟请求配对码之后断网，输入配对码，出现“出错了，请重试”的情况问题。
				//go func() {
				//	time.Sleep(10000 * time.Millisecond)
				//	client.Disconnect()
				//	time.Sleep(10000 * time.Millisecond)
				//	client.Connect()
				//}()
			}
		} else {
			// ---------- 二维码登录
			qrChan, _ := client.GetQRChannel(context.Background())
			err = client.Connect()
			if err != nil {
				panic(err)
			}
			for evt := range qrChan {
				// evt.Type: "code", "success", "timeout" 等
				if evt.Event == "code" {
					// 渲染 QR
					lastQR = evt.Code
					qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
					fmt.Println("请使用手机 WhatsApp 扫码登录")
				} else {
					fmt.Println("Login event:", evt.Event)
				}
			}
		}
	} else {
		// 已登录直接连接
		msg = "success"
		err = client.Connect()
		if err != nil {
			panic(err)
		}
	}

	// 返回成功
	response := map[string]string{
		"status": "ok",
		"msg":    msg,
		"time":   time.Now().Format("2006-01-02 15:04:05"),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// 发送消息接口
func sendMessageHandler(w http.ResponseWriter, r *http.Request) {
	// 记录开始时间戳
	startTimeStamp := time.Now().UnixMilli()

	type Req struct {
		JID     string `json:"jid"`
		TYPE    int32  `json:"type"`
		Message string `json:"message"`
		HEX     string `json:"hex"`
		Remove  bool   `json:"isRemove"`
	}
	var req Req
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	// 把字符串 JID 转成 types.JID
	jid, _ := types.ParseJID(req.JID)

	// jid升级到lid
	realJID, _ := client.Store.GetAltJID(ctx, jid)
	if realJID.IsEmpty() {
		info, _ := client.GetUserInfo(ctx, []types.JID{jid})
		fmt.Printf("userInfo: %+v\n", info)
		realJID, _ = client.Store.GetAltJID(ctx, jid)
	}
	fmt.Printf("realJID: %+v\n", realJID)
	if !realJID.IsEmpty() {
		jid = realJID
	}

	// -------

	// 下面模拟真实场景补充协议发送-------

	// 1、发送自己在线状态
	fmt.Println("1、发送自己在线状态" + req.JID)
	// <presence type="available" name="Tank" />
	_ = client.SendPresence(ctx, types.PresenceAvailable)

	// 2、发送订阅请求
	fmt.Println("2、发送订阅请求" + req.JID)
	// <presence type="subscribe" to="639757430046@s.whatsapp.net"><tctoken>0401173767940d8cc2be16</tctoken></presence>
	_ = client.SubscribePresence(ctx, jid)

	// 3、开始输入
	fmt.Println("3、开始输入" + req.JID)
	// <chatstate to="639757430046@s.whatsapp.net"><composing /></chatstate>
	_ = client.SendChatPresence(ctx, jid, types.ChatPresenceComposing, types.ChatPresenceMediaText)

	// 4、查询用户
	if !strings.Contains(req.JID, "@g.us") {
		fmt.Println("4、查询用户" + req.JID)
		devices, _ := client.GetUserDevicesContext(ctx, []types.JID{jid})
		// 如果设备为空，直接返回错误，用户不存在
		if len(devices) == 0 {
			http.Error(w, "no found the user", 500)
			return
		}
	}

	// 延迟0.5 - 1 秒
	randomSleep(500, 1000)

	// 5、输入结束
	fmt.Println("5、输入结束" + req.JID)
	// <chatstate to="639757430046@s.whatsapp.net"><paused /></chatstate>
	_ = client.SendChatPresence(ctx, jid, types.ChatPresencePaused, types.ChatPresenceMediaText)

	// 6、建立信任
	fmt.Println("6、建立信任" + req.JID)
	// <iq to="s.whatsapp.net" type="set" xmlns="privacy" id="29294.52599-149"><tokens><token jid="639757430046@s.whatsapp.net" t="1761622030" type="trusted_contact" /></tokens></iq>
	_ = client.SetTrustedContact(ctx, req.JID)

	// 发送消息
	var msg *waE2E.Message
	switch req.TYPE {
	case 1:
		// 普通文本
		msg = message.BuildTextMessage(req.Message)
	case 2:
		//	按钮消息
		msg = message.BuildButtonMessage()
	case 3:
		//	短链位置
		msg = message.BuildLocalLinkMessage()
	case 5:
		//	大图超链
		msg = message.BuildBigImageMessage()
	case 6:
		//	位置带超链按钮
		msg = message.BuildLocationButtonMessage()
	case 7:
		//
		msg = message.BuildImageUrlMessage()
	case 13:
		// 群聊邀请模式3
		msg = message.BuildGroupMode3()
	case 20:
		// 直接传递十六进制字符串，解析反序列化再发送
		msg = message.BuildMessageFromHex(req.HEX)
	default:
		msg = message.BuildTextMessage(req.Message)
	}

	// 发送消息
	fmt.Println("7、发送消息" + req.JID)
	resp, err := client.SendMessage(context.Background(), jid, msg)
	if err != nil {
		http.Error(w, "failed to send message: "+err.Error(), 500)
		return
	}

	//// 延迟0.5 - 1 秒
	//randomSleep(200, 500)

	// 删除发送的信息
	if req.Remove {
		fmt.Printf("8、删除消息 resp.ID=%s , resp.Timestamp=%d \n", resp.ID, resp.Timestamp.Unix())
		err = message.DeleteChat(client, req.JID, resp.ID, resp.Timestamp.Unix())
		if err != nil {
			fmt.Printf("failed DeleteChat: %v", err)
		}
	}

	// 返回成功
	response := map[string]string{
		"status":       "ok",
		"time":         time.Now().Format("2006-01-02 15:04:05"),
		"time_consume": fmt.Sprintf("%d", (time.Now().UnixMilli() - startTimeStamp)),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// 返回用户关联的设备列表
func getDevicesHandler(w http.ResponseWriter, r *http.Request) {
	type Req struct {
		Numbers []string `json:"numbers"`
	}
	// 获取当前关联的设备
	var req Req
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	var batch []types.JID
	for _, number := range req.Numbers {
		jid, _ := types.ParseJID(number + "@s.whatsapp.net")
		batch = append(batch, jid)
	}

	contacts, err := client.Store.Contacts.GetAllContacts(ctx)
	fmt.Println(contacts)

	devices, err := client.GetUserDevicesContext(ctx, batch)
	if err != nil {
		log.Fatalf("get devices error: %v", err)
	}

	//devices=[923485507679@s.whatsapp.net 923222051194@s.whatsapp.net 923222051194:28@s.whatsapp.net 923222051194:29@s.whatsapp.net 923222051194:31@s.whatsapp.net]

	// 初始化结果 map
	result := make(map[string]int)
	for _, num := range req.Numbers {
		result[num] = 0
	}
	// 统计
	for _, d := range devices {
		// 去掉 @s.whatsapp.net
		left := strings.Split(d.String(), "@")[0]
		// 去掉 :device_id
		base := strings.Split(left, ":")[0]
		if _, ok := result[base]; ok {
			result[base]++
		}
	}
	// 转成 json
	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println("用户的设备列表:" + string(data))

	// 返回 JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// 生成指定范围内的随机延迟
func randomMilliseconds(min, max int) time.Duration {
	if min >= max {
		return time.Duration(min) * time.Millisecond
	}
	// 使用当前时间作为随机种子
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	delay := r.Intn(max-min+1) + min
	return time.Duration(delay) * time.Millisecond
}

// 随机延迟函数	随机延迟min - max 毫秒
func randomSleep(minMs, maxMs int) {
	delay := randomMilliseconds(minMs, maxMs)
	fmt.Printf("随机延迟: %v\n", delay)
	time.Sleep(delay)
}
