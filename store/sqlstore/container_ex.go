package sqlstore

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"go.mau.fi/whatsmeow/store"
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
	sess, err := c.scanDevice(c.db.QueryRow(ctx, getPhoneDeviceQuery, phoneNumber+":%"))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return sess, err
}

func GetStaticKey(inputPubBase64 string, inputPrivBase64 string) (*[32]byte, *[32]byte, error) {
	pubBytes, err := base64.StdEncoding.DecodeString(inputPubBase64)
	if err != nil {
		return nil, nil, err
	}
	privBytes, err := base64.StdEncoding.DecodeString(inputPrivBase64)
	if err != nil {
		return nil, nil, err
	}
	if len(pubBytes) != 32 || len(privBytes) != 32 {
		return nil, nil, err
	}

	// 将解码后的数据放入 whatsmeow 要求的 [32]byte 数组中
	var noisePubArr [32]byte
	var noisePrivArr [32]byte
	copy(noisePubArr[:], pubBytes)
	copy(noisePrivArr[:], privBytes)

	return &noisePubArr, &noisePrivArr, nil
}
