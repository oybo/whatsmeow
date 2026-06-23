// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"crypto/hmac"
	"encoding/base64"
	"fmt"
	"time"

	"go.mau.fi/libsignal/ecc"
	"google.golang.org/protobuf/proto"

	"go.mau.fi/whatsmeow/proto/waCert"
	"go.mau.fi/whatsmeow/proto/waWa6"
	"go.mau.fi/whatsmeow/socket"
	"go.mau.fi/whatsmeow/util/keys"
)

const NoiseHandshakeResponseTimeout = 20 * time.Second
const WACertIssuerSerial = 0

var WACertPubKey = [...]byte{0x14, 0x23, 0x75, 0x57, 0x4d, 0xa, 0x58, 0x71, 0x66, 0xaa, 0xe7, 0x1e, 0xbe, 0x51, 0x64, 0x37, 0xc4, 0xa2, 0x8b, 0x73, 0xe3, 0x69, 0x5c, 0x6c, 0xe1, 0xf7, 0xf9, 0x54, 0x5d, 0xa8, 0xee, 0x6b}

func (cli *Client) shouldUseIKHandshake() bool {
	// 判断是否可以走 IK 模式，防止证书过期
	isIKValid := false
	if cli.Store.ServerStaticKey != [32]byte{} && len(cli.Store.CertificateChain) > 0 {
		// 设置 2 小时的安全缓冲区  判断当前时间 + 2小时是否超过过期时间
		isIKValid = time.Now().Add(2 * time.Hour).Before(cli.Store.ServerKeyExp)
	}
	return isIKValid
}

func (cli *Client) doConnectHandshake(ctx context.Context, fs *socket.FrameSocket, ephemeralKP keys.KeyPair) error {
	// 判断有没有routingInfo的值，如果有值，建立socket连接后就发送这个ED
	routing_info := cli.Store.RoutingInfo
	if routing_info != "" {
		encoded, _ := base64.RawURLEncoding.DecodeString(routing_info)
		_ = fs.SendFrameOrigin(fs.EncodeRoutingInfo(encoded))
	}

	if cli.shouldUseIKHandshake() {
		fmt.Println("doIKHandshake")
		// 这里第三个参数要传自己本地保存的Noise静态公私钥
		//err := cli.doIKHandshake(ctx, fs, *cli.Store.NoiseKey, cli.Store.ServerStaticKey)
		err := cli.doIKHandshake(ctx, fs, ephemeralKP, cli.Store.ServerStaticKey)
		if err == nil {
			// IK 模式恢复连接成功
			fmt.Println("IK 模式恢复连接成功")
			return nil
		}
	}
	fmt.Println("doXXHandshake")
	return cli.doXXHandshake(ctx, fs, ephemeralKP)
}

// doHandshake implements the Noise_XX_25519_AESGCM_SHA256 handshake for the WhatsApp web API.
func (cli *Client) doHandshake(ctx context.Context, fs *socket.FrameSocket, ephemeralKP keys.KeyPair) error {
	return cli.doXXHandshake(ctx, fs, ephemeralKP)
}

func readHandshakeResponse(fs *socket.FrameSocket) (*waWa6.HandshakeMessage, error) {
	var resp []byte
	select {
	case resp = <-fs.Frames:
	case <-time.After(NoiseHandshakeResponseTimeout):
		return nil, fmt.Errorf("timed out waiting for handshake response")
	}
	var handshakeResponse waWa6.HandshakeMessage
	err := proto.Unmarshal(resp, &handshakeResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal handshake response: %w", err)
	}
	return &handshakeResponse, nil
}

func (cli *Client) getClientPayloadBytes() ([]byte, error) {
	var clientPayload *waWa6.ClientPayload
	if cli.GetClientPayload != nil {
		clientPayload = cli.GetClientPayload()
	} else {
		clientPayload = cli.Store.GetClientPayload()
	}

	clientPayloadBytes, err := proto.Marshal(clientPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal client payload: %w", err)
	}
	return clientPayloadBytes, nil
}

func (cli *Client) saveHandshakeKeys(ctx context.Context) error {
	if cli.Store.ID == nil {
		return nil
	}
	err := cli.Store.Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to save handshake keys: %w", err)
	}
	return nil
}

func (cli *Client) finishHandshake(ctx context.Context, nh *socket.NoiseHandshake, fs *socket.FrameSocket) error {
	ns, err := nh.Finish(ctx, fs, cli.handleFrame, cli.onDisconnect)
	if err != nil {
		return fmt.Errorf("failed to create noise socket: %w", err)
	}

	cli.socket = ns
	return nil
}

// doNoiseXXHandshake implements the Noise_XX_25519_AESGCM_SHA256 handshake for the WhatsApp web API.
// XX 模式：开始时客户端不知道服务器的静态公钥，需要服务器在握手过程中（Server Hello）把自己的静态公钥加密传过来，客户端解密后再进行身份验证。
func (cli *Client) doXXHandshake(ctx context.Context, fs *socket.FrameSocket, ephemeralKP keys.KeyPair) error {
	// 初始化 Noise
	nh := socket.NewNoiseHandshake()

	// 设置握手模式，加密模式GCM AES，并把header数据作为基础校验数据
	nh.Start(socket.NoiseXXStartPattern, fs.Header)

	// 认证客户端公钥	（随机生成的私有，再计算的公钥）
	nh.Authenticate(ephemeralKP.Pub[:])

	// 序列号Client Hello
	data, err := proto.Marshal(&waWa6.HandshakeMessage{
		ClientHello: &waWa6.HandshakeMessage_ClientHello{
			Ephemeral: ephemeralKP.Pub[:],
		},
	})
	if err != nil {
		return fmt.Errorf("failed to marshal handshake message: %w", err)
	}

	// 执行发送
	err = fs.SendFrame(data)

	handshakeResponse, err := readHandshakeResponse(fs)
	if err != nil {
		return fmt.Errorf("failed to unmarshal handshake response: %w", err)
	}

	// 服务器临时公钥
	serverEphemeral := handshakeResponse.GetServerHello().GetEphemeral()
	// 服务器静态公钥（加密的）
	serverStaticCiphertext := handshakeResponse.GetServerHello().GetStatic()
	// 服务器证书（加密的）
	certificateCiphertext := handshakeResponse.GetServerHello().GetPayload()
	if len(serverEphemeral) != 32 || serverStaticCiphertext == nil || certificateCiphertext == nil {
		return fmt.Errorf("missing parts of handshake response")
	}
	// 格式转换，转成go的字节数组
	serverEphemeralArr := *(*[32]byte)(serverEphemeral)
	// 认证服务端公钥
	nh.Authenticate(serverEphemeral)

	// Noise核心：通过 DH 密钥交换算法，混合【本地生成的私钥】 + 【服务器公钥】
	err = nh.MixSharedSecretIntoKey(*ephemeralKP.Priv, serverEphemeralArr)
	if err != nil {
		return fmt.Errorf("failed to mix server ephemeral key in: %w", err)
	}

	// 用刚刚诞生的第一代密钥，解密服务器的静态公钥（真身）
	staticDecrypted, err := nh.Decrypt(serverStaticCiphertext)
	if err != nil {
		return fmt.Errorf("failed to decrypt server static ciphertext: %w", err)
	} else if len(staticDecrypted) != 32 {
		return fmt.Errorf("unexpected length of server static plaintext %d (expected 32)", len(staticDecrypted))
	}

	// 通过 DH 算法，混合【本地生成的私钥】 + 【服务器静态公钥】
	err = nh.MixSharedSecretIntoKey(*ephemeralKP.Priv, *(*[32]byte)(staticDecrypted))
	if err != nil {
		return fmt.Errorf("failed to mix server static key in: %w", err)
	}

	// 用第二代密钥解密服务器传过来的证书
	certDecrypted, err := nh.Decrypt(certificateCiphertext)
	if err != nil {
		return fmt.Errorf("failed to decrypt noise certificate ciphertext: %w", err)
	} else if err = verifyServerCert(certDecrypted, staticDecrypted); err != nil { // 验证证书
		return fmt.Errorf("failed to verify server cert: %w", err)
	}

	// 解析最外层的证书链 (CertChain) 和服务端静态公钥 进行存储
	_ = cli.handleAndSaveServerCert(ctx, certDecrypted, staticDecrypted)

	// 1. 加密我们自己的 Noise 静态公钥（让服务器知道我是哪个设备） -- 这个NoiseKey.Pub就是我们账号存储的公钥
	encryptedPubkey := nh.Encrypt(cli.Store.NoiseKey.Pub[:])

	// 2. 核心：通过 DH 算法，混合【客户端静态私钥】 + 【服务器临时公钥】 -- 这个NoiseKey.Priv就是我们账号存储的私钥
	err = nh.MixSharedSecretIntoKey(*cli.Store.NoiseKey.Priv, serverEphemeralArr)
	if err != nil {
		return fmt.Errorf("failed to mix noise private key in: %w", err)
	}

	// 3. 获取并打包客户端登录负载（包含设备操作系统、版本、登录 Token 等）
	// 序列化
	clientFinishPayloadBytes, err := cli.getClientPayloadBytes()
	if err != nil {
		return fmt.Errorf("failed to marshal client finish payload: %w", err)
	}

	// 4. 用最新的密钥加密这个 Payload
	encryptedClientFinishPayload := nh.Encrypt(clientFinishPayloadBytes)

	// 序列化并发送给服务器
	data, err = proto.Marshal(&waWa6.HandshakeMessage{
		ClientFinish: &waWa6.HandshakeMessage_ClientFinish{
			Static:  encryptedPubkey,              // 加密后的公钥
			Payload: encryptedClientFinishPayload, // 加密后的登录数据
		},
	})
	if err != nil {
		return fmt.Errorf("failed to marshal handshake finish message: %w", err)
	}

	// 执行发送
	err = fs.SendFrame(data)
	if err != nil {
		return fmt.Errorf("failed to send handshake finish message: %w", err)
	}

	// 握手结束
	return cli.finishHandshake(ctx, nh, fs)
}

// HandleAndSaveServerCert 解析并保存服务器证书链信息
func (cli *Client) handleAndSaveServerCert(ctx context.Context, certDecrypted []byte, staticDecrypted []byte) error {
	// 1. 第一层反序列化：解析最外层的证书链 (CertChain)
	var chain waCert.CertChain
	if err := proto.Unmarshal(certDecrypted, &chain); err != nil {
		return fmt.Errorf("failed to unmarshal CertChain: %w", err)
	}

	// 2. 安全检查：确保叶子证书存在
	leaf := chain.GetLeaf()
	if leaf == nil || leaf.GetDetails() == nil {
		return fmt.Errorf("invalid cert chain: missing leaf certificate or details")
	}

	// 3. 第二层反序列化：解析叶子证书内部嵌套的 Details 字节流
	var details waCert.CertChain_NoiseCertificate_Details
	if err := proto.Unmarshal(leaf.GetDetails(), &details); err != nil {
		return fmt.Errorf("failed to unmarshal leaf certificate details: %w", err)
	}

	// 4. 提取过期时间戳 (notAfter)
	// 根据 proto 定义，notAfter 是 uint64，通常是秒级 Unix 时间戳
	var expiresAt time.Time
	if details.GetNotAfter() > 0 {
		expiresAt = time.Unix(int64(details.GetNotAfter()), 0)
	}

	// 5. 转换并打包服务端静态公钥 (serverStaticPub)
	var serverStaticPub [32]byte
	copy(serverStaticPub[:], staticDecrypted)

	// 6. 异步/独立 context 持久化到数据库，防止因网络中断导致 ctx 被 Cancel
	dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 将 公钥、完整的原始证书链字节、计算出的过期时间 一并存入数据库
	errDb := cli.Store.Container.PutServerStaticInfo(dbCtx, serverStaticPub, certDecrypted, expiresAt)
	if errDb != nil {
		return fmt.Errorf("failed to save server static info to database: %w", errDb)
	}
	return nil
}

// doNoiseIKHandshake implements the Noise_IK_25519_AESGCM_SHA256 handshake for the WhatsApp web API.
// IK 模式：客户端在发起握手前，已经提前知道了服务器的静态公钥（通常是硬编码在客户端，或者上一次连接时缓存下来的）。因此，客户端在第一步（Client Hello）就可以直接利用服务器的静态公钥加密数据并进行密钥交换（DH）。
// 注意：IK 模式要求必须传入服务器的静态公钥 serverStaticPub
func (cli *Client) doIKHandshake(ctx context.Context, fs *socket.FrameSocket, ephemeralKP keys.KeyPair, serverStaticKey [32]byte) error {
	// 初始化 Noise
	nh := socket.NewNoiseHandshake()
	nh.Start(socket.NoiseIKStartPattern, fs.Header)

	// 认证服务器静态公钥 v
	nh.Authenticate(serverStaticKey[:])

	// 认证客户端Noise静态公钥
	nh.Authenticate(ephemeralKP.Pub[:])

	// 【EC Agreement 1】混合：客户端临时私钥 + 服务器静态公钥
	err := nh.MixSharedSecretIntoKey(*ephemeralKP.Priv, serverStaticKey)
	if err != nil {
		return fmt.Errorf("failed to mix server static key in: %w", err)
	}

	// 在混入 s,s 之前，先加密客户端静态公钥
	encryptedPubkey := nh.Encrypt(cli.Store.NoiseKey.Pub[:])

	// 混合：客户端静态私钥 + 服务器静态公钥
	err = nh.MixSharedSecretIntoKey(*cli.Store.NoiseKey.Priv, serverStaticKey)
	if err != nil {
		return fmt.Errorf("failed to mix noise private key with server static key: %w", err)
	}

	clientPayloadBytes, err := cli.getClientPayloadBytes()
	if err != nil {
		return err
	}

	// 用混合了 s,s 后的最新密钥，加密登录负载
	encryptedClientPayload := nh.Encrypt(clientPayloadBytes)

	// 打包序列化并发送 Client Hello
	data, err := proto.Marshal(&waWa6.HandshakeMessage{
		ClientHello: &waWa6.HandshakeMessage_ClientHello{
			Ephemeral: ephemeralKP.Pub[:],
			Static:    encryptedPubkey,
			Payload:   encryptedClientPayload,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to marshal handshake message: %w", err)
	}
	err = fs.SendFrame(data)
	if err != nil {
		return fmt.Errorf("failed to send handshake message: %w", err)
	}

	handshakeResponse, err := readHandshakeResponse(fs)
	if err != nil {
		return err
	}
	serverEphemeral := handshakeResponse.GetServerHello().GetEphemeral()
	if len(serverEphemeral) != 32 {
		return fmt.Errorf("missing server ephemeral key in handshake response")
	}
	serverEphemeralArr := *(*[32]byte)(serverEphemeral)

	// S.authenticate(d) 认证服务器临时公钥
	nh.Authenticate(serverEphemeral)

	// 混合服务器临时公钥 + 客户端临时私钥 (e, e)
	err = nh.MixSharedSecretIntoKey(*ephemeralKP.Priv, serverEphemeralArr)
	if err != nil {
		return fmt.Errorf("failed to mix server ephemeral key in: %w", err)
	}

	// 混合服务器临时公钥 + 客户端静态私钥 (s, e)
	err = nh.MixSharedSecretIntoKey(*cli.Store.NoiseKey.Priv, serverEphemeralArr)
	if err != nil {
		return fmt.Errorf("failed to mix noise private key with server ephemeral key: %w", err)
	}

	serverPayloadCiphertext := handshakeResponse.GetServerHello().GetPayload()
	if serverPayloadCiphertext != nil {
		// 校验证书密钥
		_, err = nh.Decrypt(serverPayloadCiphertext)
		if err != nil {
			cli.Log.Errorf("failed to decrypt server hello payload ciphertext: %w", err)
			return err
		}
	}

	// 14. 结束握手
	return cli.finishHandshake(ctx, nh, fs)
}

func checkCertValidity(cert *waCert.CertChain_NoiseCertificate_Details) error {
	notBefore := time.Unix(int64(cert.GetNotBefore()), 0)
	notAfter := time.Unix(int64(cert.GetNotAfter()), 0)
	now := time.Now()
	if now.Before(notBefore) {
		return fmt.Errorf("certificate not valid yet (current time %s is before %s)", now, notBefore)
	} else if now.After(notAfter) {
		return fmt.Errorf("certificate expired (current time %s is after %s)", now, notAfter)
	}
	return nil
}

func verifyServerCert(certDecrypted, staticDecrypted []byte) error {
	var certChain waCert.CertChain
	err := proto.Unmarshal(certDecrypted, &certChain)
	if err != nil {
		return fmt.Errorf("failed to unmarshal noise certificate: %w", err)
	}
	var intermediateCertDetails, leafCertDetails waCert.CertChain_NoiseCertificate_Details
	intermediateCertDetailsRaw := certChain.GetIntermediate().GetDetails()
	intermediateCertSignature := certChain.GetIntermediate().GetSignature()
	leafCertDetailsRaw := certChain.GetLeaf().GetDetails()
	leafCertSignature := certChain.GetLeaf().GetSignature()
	if intermediateCertDetailsRaw == nil || intermediateCertSignature == nil || leafCertDetailsRaw == nil || leafCertSignature == nil {
		return fmt.Errorf("missing parts of noise certificate")
	} else if len(intermediateCertSignature) != 64 {
		return fmt.Errorf("unexpected length of intermediate cert signature %d (expected 64)", len(intermediateCertSignature))
	} else if len(leafCertSignature) != 64 {
		return fmt.Errorf("unexpected length of leaf cert signature %d (expected 64)", len(leafCertSignature))
	} else if !ecc.VerifySignature(ecc.NewDjbECPublicKey(WACertPubKey), intermediateCertDetailsRaw, [64]byte(intermediateCertSignature)) {
		return fmt.Errorf("failed to verify intermediate cert signature")
	} else if err = proto.Unmarshal(intermediateCertDetailsRaw, &intermediateCertDetails); err != nil {
		return fmt.Errorf("failed to unmarshal noise certificate details: %w", err)
	} else if intermediateCertDetails.GetIssuerSerial() != WACertIssuerSerial {
		return fmt.Errorf("unexpected intermediate issuer serial %d (expected %d)", intermediateCertDetails.GetIssuerSerial(), WACertIssuerSerial)
	} else if len(intermediateCertDetails.GetKey()) != 32 {
		return fmt.Errorf("unexpected length of intermediate cert key %d (expected 32)", len(intermediateCertDetails.GetKey()))
	} else if !ecc.VerifySignature(ecc.NewDjbECPublicKey([32]byte(intermediateCertDetails.GetKey())), leafCertDetailsRaw, [64]byte(leafCertSignature)) {
		return fmt.Errorf("failed to verify intermediate cert signature")
	} else if err = checkCertValidity(&intermediateCertDetails); err != nil {
		return fmt.Errorf("intermediate cert %w", err)
	} else if err = proto.Unmarshal(leafCertDetailsRaw, &leafCertDetails); err != nil {
		return fmt.Errorf("failed to unmarshal noise certificate details: %w", err)
	} else if leafCertDetails.GetIssuerSerial() != intermediateCertDetails.GetSerial() {
		return fmt.Errorf("unexpected leaf issuer serial %d (expected %d)", leafCertDetails.GetIssuerSerial(), intermediateCertDetails.GetSerial())
	} else if !hmac.Equal(leafCertDetails.GetKey(), staticDecrypted) {
		return fmt.Errorf("cert key doesn't match decrypted static")
	} else if err = checkCertValidity(&leafCertDetails); err != nil {
		return fmt.Errorf("leaf cert cert %w", err)
	}
	return nil
}
