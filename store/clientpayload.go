// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package store

import (
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"google.golang.org/protobuf/proto"

	"go.mau.fi/libsignal/ecc"

	"go.mau.fi/whatsmeow/proto/waCompanionReg"
	"go.mau.fi/whatsmeow/proto/waWa6"
	"go.mau.fi/whatsmeow/types"
)

// WAVersionContainer is a container for a WhatsApp web version number.
type WAVersionContainer [3]uint32

// ParseVersion parses a version string (three dot-separated numbers) into a WAVersionContainer.
func ParseVersion(version string) (parsed WAVersionContainer, err error) {
	var part1, part2, part3 int
	if parts := strings.Split(version, "."); len(parts) != 3 {
		err = fmt.Errorf("'%s' doesn't contain three dot-separated parts", version)
	} else if part1, err = strconv.Atoi(parts[0]); err != nil {
		err = fmt.Errorf("first part of '%s' is not a number: %w", version, err)
	} else if part2, err = strconv.Atoi(parts[1]); err != nil {
		err = fmt.Errorf("second part of '%s' is not a number: %w", version, err)
	} else if part3, err = strconv.Atoi(parts[2]); err != nil {
		err = fmt.Errorf("third part of '%s' is not a number: %w", version, err)
	} else {
		parsed = WAVersionContainer{uint32(part1), uint32(part2), uint32(part3)}
	}
	return
}

func (vc WAVersionContainer) LessThan(other WAVersionContainer) bool {
	return vc[0] < other[0] ||
		(vc[0] == other[0] && vc[1] < other[1]) ||
		(vc[0] == other[0] && vc[1] == other[1] && vc[2] < other[2])
}

// IsZero returns true if the version is zero.
func (vc WAVersionContainer) IsZero() bool {
	return vc == [3]uint32{0, 0, 0}
}

// String returns the version number as a dot-separated string.
func (vc WAVersionContainer) String() string {
	parts := make([]string, len(vc))
	for i, part := range vc {
		parts[i] = strconv.Itoa(int(part))
	}
	return strings.Join(parts, ".")
}

// Hash returns the md5 hash of the String representation of this version.
func (vc WAVersionContainer) Hash() [16]byte {
	return md5.Sum([]byte(vc.String()))
}

func (vc WAVersionContainer) ProtoAppVersion() *waWa6.ClientPayload_UserAgent_AppVersion {
	return &waWa6.ClientPayload_UserAgent_AppVersion{
		Primary:   &vc[0],
		Secondary: &vc[1],
		Tertiary:  &vc[2],
	}
}

// waVersion is the WhatsApp web client version
var waVersion = WAVersionContainer{2, 3000, 1040098269}

// waVersionHash is the md5 hash of a dot-separated waVersion
var waVersionHash = waVersion.Hash()

// GetWAVersion gets the current WhatsApp web client version.
func GetWAVersion() WAVersionContainer {
	return waVersion
}

// SetWAVersion sets the current WhatsApp web client version.
//
// In general, you should keep the library up-to-date instead of using this,
// as there may be code changes that are necessary too (like protobuf schema changes).
func SetWAVersion(version WAVersionContainer) {
	if version.IsZero() {
		return
	}
	waVersion = version
	waVersionHash = version.Hash()
	BaseClientPayload.UserAgent.AppVersion = waVersion.ProtoAppVersion()
}

var BaseClientPayload = &waWa6.ClientPayload{
	// 5
	UserAgent: &waWa6.ClientPayload_UserAgent{
		Platform:       waWa6.ClientPayload_UserAgent_WEB.Enum(),
		AppVersion:     waVersion.ProtoAppVersion(),
		Mcc:            proto.String("000"),
		Mnc:            proto.String("000"),
		OsVersion:      proto.String("0.1"),
		Manufacturer:   proto.String(""),
		Device:         proto.String("Desktop"),
		OsBuildNumber:  proto.String("0.1"),
		ReleaseChannel: waWa6.ClientPayload_UserAgent_RELEASE.Enum(),

		LocaleLanguageIso6391:       proto.String("en"),
		LocaleCountryIso31661Alpha2: proto.String("US"),
	},
	// 6
	WebInfo: &waWa6.ClientPayload_WebInfo{
		WebSubPlatform: waWa6.ClientPayload_WebInfo_WEB_BROWSER.Enum(),
	},
	// 12
	ConnectType: waWa6.ClientPayload_WIFI_UNKNOWN.Enum(),
	// 13
	ConnectReason: waWa6.ClientPayload_USER_ACTIVATED.Enum(),
}

var DeviceProps = &waCompanionReg.DeviceProps{
	// 1
	Os: proto.String("Windows"),
	// 2
	Version: &waCompanionReg.DeviceProps_AppVersion{
		Primary: proto.Uint32(10),
	},
	// 3
	//PlatformType: waCompanionReg.DeviceProps_CHROME.Enum(),
	// 改成桌面版
	PlatformType: waCompanionReg.DeviceProps_UWP.Enum(),
	// 4
	RequireFullSync: proto.Bool(false),
	HistorySyncConfig: &waCompanionReg.DeviceProps_HistorySyncConfig{
		// 3
		// 指定客户端可用于存储同步历史的空间上限（单位 MB）
		StorageQuotaMb: proto.Uint32(10240),
		// 4
		// 是否在首次端到端加密消息中内联包含初始同步数据
		InlineInitialPayloadInE2EeMsg: proto.Bool(true),
		// 是否支持同步通话记录历史
		// 6
		SupportCallLogHistory: proto.Bool(false),
		// 是否支持同步Bot 用户代理会话历史
		// 7
		SupportBotUserAgentChatHistory: proto.Bool(true), // 0
		// 是否支持同步消息反应 (Reactions) 和 投票 (Polls)
		// 8
		SupportCagReactionsAndPolls: proto.Bool(true),
		// 是否支持同步 Biz Hosted Message
		// 9
		SupportBizHostedMsg: proto.Bool(true),
		// 是否支持 Recent Sync Chunk 动态消息数量调整
		// 10
		SupportRecentSyncChunkMessageCountTuning: proto.Bool(true),
		// 是否支持 Hosted Group Message 同步这些群聊的消息
		// 11
		SupportHostedGroupMsg: proto.Bool(true),
		// 是否支持同步 FBID Bot 聊天记录
		// 12
		SupportFbidBotChatHistory: proto.Bool(true), // 0
		// 是否支持 Message Association 元数据同步
		// 14
		SupportMessageAssociation: proto.Bool(true),
		// 是否支持同步 群组历史消息
		// 15
		SupportGroupHistory: proto.Bool(false),
		// 缩略图同步天数限制
		// 19
		ThumbnailSyncDaysLimit: proto.Uint32(60),
		// 21
		SupportManusHistory: proto.Bool(true),
		// 22
		SupportHatchHistory: proto.Bool(true),
	},
}

func SetOSInfo(name string, version [3]uint32) {
	DeviceProps.Os = &name
	DeviceProps.Version.Primary = &version[0]
	DeviceProps.Version.Secondary = &version[1]
	DeviceProps.Version.Tertiary = &version[2]
	BaseClientPayload.UserAgent.OsVersion = proto.String(fmt.Sprintf("%d.%d.%d", version[0], version[1], version[2]))
	BaseClientPayload.UserAgent.OsBuildNumber = BaseClientPayload.UserAgent.OsVersion
}

func (device *Device) getRegistrationPayload() *waWa6.ClientPayload {
	payload := proto.Clone(BaseClientPayload).(*waWa6.ClientPayload)

	// 3
	payload.Passive = proto.Bool(false)

	regID := make([]byte, 4)
	binary.BigEndian.PutUint32(regID, device.RegistrationID)

	preKeyID := make([]byte, 4)
	binary.BigEndian.PutUint32(preKeyID, device.SignedPreKey.KeyID)

	deviceProps, _ := proto.Marshal(DeviceProps)
	// 19
	payload.DevicePairingData = &waWa6.ClientPayload_DevicePairingRegistrationData{
		// 1
		ERegid: regID,
		// 2
		EKeytype: []byte{ecc.DjbType},
		// 3
		EIdent: device.IdentityKey.Pub[:],
		// 4
		ESkeyID: preKeyID[1:],
		// 5
		ESkeyVal: device.SignedPreKey.Pub[:],
		// 6
		ESkeySig: device.SignedPreKey.Signature[:],
		// 7
		BuildHash: waVersionHash[:],
		// 8
		DeviceProps: deviceProps,
	}
	// 33
	payload.Pull = proto.Bool(false)

	// protobuf序列化
	data, err := proto.Marshal(payload)
	if err != nil {
		fmt.Printf("marshal payload failed: %v\n", err)
		return payload
	}
	fmt.Printf("getRegistrationPayload payload hex: %s\n", hex.EncodeToString(data))

	return payload
}

func (device *Device) getLoginPayload() *waWa6.ClientPayload {
	// 注意逻辑：成功关联绑定之前，Passive=true，Lc=0，
	// 后续登录：Passive=false，Lc就不会是累加的
	var passive bool
	var lc int32
	// 这里就按是否拿到routingInfo来判断
	if device.RoutingInfo == "" {
		passive = true
		lc = 0
	} else {
		passive = false
		lc = 1
	}

	payload := proto.Clone(BaseClientPayload).(*waWa6.ClientPayload)
	// 1
	payload.Username = proto.Uint64(device.ID.UserInt())
	// 3	这里要设置为false，如果为true有时候服务器会缺少一些同步
	payload.Passive = proto.Bool(passive)
	// 16
	// web：connectAttemptCount: u,
	// 连接尝试次数，每次登录初始都为0，每次尝试连接+1，直到登录成功。	这里直接为0
	payload.ConnectAttemptCount = proto.Uint32(0)
	// 18
	payload.Device = proto.Uint32(uint32(device.ID.Device))
	// 24
	// web：关键字o("WAWebUserPrefsGeneral").getLoginCounter()
	// 登录次数，var maxInt32 = Math.pow(2, 31) - 1; 会判断如果大于这个值则重置为0，否则每次登录都递增+1
	payload.Lc = proto.Int32(lc)
	// 33
	payload.Pull = proto.Bool(true)
	// 41	是否迁移为lid
	payload.LidDbMigrated = proto.Bool(true)

	// protobuf序列化
	data, err := proto.Marshal(payload)
	if err != nil {
		fmt.Printf("marshal payload failed: %v\n", err)
		return payload
	}
	fmt.Printf("getLoginPayload payload hex: %s\n", hex.EncodeToString(data))

	return payload
}

func (device *Device) GetClientPayload() *waWa6.ClientPayload {
	if device.ID != nil {
		if *device.ID == types.EmptyJID {
			panic(fmt.Errorf("GetClientPayload called with empty JID"))
		}
		return device.getLoginPayload()
	} else {
		return device.getRegistrationPayload()
	}
}
