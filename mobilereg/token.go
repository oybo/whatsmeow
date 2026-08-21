package mobilereg

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	defaultRegistrationTokenKeyHex = "b1dbc985d769719fadb8d7de68a004852f8bdccd6b83b8c2de46b8f6e28a29b3a78209ee3768140192e5848b676a0544eddd123ce4cf3e995f18ea3077adb8d1"

	defaultRegistrationTokenCertHex = `
30820332308202f0a00302010202044c2536a4300b06072a8648ce3804030500307c310b3009060355040613025553311330110603550408130a43616c69666f726e6961311430120603550407130b53616e746120436c61726131163014060355040a130d576861747341707020496e632e31143012060355040b130b456e67696e656572696e67311430120603550403130b427269616e204163746f6e301e170d3130303632353233303731365a170d3434303231353233303731365a307c310b3009060355040613025553311330110603550408130a43616c69666f726e6961311430120603550407130b53616e746120436c61726131163014060355040a130d576861747341707020496e632e31143012060355040b130b456e67696e656572696e67311430120603550403130b427269616e204163746f6e308201b83082012c06072a8648ce3804013082011f02818100fd7f53811d75122952df4a9c2eece4e7f611b7523cef4400c31e3f80b6512669455d402251fb593d8d58fabfc5f5ba30f6cb9b556cd7813b801d346ff26660b76b9950a5a49f9fe8047b1022c24fbba9d7feb7c61bf83b57e7c6a8a6150f04fb83f6d3c51ec3023554135a169132f675f3ae2b61d72aeff22203199dd14801c70215009760508f15230bccb292b982a2eb840bf0581cf502818100f7e1a085d69b3ddecbbcab5c36b857b97994afbbfa3aea82f9574c0b3d0782675159578ebad4594fe67107108180b449167123e84c281613b7cf09328cc8a6e13c167a8b547c8d28e0a3ae1e2bb3a675916ea37f0bfa213562f1fb627a01243bcca4f1bea8519089a883dfe15ae59f06928b665e807b552564014c3bfecf492a0381850002818100d1198b4b81687bcf246d41a8a725f0a989a51bce326e84c828e1f556648bd71da487054d6de70fff4b49432b6862aa48fc2a93161b2c15a2ff5e671672dfb576e9d12aaff7369b9a99d04fb29d2bbbb2a503ee41b1ff37887064f41fe2805609063500a8e547349282d15981cdb58a08bede51dd7e9867295b3dfb45ffc6b259300b06072a8648ce3804030500032f00302c021400a602a7477acf841077237be090df436582ca2f0214350ce0268d07e71e55774ab4eacd4d071cd1efad
`

	defaultRegistrationTokenClassesDexMD5Hex = "fb96836b3bdad9bc3bda71b76d37f032"
)

// CalculateRegistrationToken calculates the WhatsApp Android registration token
// used by /v2/exist and /v2/code for the embedded app signature/classes.dex
// constants. The phone argument should match the "in" registration parameter.
func CalculateRegistrationToken(phone string) (string, error) {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return "", fmt.Errorf("phone is empty")
	}
	return calculateRegistrationTokenWithMaterial(
		phone,
		defaultRegistrationTokenKeyHex,
		defaultRegistrationTokenCertHex,
		defaultRegistrationTokenClassesDexMD5Hex,
	)
}

func calculateRegistrationTokenWithMaterial(phone, keyHex, certHex, classesDexMD5Hex string) (string, error) {
	key, err := decodeTokenHex(keyHex, "registration token key")
	if err != nil {
		return "", err
	}
	cert, err := decodeTokenHex(certHex, "registration token certificate")
	if err != nil {
		return "", err
	}
	classesDexMD5, err := decodeTokenHex(classesDexMD5Hex, "registration token classes.dex MD5")
	if err != nil {
		return "", err
	}

	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(cert)
	_, _ = mac.Write(classesDexMD5)
	_, _ = mac.Write([]byte(phone))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil)), nil
}

func decodeTokenHex(input, name string) ([]byte, error) {
	decoded, err := hex.DecodeString(strings.Join(strings.Fields(input), ""))
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", name, err)
	}
	return decoded, nil
}
