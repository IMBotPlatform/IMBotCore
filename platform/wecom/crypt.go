package wecom

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"sort"
	"strings"
)

const padBlockSize = 32

var (
	// ErrInvalidSignature 在签名校验失败时返回。
	ErrInvalidSignature = errors.New("invalid signature")
	// ErrInvalidAESKey 当 AESKey 长度不符合规范时返回。
	ErrInvalidAESKey = errors.New("invalid aes key length")
)

// Crypt 封装企业微信的加解密逻辑。
type Crypt struct {
	token  string
	aesKey []byte
	corpID string
}

// NewCrypt 创建一个新的 Crypt 实例，使用企业微信提供的 token、加密密钥与 corpID 作为上下文。
// Parameters:
//   - token: 企业微信配置的消息校验 Token
//   - encodingAESKey: 企业微信后台生成的 43 字节 Base64 编码字符串
//   - corpID: 企业 ID，用于校验消息归属
//
// Returns:
//   - *Crypt: 成功时的加解密器实例
//   - error: 当 EncodingAESKey 无法解码或长度不合法时返回错误
func NewCrypt(token, encodingAESKey, corpID string) (*Crypt, error) {
	key, err := decodeAESKey(encodingAESKey)
	if err != nil {
		return nil, err
	}
	return &Crypt{
		token:  token,
		aesKey: key,
		corpID: corpID,
	}, nil
}

// VerifyURL 用于处理企业微信的 GET 验证，返回解密后的明文。
// Parameters:
//   - msgSignature: 企业微信回调签名
//   - timestamp: 时间戳字符串
//   - nonce: 随机串
//   - echoStr: 回调中携带的加密字符串
//
// Returns:
//   - string: 解密后的明文
//   - error: 签名不匹配、解密失败时返回错误
//
// 流程图：
//
//	[收到URL参数]
//	     |
//	     v
//	[校验签名] --否--> [返回ErrInvalidSignature]
//	     |
//	    是
//	     |
//	     v
//	[解密echoStr]
//	     |
//	     v
//	[返回明文]

func (c *Crypt) VerifyURL(msgSignature, timestamp, nonce, echoStr string) (string, error) {
	// 第一步：修正 URL 查询解析后可能出现的空格，还原原始 Base64 字符串。
	if !c.validateSignature(msgSignature, timestamp, nonce, echoStr) {
		decoded, err := url.QueryUnescape(echoStr)
		if err != nil {
			return "", fmt.Errorf("decode echostr: %w", err)
		}

		if !c.validateSignature(msgSignature, timestamp, nonce, decoded) {
			return "", ErrInvalidSignature
		}
	}

	// 第二步：解密回调密文，将其恢复为明文形式。
	plain, err := c.decrypt(echoStr)
	if err != nil {
		return "", err
	}

	return string(plain), nil
}

// DecryptMessage 解密 POST 回调中的加密消息，并返回结构化后的 Message。
// Parameters:
//   - msgSignature: 企业微信用于签名的字段
//   - timestamp: 时间戳
//   - nonce: 随机串
//   - req: 包含 encrypt 字段的回调体
//
// Returns:
//   - *Message: 成功解密并解析后的业务消息
//   - error: 签名校验失败、解密失败或 JSON 解析失败时返回
//
// 流程图：
//
//	[收到加密请求]
//	     |
//	     v
//	[校验签名] --否--> [返回ErrInvalidSignature]
//	     |
//	    是
//	     |
//	     v
//	[Base64解密+AES-CBC解密]
//	     |
//	     v
//	[JSON解析为Message]
//	     |
//	     v
//	[返回Message]
func (c *Crypt) DecryptMessage(msgSignature, timestamp, nonce string, req EncryptedRequest) (*Message, error) {
	// 第一步：复用签名校验，拦截伪造请求。
	if !c.validateSignature(msgSignature, timestamp, nonce, req.Encrypt) {
		return nil, ErrInvalidSignature
	}

	// 第二步：将密文 decrypt 成原始业务 JSON 数据。
	plain, err := c.decrypt(req.Encrypt)
	if err != nil {
		return nil, err
	}

	// 第三步：将明文 JSON 解析为消息结构体。
	msg, err := ParseMessage(plain)
	if err != nil {
		return nil, err
	}

	// LOG: 记录解密后的请求明文，便于调试业务逻辑
	if pretty, err := json.MarshalIndent(msg, "", "  "); err == nil {
		log.Printf("WeCom Request Decrypted:\n%s\n", string(pretty))
	} else {
		log.Printf("WeCom Request Decrypted: %s\n", string(plain))
	}

	return msg, nil
}

// EncryptResponse 对回复明文进行加密封装。
// Parameters:
//   - payload: 待发送的明文结构体（如 StreamReply, TextMessage, TemplateCardMessage 等）
//   - timestamp: 调用方生成的时间戳字符串
//   - nonce: 回调参数中的随机串
//
// Returns:
//   - EncryptedResponse: 包含密文、签名等字段的响应包
//   - error: 序列化或加密失败时返回
//
// 流程图：
//
//	[响应明文] -> [JSON序列化] -> [AES组包加密] -> [生成签名] -> [封装响应]
func (c *Crypt) EncryptResponse(payload interface{}, timestamp, nonce string) (EncryptedResponse, error) {
	// 第一步：将回复结构体序列化为 JSON 字节。
	body, err := jsonMarshal(payload)
	if err != nil {
		return EncryptedResponse{}, err
	}

	// LOG: 记录即将加密的响应明文，使用 Pretty Print 格式
	if pretty, err := json.MarshalIndent(payload, "", "  "); err == nil {
		log.Printf("WeCom Response Plain:\n%s\n", string(pretty))
	} else {
		log.Printf("WeCom Response Plain: %s\n", string(body))
	}

	// 第二步：调用 encrypt 将 JSON 明文转换为企业微信要求的密文。
	encrypted, err := c.encrypt(body)
	if err != nil {
		return EncryptedResponse{}, err
	}

	// 第三步：生成签名并组装响应结构，与 nonce 一并返回。
	signature := calcSignature(c.token, timestamp, nonce, encrypted)
	return EncryptedResponse{
		Encrypt:      encrypted,
		MsgSignature: signature,
		Timestamp:    timestamp,
		Nonce:        nonce,
	}, nil
}

// validateSignature 校验签名是否匹配。
// Parameters:
//   - msgSignature: 请求携带的签名
//   - timestamp: 时间戳
//   - nonce: 随机串
//   - data: 密文或回显字符串
//
// Returns:
//   - bool: 签名一致返回 true，否则 false
func (c *Crypt) validateSignature(msgSignature, timestamp, nonce, data string) bool {
	// 生成企业微信规则下的期望签名值。
	expected := calcSignature(c.token, timestamp, nonce, data)

	// 使用不区分大小写的比较，抵御大小写差异导致的误判。
	return strings.EqualFold(expected, msgSignature)
}

// calcSignature 根据企业微信规则生成签名。
// Parameters:
//   - token: 消息配置的 Token
//   - timestamp: 时间戳字符串
//   - nonce: 随机串
//   - data: 加密数据或 echostr
//
// Returns:
//   - string: 十六进制的 SHA1 签名
func calcSignature(token, timestamp, nonce, data string) string {
	// 第一步：按字典序排列 token、timestamp、nonce、data。
	parts := []string{token, timestamp, nonce, data}
	sort.Strings(parts)

	// 第二步：将排好序的字段拼接成单个字符串。
	s := strings.Join(parts, "")

	// 第三步：计算 SHA1 摘要并转为十六进制表示。
	h := sha1.Sum([]byte(s))
	return hex.EncodeToString(h[:]) // 拼接后进行 SHA1，再输出 16 进制字符串
}

// decodeAESKey 将企业微信提供的 EncodingAESKey 转换为 32 字节 AES 密钥。
// Parameters:
//   - encodingKey: 企业微信后台配置的 43 字节编码串
//
// Returns:
//   - []byte: 解码后的 32 字节 AES 密钥
//   - error: 当编码非法或解码长度不匹配时返回
//
// 该密钥使用 Base64 表示，长度固定为 43，需要补齐 '='。
func decodeAESKey(encodingKey string) ([]byte, error) {
	// 企业微信给出的 EncodingAESKey 长度为 43，需要补充 '=' 才能正确解码。
	if len(encodingKey) == 0 {
		return nil, ErrInvalidAESKey
	}

	// 第一步：计算需要补充的 '=' 数量，得到有效的 Base64 字符串。
	padding := (4 - len(encodingKey)%4) % 4
	encoded := encodingKey + strings.Repeat("=", padding)

	// 第二步：执行 Base64 解码，将 43 字节编码串还原为 32 字节密钥。
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode aes key: %w", err)
	}
	if len(key) != 32 {
		return nil, ErrInvalidAESKey
	}

	return key, nil
}

// pkcs7Pad 按 PKCS#7 规则补齐块长度。
// Parameters:
//   - data: 待填充的数据
//   - blockSize: 块大小
//
// Returns:
//   - []byte: 追加填充后的数据
func pkcs7Pad(data []byte, blockSize int) []byte {
	// 计算需要补齐的字节数，并按照 PKCS#7 要求填充相同的数值。
	padLen := blockSize - len(data)%blockSize
	pad := bytesRepeat(byte(padLen), padLen)

	return append(data, pad...)
}

// pkcs7Unpad 去除 PKCS#7 填充，需保障密文长度与填充值合法。
// Parameters:
//   - data: 待去填充的数据
//   - blockSize: 块大小
//
// Returns:
//   - []byte: 去除填充后的数据
//   - error: 当填充无效时返回错误
func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, errors.New("invalid padding size")
	}

	// 读取填充长度，并确认其不超过块大小。
	padLen := int(data[len(data)-1])
	if padLen == 0 || padLen > blockSize {
		return nil, errors.New("invalid padding")
	}

	// 校验末尾每个字节是否都等于填充值，防止篡改。
	for i := 0; i < padLen; i++ {
		if data[len(data)-1-i] != byte(padLen) {
			return nil, errors.New("invalid padding")
		}
	}

	return data[:len(data)-padLen], nil
}

// decrypt 完成企业微信消息的 AES-CBC 解密与校验。
// Parameters:
//   - cipherText: Base64 表示的密文
//
// Returns:
//   - []byte: 解密后的明文字节
//   - error: Base64/AES 解密失败或 corpID 校验失败时返回
//
// 流程图：
//
//	[Base64密文]
//	     |
//	     v
//	[Base64解码]
//	     |
//	     v
//	[AES-CBC解密] -> [PKCS7去填充]
//	     |
//	     v
//	[解析随机数|长度|消息|CorpID]
//	     |
//	     v
//	[校验CorpID并返回消息体]
func (c *Crypt) decrypt(cipherText string) ([]byte, error) {
	// 第一步：Base64 解码密文，恢复原始加密字节。
	cipherData, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}

	// 第二步：构造 AES 块密码器，准备执行 CBC 解密。
	block, err := aes.NewCipher(c.aesKey)
	if err != nil {
		return nil, err
	}
	if len(cipherData)%block.BlockSize() != 0 {
		return nil, errors.New("invalid ciphertext length")
	}

	// 第三步：按照协议使用 AESKey 前 16 字节作为 IV，进行 CBC 解密。
	iv := c.aesKey[:block.BlockSize()] // 企业微信协议规定 IV 为 AESKey 前 16 字节
	mode := cipher.NewCBCDecrypter(block, iv)
	plain := make([]byte, len(cipherData))
	mode.CryptBlocks(plain, cipherData) // 逐块解密

	// 第四步：去除 PKCS#7 填充，得到真实消息体。
	plain, err = pkcs7Unpad(plain, padBlockSize)
	if err != nil {
		return nil, err
	}
	if len(plain) < 20 {
		return nil, errors.New("plaintext too short")
	}

	// 第五步：解析随机数、长度字段与 corpID，并进行合法性校验。
	content := plain[16:]
	msgLen := binary.BigEndian.Uint32(content[:4])
	if int(msgLen) > len(content[4:]) {
		return nil, errors.New("invalid message length")
	}
	msg := content[4 : 4+msgLen]

	// ReceiveId（content[4+msgLen:]）在不同场景含义如下：
	// 1. 企业应用回调：对应企业的 corpID。
	// 2. 第三方事件回调：对应服务商套件的 suiteID。
	// 3. 个人主体的第三方应用：ReceiveId 为空字符串。
	// 智能机器人场景回传空串，因此此处不再读取 ReceiveId，仅保留注释说明。
	//receiveID := content[4+msgLen:]
	//if c.corpID != "" && string(receiveID) != c.corpID {
	//	return nil, errors.New("receive id mismatch")
	//}

	return msg, nil
}

// encrypt 将明文消息封装为企业微信要求的 AES-CBC Base64 密文。
// Parameters:
//   - plain: 待加密的明文字节
//
// Returns:
//   - string: Base64 编码的密文
//   - error: 加密过程中出现异常时返回
//
// 流程图：
//
//	[随机16字节] + [消息长度] + [明文] + [CorpID]
//	     |
//	     v
//	[PKCS7填充]
//	     |
//	     v
//	[AES-CBC加密]
//	     |
//	     v
//	[Base64编码并返回]
func (c *Crypt) encrypt(plain []byte) (string, error) {
	// 第一步：初始化 AES 块密码器，后续用于 CBC 加密。
	block, err := aes.NewCipher(c.aesKey)
	if err != nil {
		return "", err
	}

	// 第二步：生成协议要求的 16 字节随机前缀。
	randomBytes := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, randomBytes); err != nil {
		return "", err
	}

	// 第三步：拼接随机数、消息长度、明文与 corpID，构造密文明文块。
	msgLen := uint32(len(plain))
	buf := make([]byte, 0, 16+4+len(plain)+len(c.corpID))
	buf = append(buf, randomBytes...) // 协议规定先拼接 16 字节随机数
	lenBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBytes, msgLen)
	buf = append(buf, lenBytes...)         // 4 字节大端消息长度
	buf = append(buf, plain...)            // 实际消息体
	buf = append(buf, []byte(c.corpID)...) // ReceiveId：企业应用=corpID，第三方事件=suiteID，个人第三方=空串
	buf = pkcs7Pad(buf, padBlockSize)

	// 第四步：使用 AES-CBC 模式加密，并将结果转换为 Base64。
	iv := c.aesKey[:block.BlockSize()]
	mode := cipher.NewCBCEncrypter(block, iv)
	cipherData := make([]byte, len(buf))
	mode.CryptBlocks(cipherData, buf)

	return base64.StdEncoding.EncodeToString(cipherData), nil
}

// jsonMarshal 抽象为函数便于测试时替换。
var jsonMarshal = func(v any) ([]byte, error) {
	return json.Marshal(v)
}

// bytesRepeat 用于构造填充字节切片，避免直接依赖 bytes.Repeat。
// Parameters:
//   - b: 需要重复的字节
//   - count: 重复次数
//
// Returns:
//   - []byte: 由重复字节组成的新切片
func bytesRepeat(b byte, count int) []byte {
	// 逐字节填充指定值，避免引入 bytes.Repeat 依赖，便于测试替换。
	buf := make([]byte, count)
	for i := range buf {
		buf[i] = b
	}

	return buf
}
