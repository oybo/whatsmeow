package mobilereg

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/crypto/curve25519"
)

var serverPublicKey = mustHex("8e8c0f74c3ebc5d7a6865c6c3c843856b06121cce8ea774d22fb6f122512302d")
var zeroRegistrationIV = make([]byte, 12)

// paramOrder is copied from the Android registration flow. WhatsApp expects
// these query parameters to be ordered before the full query string is encrypted.
var paramOrder = []string{
	"reason",
	"method",
	"e_regid",
	"e_skey_sig",
	"e_ident",
	"e_skey_val",
	"rc",
	"pid",
	"in",
	"expid",
	"mistyped",
	"client_metrics",
	"lc",
	"authkey",
	"simnum",
	"hasinrc",
	"hasav",
	"token",
	"lg",
	"e_skey_id",
	"fdid",
	"e_keytype",
	"network_radio_type",
	"cc",
	"read_phone_permission_granted",
	"offline_ab",
	"backup_token",
	"id",
	"action_taken",
	"previous_screen",
	"sim_mcc",
	"entered",
	"sim_state",
	"mnc",
	"sim_mnc",
	"funnel_id",
	"mcc",
	"network_operator_name",
	"sim_operator_name",
	"code",
	"current_screen",
	"vname",
}

func mustHex(input string) []byte {
	out, err := hex.DecodeString(input)
	if err != nil {
		panic(err)
	}
	return out
}

func base64RawURL(input []byte) string {
	return base64.RawURLEncoding.EncodeToString(input)
}

func uint32BE(input uint32) []byte {
	var out [4]byte
	binary.BigEndian.PutUint32(out[:], input)
	return out[:]
}

func buildOrderedQuery(params Params, strict bool) (string, error) {
	parts := make([]string, 0, len(params))
	used := make(map[string]struct{}, len(params))
	for _, key := range paramOrder {
		if val, ok := params[key]; ok {
			parts = append(parts, key+"="+url.QueryEscape(val))
			used[key] = struct{}{}
		}
	}
	if len(used) != len(params) {
		for key := range params {
			if _, ok := used[key]; !ok {
				if strict {
					return "", UnknownParamError{Key: key}
				}
				parts = append(parts, key+"="+url.QueryEscape(params[key]))
			}
		}
	}
	return strings.Join(parts, "&"), nil
}

func encryptRegistrationQuery(query string) (string, error) {
	priv := make([]byte, curve25519.ScalarSize)
	if _, err := rand.Read(priv); err != nil {
		return "", fmt.Errorf("generate registration ephemeral key: %w", err)
	}
	priv[0] &= 248
	priv[31] &= 127
	priv[31] |= 64

	pub, err := curve25519.X25519(priv, curve25519.Basepoint)
	if err != nil {
		return "", fmt.Errorf("derive registration public key: %w", err)
	}
	shared, err := curve25519.X25519(priv, serverPublicKey)
	if err != nil {
		return "", fmt.Errorf("derive registration shared secret: %w", err)
	}

	block, err := aes.NewCipher(shared)
	if err != nil {
		return "", fmt.Errorf("create registration AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create registration GCM: %w", err)
	}

	ciphertext := gcm.Seal(nil, zeroRegistrationIV, []byte(query), nil)
	enc := make([]byte, 0, len(pub)+len(ciphertext))
	enc = append(enc, pub...)
	enc = append(enc, ciphertext...)
	return base64RawURL(enc), nil
}
