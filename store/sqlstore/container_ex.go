package sqlstore

import (
	"context"
	"database/sql"
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
