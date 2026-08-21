package mobilereg

import (
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const DefaultHost = "v.whatsapp.net"

type Client struct {
	HTTPClient *http.Client
	Host       string
	UserAgent  string

	// DebugPlaintext is called with the ordered plaintext query just before it
	// is encrypted into ENC. It is intended for local protocol debugging only.
	DebugPlaintext func(endpoint string, query string, params Params)

	// ExtraHeaders are copied into each request after the default mobile
	// registration headers are set.
	ExtraHeaders http.Header

	// StrictParams rejects Params keys not present in WhatsApp's known mobile
	// registration parameter order. Defaults to true.
	StrictParams bool

	TokenProvider TokenProvider
}

func NewClient(userAgent string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		HTTPClient:   httpClient,
		Host:         DefaultHost,
		UserAgent:    userAgent,
		StrictParams: true,
	}
}

func AndroidUserAgent(isBusiness bool, version, androidVersion, brand, model string) string {
	app := "Android"
	if isBusiness {
		app = "SMBA"
	}
	model = strings.ReplaceAll(model, " ", "_")
	return fmt.Sprintf("WhatsApp/%s %s/%s Device/%s-%s", version, app, androidVersion, brand, model)
}

func (c *Client) Exist(ctx context.Context, req ExistRequest) (*Response, error) {
	params, err := c.existParams(req)
	if err != nil {
		return nil, err
	}
	return c.doEncryptedGet(ctx, "/v2/exist", params)
}

func (c *Client) Code(ctx context.Context, req CodeRequest) (*Response, error) {
	params, err := c.codeParams(req)
	if err != nil {
		return nil, err
	}
	return c.doEncryptedGet(ctx, "/v2/code", params)
}

func (c *Client) Register(ctx context.Context, req RegisterRequest) (*Response, error) {
	params := c.registerParams(req)
	return c.doEncryptedGet(ctx, "/v2/register", params)
}

func (c *Client) Security(ctx context.Context, req SecurityRequest) (*Response, error) {
	params := c.securityParams(req)
	return c.doEncryptedGet(ctx, "/v2/security", params)
}

func (c *Client) existParams(req ExistRequest) (Params, error) {
	p := Params{}
	if req.ReadPhonePermissionGranted != "" {
		p["read_phone_permission_granted"] = req.ReadPhonePermissionGranted
	} else {
		p["read_phone_permission_granted"] = "0"
	}
	p.setIfNotEmpty("lc", req.Device.Locale)
	p.setIfNotEmpty("offline_ab", req.OfflineAB)
	if req.Phone != "" {
		p["in"] = req.Phone
	} else {
		p.setIfNotEmpty("in", req.Device.Phone)
	}
	p.setIfNotEmpty("backup_token", req.Device.BackupToken)
	p.setIfNotEmpty("lg", req.Device.Language)
	setKeyParams(p, req.Device)
	setCommonParams(p, req.Common)
	if token, err := c.token(req.Device); err != nil {
		return nil, err
	} else if token != "" {
		p["token"] = token
	}
	p.setIfNotEmpty("sim_state", req.SIMState)
	if req.CountryCode != "" {
		p["cc"] = req.CountryCode
	} else {
		p.setIfNotEmpty("cc", req.Device.CountryCode)
	}
	p.setIfNotEmpty("network_operator_name", req.Device.NetworkOperatorName)
	p.setIfNotEmpty("sim_operator_name", req.Device.SIMOperatorName)
	applyOverrides(p, req.Params)
	return p, nil
}

func (c *Client) codeParams(req CodeRequest) (Params, error) {
	p := Params{"reason": ""}
	p.setIfNotEmpty("method", req.Method)
	p.setIfNotEmpty("lc", req.Device.Locale)
	p.setIfNotEmpty("in", req.Device.Phone)
	p.setIfNotEmpty("backup_token", req.Device.BackupToken)
	p.setIfNotEmpty("lg", req.Device.Language)
	setKeyParams(p, req.Device)
	setCommonParams(p, req.Common)
	p.setIfNotEmpty("hasav", req.HasAV)
	if token, err := c.token(req.Device); err != nil {
		return nil, err
	} else if token != "" {
		p["token"] = token
	}
	p.setIfNotEmpty("sim_mcc", req.Device.SIMMCC)
	p.setIfNotEmpty("cc", req.Device.CountryCode)
	p.setIfNotEmpty("mnc", req.Device.MNC)
	p.setIfNotEmpty("sim_mnc", req.Device.SIMMNC)
	p.setIfNotEmpty("mcc", req.Device.MCC)
	applyOverrides(p, req.Params)
	return p, nil
}

func (c *Client) registerParams(req RegisterRequest) Params {
	p := Params{}
	p.setIfNotEmpty("lc", req.Device.Locale)
	p.setIfNotEmpty("in", req.Device.Phone)
	p.setIfNotEmpty("backup_token", req.Device.BackupToken)
	p.setIfNotEmpty("lg", req.Device.Language)
	setKeyParams(p, req.Device)
	setCommonParams(p, req.Common)
	p.setIfNotEmpty("sim_mcc", req.Device.SIMMCC)
	p.setIfNotEmpty("entered", req.Entered)
	p.setIfNotEmpty("cc", req.Device.CountryCode)
	p.setIfNotEmpty("mnc", req.Device.MNC)
	p.setIfNotEmpty("sim_mnc", req.Device.SIMMNC)
	p.setIfNotEmpty("network_operator_name", req.Device.NetworkOperatorName)
	p.setIfNotEmpty("sim_operator_name", req.Device.SIMOperatorName)
	p.setIfNotEmpty("mcc", req.Device.MCC)
	p.setBytesRawURL("vname", req.VName)
	p.setIfNotEmpty("code", req.Code)
	applyOverrides(p, req.Params)
	return p
}

func (c *Client) securityParams(req SecurityRequest) Params {
	p := Params{}
	p.setIfNotEmpty("lc", req.Device.Locale)
	p.setIfNotEmpty("in", req.Device.Phone)
	p.setIfNotEmpty("backup_token", req.Device.BackupToken)
	p.setIfNotEmpty("lg", req.Device.Language)
	setKeyParams(p, req.Device)
	setCommonParams(p, req.Common)
	p.setIfNotEmpty("cc", req.Device.CountryCode)
	p.setBytesRawURL("vname", req.Device.VNameCert)
	p.setIfNotEmpty("code", req.Code)
	applyOverrides(p, req.Params)
	return p
}

func setKeyParams(p Params, dev Device) {
	if dev.RegistrationID != 0 {
		p["e_regid"] = base64RawURL(uint32BE(dev.RegistrationID))
	}
	p.setIfNotEmpty("id", dev.RecoveryToken)
	p.setBytesRawURL("authkey", dev.StaticPublicKey)
	p.setBytesRawURL("e_skey_sig", dev.SignedPreKeySignature)
	p.setBytesRawURL("expid", dev.ExpID)
	p.setBytesRawURL("e_ident", dev.IdentityPublicKey)
	p["e_skey_id"] = base64RawURL(uint32BE(dev.SignedPreKeyID)[1:])
	p.setIfNotEmpty("fdid", dev.FDID)
	p.setBytesRawURL("e_skey_val", dev.SignedPreKeyPublic)
	p["e_keytype"] = base64RawURL([]byte{5})
}

func setCommonParams(p Params, common Common) {
	p.setIfNotEmpty("mistyped", common.Mistyped)
	p.setIfNotEmpty("rc", common.RC)
	p.setIfNotEmpty("simnum", common.SIMNum)
	p.setIfNotEmpty("client_metrics", common.ClientMetrics)
	p.setIfNotEmpty("hasinrc", common.HasInRC)
	p.setIfNotEmpty("network_radio_type", common.NetworkRadioType)
	p.setIfNotEmpty("pid", common.PID)
}

func applyOverrides(dst Params, src Params) {
	for key, val := range src {
		dst[key] = val
	}
}

func (c *Client) token(dev Device) (string, error) {
	if dev.Token != "" {
		return dev.Token, nil
	}
	if c.TokenProvider != nil {
		return c.TokenProvider(dev)
	}
	return CalculateRegistrationToken(dev.Phone)
}

func (c *Client) doEncryptedGet(ctx context.Context, endpoint string, params Params) (*Response, error) {
	if c == nil {
		return nil, fmt.Errorf("nil mobilereg Client")
	}
	host := c.Host
	if host == "" {
		host = DefaultHost
	}
	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	query, err := buildOrderedQuery(params, c.StrictParams)
	if err != nil {
		return nil, err
	}
	if c.DebugPlaintext != nil {
		c.DebugPlaintext(endpoint, query, params.Clone())
	}
	enc, err := encryptRegistrationQuery(query)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+host+endpoint+"?ENC="+enc, nil)
	if err != nil {
		return nil, err
	}
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	req.Header.Set("Accept", "text/json")
	req.Header.Set("WaMsysRequest", "1")
	req.Header.Set("request_token", newUUID4())
	req.Header.Set("Host", host)
	req.Header.Set("Connection", "Keep-Alive")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	//req.Header.Set("Accept-Encoding", "gzip")
	for key, values := range c.ExtraHeaders {
		for _, val := range values {
			req.Header.Add(key, val)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	reader := resp.Body
	if strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip") {
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("open gzip registration response: %w", err)
		}
		defer gz.Close()
		reader = gz
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	out := &Response{StatusCode: resp.StatusCode, Header: resp.Header.Clone(), Body: body}
	if len(body) != 0 {
		_ = json.Unmarshal(body, &out.JSON)
	}
	return out, nil
}

func newUUID4() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "00000000-0000-4000-8000-000000000000"
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
