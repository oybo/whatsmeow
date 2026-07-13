package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"go.mau.fi/whatsmeow/store"
	"time"
)

const getPhoneDeviceQuery = `
SELECT jid, lid, registration_id, noise_key, identity_key,
       signed_pre_key, signed_pre_key_id, signed_pre_key_sig,
       adv_key, adv_details, adv_account_sig, adv_account_sig_key, adv_device_sig,
       platform, business_name, push_name, facebook_uuid, lid_migration_ts
FROM whatsmeow_device
WHERE jid LIKE $1
LIMIT 1
`

// find all devices in the database by phone.
func (c *Container) GetPhoneDevice(ctx context.Context, phoneNumber string) (*store.Device, error) {
	sess, err := c.scanDevice(c.db.QueryRow(ctx, getPhoneDeviceQuery, phoneNumber+"%"))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return sess, err
}

// ------ routingInfo  ------

const getRoutingInfoQuery = `
SELECT routing_info
FROM whatsmeow_device
WHERE jid LIKE $1
LIMIT 1
`

func (c *Container) GetRoutingInfo(ctx context.Context, phoneNumber string) string {
	var routingInfo string
	err := c.db.QueryRow(ctx, getRoutingInfoQuery, phoneNumber+":%").Scan(&routingInfo)
	if err != nil {
		return ""
	}
	return routingInfo
}

const updateRoutingInfoQuery = `
UPDATE whatsmeow_device
SET routing_info = $1
WHERE jid LIKE $2
`

func (c *Container) PutRoutingInfo(ctx context.Context, phoneNumber string, routingInfo string) error {
	_, err := c.db.Exec(ctx, updateRoutingInfoQuery, routingInfo, phoneNumber+":%")
	return err
}

// ------ 静态公钥等  ------

const updateServerStaticPubQuery = `
UPDATE whatsmeow_device
SET server_static_pub = $1, certificate_chain = $2, cert_expires_at = $3
WHERE jid LIKE $4
`

func (c *Container) PutServerStaticInfo(ctx context.Context, phoneNumber string, pub [32]byte, cert []byte, exp time.Time) error {
	_, err := c.db.Exec(ctx, updateServerStaticPubQuery, pub[:], cert, exp.Unix(), phoneNumber+":%")
	return err
}

const getServerStaticPubQuery = `
SELECT server_static_pub, certificate_chain, cert_expires_at
FROM whatsmeow_device
WHERE jid LIKE $1
LIMIT 1
`

func (c *Container) GetServerStaticInfo(ctx context.Context, phoneNumber string) ([32]byte, []byte, time.Time, error) {
	var pub [32]byte
	var rawPub []byte // 🌟 新增：专门用来对齐数据库驱动的临时切片
	var cert []byte
	var exp int64

	// 1. Scan 时，把第一个参数换成 &rawPub
	err := c.db.QueryRow(ctx, getServerStaticPubQuery, phoneNumber+":%").Scan(&rawPub, &cert, &exp)
	if err != nil {
		return pub, nil, time.Time{}, err
	}

	// 2. 严格校验捞出来的公钥长度，防止数据库数据受损
	if len(rawPub) != 32 {
		return pub, nil, time.Time{}, fmt.Errorf("database returned invalid server_static_pub length: %d (expected 32)", len(rawPub))
	}

	// 3. 将切片中的数据深度复制到固定长度的 [32]byte 数组中
	copy(pub[:], rawPub)

	// 4. 完美返回
	return pub, cert, time.Unix(exp, 0), nil
}

// ------ clientpayload lc  ------

const getLcQuery = `
SELECT lc
FROM whatsmeow_device
WHERE jid LIKE $1
`

// GetLoginLc 根据账号的 jid 查询当前的登录次数
func (c *Container) GetLoginLc(ctx context.Context, phoneNumber string) int32 {
	var lc int32
	err := c.db.QueryRow(ctx, getLcQuery, phoneNumber+":%").Scan(&lc)
	if err != nil {
		// 如果查询失败（比如新号还没记录），默认返回 0
		return 0
	}
	return lc
}

const updateLcQuery = `
UPDATE whatsmeow_device
SET lc = $1
WHERE jid LIKE $2
`

// PutLoginLc 更新指定 jid 账号的登录次数
func (c *Container) PutLoginLc(ctx context.Context, phoneNumber string, lc int32) error {
	_, err := c.db.Exec(ctx, updateLcQuery, lc, phoneNumber+":%")
	return err
}
