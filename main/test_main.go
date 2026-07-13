package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	//_ "github.com/mattn/go-sqlite3" // 使用官方 SQLite 驱动，支持 CGO
	// 替换为纯 Go 的驱动
	_ "github.com/glebarez/go-sqlite"
	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/proto/waSyncAction"
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

const globalCacheDir = "./main"

var (
	client *whatsmeow.Client
	ctx    = context.Background()
)

func main() {

	// HTTP API
	http.HandleFunc("/import", importHandler)
	http.HandleFunc("/login", pairLoginHandler)
	http.HandleFunc("/sendMessage", sendMessageHandler)
	http.HandleFunc("/devices", getDevicesHandler)
	http.HandleFunc("/addContact", addContactHandler)
	http.HandleFunc("/getAllContact", getAllContactHandler)
	http.HandleFunc("/setNickNameAndPicture", setNickNameAndPicture)

	fmt.Println("🚀 HTTP API 服务启动，监听 :9090")
	go http.ListenAndServe(":9090", nil)

	// 阻塞直到 Ctrl+C
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
}

// 请求配对码登录
func importHandler(w http.ResponseWriter, r *http.Request) {

	type Req struct {
		PHONE string `json:"phone"`
		SQL   string `json:"sql_base64"`
	}
	var req Req
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	fmt.Println("import :", req.PHONE)

	// Base64 解码
	decodedSQL, err := base64.StdEncoding.DecodeString(req.SQL)
	if err != nil {
		fmt.Println("base64 decode error:", err)
		http.Error(w, "Invalid base64 encoding", 400)
		return
	}

	// 将解码后的字节转为字符串
	sqlStr := string(decodedSQL)
	fmt.Println("Decoded SQL:", sqlStr)

	success, err := importPhoneNoise(req.PHONE, sqlStr)
	if err != nil {
		fmt.Println("login error:", err)
		http.Error(w, err.Error(), 400)
	}

	// 返回成功
	response := map[string]interface{}{
		"status":  "ok",
		"success": success,
		"time":    time.Now().Format("2006-01-02 15:04:05"),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// 请求配对码登录
func pairLoginHandler(w http.ResponseWriter, r *http.Request) {
	type Req struct {
		PHONE    string `json:"phone"`
		PairCode bool   `json:"pairCode"`
	}
	var req Req
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	// 数据库日志
	dbLog := waLog.Stdout("Database", "DEBUG", true)

	// 如果要新扫码账户登录，改个数据库名就行
	var userDbName = globalCacheDir + "/db/" + "whatsmeow_" + req.PHONE + ".db"
	// 创建 SQLite 数据库存储
	/*
		container, err := sqlstore.New(ctx, "sqlite3", "file:"+userDbName+"?_foreign_keys=on", dbLog)
		改为 "sqlite"
	*/

	// 数据库优化：在连接字符串中加入 WAL 模式和忙等待超时参数
	// _pragma=journal_mode(WAL) -> 开启预写日志模式，读写互不阻塞
	// _pragma=synchronous(NORMAL) -> 配合 WAL 模式，提升写入性能并确保安全
	// _busy_timeout=5000 -> 遭遇锁定时自动等待 5000 毫秒（5秒）再重试，不立刻报错
	dsn := "file:" + userDbName + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_busy_timeout=5000"

	// 创建 SQLite 数据库存储
	container, err := sqlstore.New(ctx, "sqlite", dsn, dbLog)
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

			// 查询ws认为我所在的地区
			resNode, _ := client.SendIQGetCountryCode()
			fmt.Println("SendIQGetCountryCode=", resNode)

		case *events.Disconnected:
			// 断开连接了，需要考虑重连
			fmt.Println("===Disconnected")
		case *events.Message:

			fmt.Println("收到消息==============")

			// 是否自己账号发出的
			fmt.Println("是否自己账号发送：", v.Info.IsFromMe)

			// 当前设备ID
			myDeviceID := client.Store.ID.Device

			// 消息来源设备ID
			msgDeviceID := v.Info.Sender.Device

			// 是否当前设备发送
			isFromThisDevice := v.Info.IsFromMe && (myDeviceID == msgDeviceID)

			fmt.Println("当前设备ID：", myDeviceID)
			fmt.Println("消息设备ID：", msgDeviceID)
			fmt.Println("是否本设备发送：", isFromThisDevice)

			raw, err := proto.Marshal(v.Message)
			if err == nil {
				fmt.Println("message protobuf hex:", hex.EncodeToString(raw))
			}

		case *events.LoggedOut:
			fmt.Println("设备移除:")

		case *events.AppStateSyncComplete:

			fmt.Println("同步AppStateSyncComplete:")

			switch v.Name {
			case appstate.WAPatchCriticalBlock:
				fmt.Println("critical_block 同步完成")
				// 用户资料、pushname、locale 等
			case appstate.WAPatchCriticalUnblockLow:
				fmt.Println("critical_unblock_low 同步完成")
				// 通讯录联系人
			case appstate.WAPatchRegularLow:
				fmt.Println("regular_low 同步完成")
				// pin/archive 等聊天设置
			case appstate.WAPatchRegularHigh:
				fmt.Println("regular_high 同步完成")
				// mute/star 等
			case appstate.WAPatchRegular:
				fmt.Println("regular 同步完成")
				// appstate协议自身数据
			}

		case *events.AppStateSyncError:

			fmt.Println("同步AppStateSyncError:")
			fmt.Printf(`v="%v"`, v)

		case *events.HistorySync:
			log.Printf("HISTORY SYNC")

		}
	})

	var msg = ""

	// 已连接
	if client.IsConnected() {
		msg = "已连接"
	}

	err = client.Connect()
	if err != nil {
		msg = err.Error()
	}

	// 未登录 → 返回配对码
	if client.Store.ID == nil {

		fmt.Println("📱 未登录，开始配对:", req.PHONE)

		var code string

		if req.PairCode {
			// ---------- 配对码登录
			err = client.Connect()
			code, err = client.PairPhone(
				ctx,
				req.PHONE,
				true,
				whatsmeow.PairClientChrome,
				"Chrome (Linux)",
			)
		} else {
			// ---------- 二维码登录
			client.Disconnect()
			qrChan, _ := client.GetQRChannel(context.Background())
			err = client.Connect()
			for evt := range qrChan {
				// evt.Type: "code", "success", "timeout" 等
				if evt.Event == "code" {
					code = evt.Code
					// 渲染 QR
					qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
					fmt.Println("请使用手机 WhatsApp 扫码登录")
				} else {
					fmt.Println("Login event:", evt.Event)
				}
			}
		}

		if err != nil {
			msg = err.Error()
		}

		msg = "配对码:" + code
	} else {
		msg = "已直接登录"
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
		JSON map[string]interface{} `json:"json"`
	}
	var req Req
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	jsonBytes, err := json.Marshal(req.JSON)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	err = SendMessage(true, true, string(jsonBytes))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
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

func SendMessage(isLid bool, isBiz bool, jsonStr string) error {

	ctx := context.Background()

	var request MessageRequest

	err := json.Unmarshal([]byte(jsonStr), &request)
	if err != nil {
		return err
	}

	// 把字符串 JID 转成 types.JID
	jid, _ := types.ParseJID(request.To + "@s.whatsapp.net")

	if isLid {
		fmt.Println("转成lid")
		// jid升级到lid
		realJID, _ := client.Store.GetAltJID(ctx, jid)
		if realJID.IsEmpty() {
			info, _ := client.GetUserInfo(ctx, []types.JID{jid})
			fmt.Printf("userInfo: %+v\n", info)
			realJID, _ = client.Store.GetAltJID(ctx, jid)
		}
		if !realJID.IsEmpty() {
			jid = realJID
		}
	}

	fmt.Printf("jid: %s", jid)

	// 4、查询用户
	devices, err := client.IsOnWhatsApp(ctx, []string{request.To})
	if err != nil {
		return err
	}
	// 如果设备为空，直接返回错误，用户不存在
	if len(devices) == 0 {
		return errors.New("no found the user")
	}

	// 下面模拟真实场景补充协议发送-------

	// 1、发送自己在线状态
	// <presence type="available" name="Tank" />
	_ = client.SendPresence(ctx, types.PresenceAvailable)

	// 2-3分钟后离线
	go func(c *whatsmeow.Client) {
		waitSeconds := 120 + rand.Intn(60)
		time.Sleep(time.Duration(waitSeconds) * time.Second)
		if c != nil && c.IsConnected() {
			_ = c.SendPresence(ctx, types.PresenceUnavailable)
		}
	}(client)

	// 2、发送订阅请求
	fmt.Println("2、发送订阅请求")
	// <presence type="subscribe" to="639757430046@s.whatsapp.net"><tctoken>0401173767940d8cc2be16</tctoken></presence>
	_ = client.SubscribePresence(ctx, jid)

	// 3、开始输入
	// <chatstate to="639757430046@s.whatsapp.net"><composing /></chatstate>
	_ = client.SendChatPresence(
		ctx,
		jid,
		types.ChatPresenceComposing,
		types.ChatPresenceMediaText,
	)

	// 延迟1 - 2 秒
	randomSleep(1000, 2000)

	// 5、输入结束
	// <chatstate to="639757430046@s.whatsapp.net"><paused /></chatstate>
	_ = client.SendChatPresence(
		ctx,
		jid,
		types.ChatPresencePaused,
		types.ChatPresenceMediaText,
	)

	// 执行发送
	msg := waE2E.Message{}
	typeVal := request.Type
	imageCachePath := globalCacheDir + "/images"
	if typeVal == 0 {
		// 0短消息（地图定位）
		msg = SendLinkType0(imageCachePath, request)
	} else if typeVal == 2 {
		// 2大图消息
		msg = SendLinkType2(client, imageCachePath, request)
	} else if typeVal == 11 {
		// 11邀请入群模式1
		msg = SendLinkType11(client, imageCachePath, request)
	} else if typeVal == 12 {
		// 12邀请入群模式2
		msg = SendLinkType12(imageCachePath, request)
	} else if typeVal == 13 {
		// 13邀请入群模式3
		msg = SendLinkType13(imageCachePath, request)
	} else {
		// 发送proto hex
		msg = BuildMessageFromHex(request.Hex)
	}

	client.SendReportingTokens = true
	resp, err := client.SendMessage(ctx, isLid, false, isBiz, jid, &msg)
	if err != nil {
		return err
	}

	fmt.Println(resp)

	return nil
}

// 返回用户关联的设备列表
func getDevicesHandler(w http.ResponseWriter, r *http.Request) {

	type Req struct {
		PHONE   string   `json:"phone"`
		Numbers []string `json:"numbers"`
	}
	var req Req
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	result := GetDevices(req.PHONE, req.Numbers)

	var obj interface{}

	err := json.Unmarshal([]byte(result), &obj)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// 返回 JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(obj)
}

func GetDevices(phone string, numbers []string) string {
	ctx := context.Background()

	var batch []types.JID
	for _, number := range numbers {
		jid, _ := types.ParseJID(number + "@s.whatsapp.net")
		batch = append(batch, jid)
	}

	// --查询目标用户绑定设备数

	devices, err := client.GetUserDevicesContext(ctx, batch)
	if err != nil {
		log.Fatalf("get devices error: %v", err)
	}

	// 初始化结果 map
	result := make(map[string]int)
	for _, num := range numbers {
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

	return string(data)
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

func addContactHandler(w http.ResponseWriter, r *http.Request) {
	type Req struct {
		PHONE   string   `json:"phone"`
		NUMBERS []string `json:"numbers"`
	}
	var req Req
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	err := AddContacts(req.PHONE, req.NUMBERS)
	if err != nil {
		http.Error(w, err.Error(), 400)

		// 强刷一下
		// 强制同步一下联系人
		err = client.FetchAppState(ctx,
			appstate.WAPatchCriticalUnblockLow,
			true,
			false,
		)

		return
	}

	// 返回成功
	response := map[string]string{
		"status": "ok",
		"time":   time.Now().Format("2006-01-02 15:04:05"),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

/*
添加为联系人
*/
func AddContacts(phone string, numbers []string) error {
	// 添加联系人
	ctx := context.Background()

	// --查询去除未注册的号码
	devices, err := client.IsOnWhatsApp(ctx, numbers)
	if err != nil {
		return err
	}

	mutations := make([]appstate.MutationInfo, 0, len(devices))

	for _, device := range devices {
		// 判断获取lid
		lid, _ := client.Store.LIDs.GetLIDForPN(ctx, device.JID)
		if lid.User == "" {
			// 需要获取信息
			info, _ := client.GetUserInfo(ctx, []types.JID{device.JID})
			if info != nil {
				lid, _ = client.Store.LIDs.GetLIDForPN(ctx, device.JID)
			}
			// 最好还是随机延迟一下
			randomMilliseconds(1000, 2000)
		}

		mutation := appstate.MutationInfo{
			// 加上这个"1" 不会同步到挂机账号的手机上
			//Index: []string{appstate.IndexContact, device.JID.String(), "1"},
			Index: []string{appstate.IndexContact, device.JID.String()},
			Value: &waSyncAction.SyncActionValue{
				Timestamp: proto.Int64(time.Now().UnixMilli()),
				ContactAction: &waSyncAction.ContactAction{
					// 1
					FullName: proto.String(device.JID.User),
					// 3
					LidJID: proto.String(lid.String()),
					// 5
					PnJID: proto.String(device.JID.String()),
					// 4 保存到主通讯录
					SaveOnPrimaryAddressbook: proto.Bool(true),
				},
			},
		}

		mutations = append(mutations, mutation)
	}

	patch := appstate.PatchInfo{
		Type:      appstate.WAPatchCriticalUnblockLow,
		Mutations: mutations,
	}

	// 发送 patch 到服务器
	err = client.SendAppState(ctx, patch)

	return err

}

type MessageRequest struct {
	TaskRecordId uint64 `json:"taskRecordId"`
	SendPhone    string `json:"sendPhone"`
	To           string `json:"to" binding:"required"`
	Link         string `json:"link" binding:"required"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	ThumbnailUrl string `json:"thumbnailUrl"`
	Content      string `json:"content"`
	Width        uint32 `json:"width"`  // 图片的宽，像素
	Height       uint32 `json:"height"` // 图片的高，像素
	Type         int    `json:"type"`   // 0短消息（地图定位）， 1长消息， 2大图消息， 11邀请入群模式1， 12邀请入群模式2， 13邀请入群模式3

	Hex string `json:"hex"` // hex
}

// 获取所有联系人
func getAllContactHandler(w http.ResponseWriter, r *http.Request) {
	type Req struct {
		PHONE string `json:"phone"`
	}
	var req Req
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	result := getContacts(req.PHONE)

	// 返回成功
	response := map[string]string{
		"status": "ok",
		"result": result,
		"time":   time.Now().Format("2006-01-02 15:04:05"),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func getContacts(phone string) string {
	// 获取联系人
	ctx := context.Background()

	contacts, err := client.Store.Contacts.GetAllContacts(ctx)

	if err != nil {
		fmt.Println("获取联系人失败:", err)
		return ""
	}

	fmt.Printf("总联系人数量: %d\n\n", len(contacts))

	// 通讯录联系人数量
	realContactCount := 0

	// 通讯录联系人数量
	result := make(map[string]types.ContactInfo)
	for jid, info := range contacts {
		// 只判断 FullName 有值的联系人，FullName有值才是真正通讯录好友
		if info.FullName == "" {
			continue
		}

		fmt.Println("===================================")
		fmt.Println("JID         :", jid.String())
		fmt.Println("PushName    :", info.PushName)
		fmt.Println("FullName    :", info.FullName)
		fmt.Println("FirstName   :", info.FirstName)
		fmt.Println("BusinessName:", info.BusinessName)
		fmt.Println("FoundName   :", info.Found)
		fmt.Println("")

		fmt.Printf("详细信息: %+v\n", info)
		fmt.Println("===================================")

		realContactCount++
		result[jid.String()] = info
	}

	// 输出通讯录联系人数量
	fmt.Printf("\n通讯录联系人数量: %d\n", realContactCount)

	data, err := json.Marshal(result)
	if err != nil {
		return ""
	}

	return string(data)
}

// 设置昵称和头像
func setNickNameAndPicture(w http.ResponseWriter, r *http.Request) {
	type Req struct {
		JID      string `json:"jid"`
		NickName string `json:"nickName"`
		Picture  string `json:"picture"`
	}
	var req Req
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	// 设置昵称
	// <presence name="TankPanda" />

	picture, _ := hex.DecodeString(req.Picture)
	err := client.SetPicture(ctx, picture)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	// 返回成功
	response := map[string]string{
		"status": "ok",
		"time":   time.Now().Format("2006-01-02 15:04:05"),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func importPhoneNoise(phone string, insertSQL string) (bool, error) {

	err := importSQL(phone, insertSQL)
	if err != nil {
		return false, err
	}

	return true, nil
}

func importSQL(phone string, insertSQL string) error {

	ctx := context.Background()

	// 如果要新扫码账户登录，改个数据库名就行
	var userDbName = globalCacheDir + "/db/" + "whatsmeow_" + phone + ".db"
	dsn := "file:" + userDbName + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_busy_timeout=5000"

	container, err := sqlstore.New(ctx, "sqlite", dsn, nil)
	if err != nil {
		return err
	}

	// 触发自动建表
	_, err = container.GetFirstDevice(ctx)
	if err != nil {
		return err
	}

	// 2. 打开 sqlite
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	// 3. 清空原 device
	_, _ = db.Exec(`DELETE FROM whatsmeow_device`)

	// 4. 插入你的 SQL
	_, err = db.Exec(insertSQL)
	if err != nil {
		return fmt.Errorf("insert failed: %v", err)
	}

	return nil
}
