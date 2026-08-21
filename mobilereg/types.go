package mobilereg

import (
	"fmt"
	"net/http"
)

// Params contains plaintext registration parameters before they are ordered,
// URL-escaped, encrypted, and sent as the ENC query parameter.
type Params map[string]string

// Clone returns a copy of the parameter map.
func (p Params) Clone() Params {
	if len(p) == 0 {
		return Params{}
	}
	out := make(Params, len(p))
	for key, val := range p {
		out[key] = val
	}
	return out
}

func (p Params) setIfNotEmpty(key, val string) {
	if val != "" {
		p[key] = val
	}
}

func (p Params) setBytesRawURL(key string, val []byte) {
	if len(val) != 0 {
		p[key] = base64RawURL(val)
	}
}

// Device contains the Android/mobile device values used by registration
// endpoints. The string fields intentionally remain opaque: the original mobile
// protocol sends some values as raw strings and some as encoded strings.
type Device struct {
	CountryCode string
	Phone       string
	Locale      string
	Language    string

	BackupToken    string
	RecoveryToken  string
	RegistrationID uint32

	StaticPublicKey       []byte
	SignedPreKeyID        uint32
	SignedPreKeySignature []byte
	SignedPreKeyPublic    []byte
	IdentityPublicKey     []byte
	ExpID                 []byte

	FDID string

	MCC    string
	MNC    string
	SIMMCC string
	SIMMNC string

	NetworkOperatorName string
	SIMOperatorName     string

	// Token is the standard-base64 registration token used by /v2/exist and
	// /v2/code. If empty, Client.TokenProvider or the built-in Android token
	// calculator may provide it.
	Token string

	// VNameCert is used by /v2/security for WhatsApp Business flows when the
	// server asks for a security code after /v2/register.
	VNameCert []byte
}

// Common contains user/session metrics that appear across multiple endpoints.
type Common struct {
	Mistyped         string
	RC               string
	SIMNum           string
	ClientMetrics    string
	HasInRC          string
	NetworkRadioType string
	PID              string
}

// BaseRequest is embedded by endpoint-specific request types.
type BaseRequest struct {
	Device Device
	Common Common

	// Params can override or add values after the built-in endpoint parameters
	// are populated. Unknown parameter names are rejected unless Client.StrictParams
	// is false.
	Params Params
}

type ExistRequest struct {
	BaseRequest

	ReadPhonePermissionGranted string
	OfflineAB                  string
	Phone                      string
	CountryCode                string
	SIMState                   string
}

type CodeRequest struct {
	BaseRequest

	Method string
	HasAV  string
}

type RegisterRequest struct {
	BaseRequest

	Entered string
	Code    string
	VName   []byte
}

type SecurityRequest struct {
	BaseRequest

	Code string
}

// Response is returned by all registration endpoint methods.
type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
	JSON       map[string]any
}

// TokenProvider can be used to override the built-in mobile registration token
// calculator.
type TokenProvider func(Device) (string, error)

type UnknownParamError struct {
	Key string
}

func (upe UnknownParamError) Error() string {
	return fmt.Sprintf("unknown WhatsApp registration parameter %q", upe.Key)
}
