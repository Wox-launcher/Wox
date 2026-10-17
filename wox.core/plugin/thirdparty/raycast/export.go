// Package raycast reads a user-exported Raycast .rayconfig.
// This file only unwraps the RAYCFG3 container. It does not know about Wox settings.
package raycast

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"

	"golang.org/x/crypto/scrypt"
)

const (
	exportMagic        = "RAYCFG3\n"
	exportHeaderBytes  = 12
	exportTagBytes     = 16
	exportNonceBytes   = 16
	exportSaltBytes    = 16
	exportSchema       = 3
	exportScryptN      = 16384
	exportScryptR      = 8
	exportScryptP      = 1
	exportScryptKeyLen = 32

	// Stable error codes. The wizard translates them; format failures stay distinct from a bad passphrase.
	codeNotExport          = "not_export"
	codeUnsupportedVersion = "unsupported_version"
	codeCorrupt            = "corrupt"
	codeAuth               = "auth"
	codeLimit              = "limit"
)

// Limits bounds one export before and after decryption.
// The payload cap is a decompressed-byte ceiling, not a process memory ceiling.
type Limits struct {
	MaxFileBytes    int64
	MaxHeaderBytes  int
	MaxPayloadBytes int
	MaxEntries      int
	MaxTextBytes    int
	MaxImageBytes   int64
}

// DefaultLimits matches the container caps used by the Raycast 2.x reader.
func DefaultLimits() Limits {
	return Limits{
		MaxFileBytes:    256 << 20,
		MaxHeaderBytes:  1 << 20,
		MaxPayloadBytes: 512 << 20,
		MaxEntries:      100000,
		MaxTextBytes:    1 << 20,
		MaxImageBytes:   25 << 20,
	}
}

func (limits Limits) withDefaults() Limits {
	defaults := DefaultLimits()
	if limits.MaxFileBytes == 0 {
		limits.MaxFileBytes = defaults.MaxFileBytes
	}
	if limits.MaxHeaderBytes == 0 {
		limits.MaxHeaderBytes = defaults.MaxHeaderBytes
	}
	if limits.MaxPayloadBytes == 0 {
		limits.MaxPayloadBytes = defaults.MaxPayloadBytes
	}
	if limits.MaxEntries == 0 {
		limits.MaxEntries = defaults.MaxEntries
	}
	if limits.MaxTextBytes == 0 {
		limits.MaxTextBytes = defaults.MaxTextBytes
	}
	if limits.MaxImageBytes == 0 {
		limits.MaxImageBytes = defaults.MaxImageBytes
	}
	return limits
}

// ExportError is a container failure. Code is one of the stable export codes.
type ExportError struct {
	Code string
}

func (e *ExportError) Error() string {
	if e == nil {
		return ""
	}
	return e.Code
}

func exportErr(code string) error {
	return &ExportError{Code: code}
}

// ExportCode returns a stable code when err came from this package.
func ExportCode(err error) string {
	var target *ExportError
	if errors.As(err, &target) {
		return target.Code
	}
	return ""
}

// DecryptOptions opens one container. A pre-derived Key skips scrypt so tests can share one derivation.
type DecryptOptions struct {
	Passphrase string
	Key        []byte
	Limits     Limits
}

// Recognize reports whether path begins with the RAYCFG3 signature. It does not decrypt.
func Recognize(path string) (bool, string) {
	file, err := os.Open(path)
	if err != nil {
		return false, codeCorrupt
	}
	defer file.Close()
	var magic [len(exportMagic)]byte
	if _, err := io.ReadFull(file, magic[:]); err != nil {
		return false, codeCorrupt
	}
	if string(magic[:]) != exportMagic {
		return false, codeNotExport
	}
	return true, ""
}

// DecryptFile reads and opens one .rayconfig. The passphrase copy used for scrypt is wiped after derivation.
func DecryptFile(ctx context.Context, path, passphrase string, limits Limits) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, exportErr(codeCorrupt)
	}
	if limits.MaxFileBytes > 0 && info.Size() > limits.MaxFileBytes {
		return nil, exportErr(codeLimit)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, exportErr(codeCorrupt)
	}
	return Decrypt(ctx, raw, DecryptOptions{Passphrase: passphrase, Limits: limits})
}

// Decrypt opens a RAYCFG3 blob. Framing is checked before any key derivation.
func Decrypt(ctx context.Context, raw []byte, options DecryptOptions) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	limits := options.Limits.withDefaults()
	frame, err := readFrame(raw, limits)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	key := options.Key
	if len(key) == 0 {
		secret := []byte(options.Passphrase)
		derived, deriveErr := scrypt.Key(secret, frame.salt, exportScryptN, exportScryptR, exportScryptP, exportScryptKeyLen)
		wipe(secret)
		if deriveErr != nil {
			return nil, exportErr(codeAuth)
		}
		defer wipe(derived)
		key = derived
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, exportErr(codeAuth)
	}
	// Raycast uses a 16-byte nonce. The default GCM constructor only accepts 12.
	aead, err := cipher.NewGCMWithNonceSize(block, exportNonceBytes)
	if err != nil {
		return nil, exportErr(codeCorrupt)
	}
	sealed := make([]byte, 0, len(frame.ciphertext)+len(frame.tag))
	sealed = append(sealed, frame.ciphertext...)
	sealed = append(sealed, frame.tag...)
	compressed, err := aead.Open(nil, frame.nonce, sealed, nil)
	wipe(sealed)
	if err != nil {
		return nil, exportErr(codeAuth)
	}
	payload, err := gunzipLimited(compressed, limits.MaxPayloadBytes)
	wipe(compressed)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

type exportFrame struct {
	salt       []byte
	nonce      []byte
	ciphertext []byte
	tag        []byte
}

// readFrame checks sizes and the unauthenticated header before the caller derives a key.
func readFrame(raw []byte, limits Limits) (exportFrame, error) {
	if len(raw) < len(exportMagic) || string(raw[:len(exportMagic)]) != exportMagic {
		return exportFrame{}, exportErr(codeNotExport)
	}
	if len(raw) < exportHeaderBytes+exportTagBytes {
		return exportFrame{}, exportErr(codeCorrupt)
	}
	headerLen := binary.LittleEndian.Uint32(raw[8:12])
	if headerLen == 0 {
		return exportFrame{}, exportErr(codeCorrupt)
	}
	if uint64(headerLen) > uint64(limits.MaxHeaderBytes) {
		return exportFrame{}, exportErr(codeLimit)
	}
	payloadStart := uint64(exportHeaderBytes) + uint64(headerLen)
	if payloadStart > uint64(len(raw)) || payloadStart+uint64(exportTagBytes) > uint64(len(raw)) {
		return exportFrame{}, exportErr(codeCorrupt)
	}
	headerJSON, err := gunzipLimited(raw[exportHeaderBytes:payloadStart], limits.MaxHeaderBytes)
	if err != nil {
		return exportFrame{}, err
	}
	var header struct {
		SchemaVersion int `json:"schemaVersion"`
		Encryption    *struct {
			IV   string `json:"iv"`
			Salt string `json:"salt"`
		} `json:"encryption"`
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return exportFrame{}, exportErr(codeCorrupt)
	}
	if header.SchemaVersion != exportSchema {
		return exportFrame{}, exportErr(codeUnsupportedVersion)
	}
	if header.Encryption == nil {
		return exportFrame{}, exportErr(codeCorrupt)
	}
	nonce, err := decodeFixedHex(header.Encryption.IV, exportNonceBytes)
	if err != nil {
		return exportFrame{}, exportErr(codeCorrupt)
	}
	salt, err := decodeFixedHex(header.Encryption.Salt, exportSaltBytes)
	if err != nil {
		return exportFrame{}, exportErr(codeCorrupt)
	}
	tagStart := len(raw) - exportTagBytes
	return exportFrame{
		salt:       salt,
		nonce:      nonce,
		ciphertext: raw[payloadStart:tagStart],
		tag:        raw[tagStart:],
	}, nil
}

func decodeFixedHex(value string, size int) ([]byte, error) {
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != size {
		return nil, exportErr(codeCorrupt)
	}
	return decoded, nil
}

// gunzipLimited inflates data and fails when the output would pass maxBytes.
func gunzipLimited(data []byte, maxBytes int) ([]byte, error) {
	if maxBytes <= 0 {
		return nil, exportErr(codeLimit)
	}
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, exportErr(codeCorrupt)
	}
	defer reader.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, io.LimitReader(reader, int64(maxBytes)+1)); err != nil {
		return nil, exportErr(codeCorrupt)
	}
	if buf.Len() > maxBytes {
		return nil, exportErr(codeLimit)
	}
	return buf.Bytes(), nil
}

func wipe(buf []byte) {
	for i := range buf {
		buf[i] = 0
	}
}

// Seal builds a RAYCFG3 container around payload. Callers use it to create fixtures, not to write user exports.
func Seal(payload []byte, passphrase string) ([]byte, error) {
	salt := make([]byte, exportSaltBytes)
	nonce := make([]byte, exportNonceBytes)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	header := map[string]any{
		"schemaVersion": exportSchema,
		"encryption": map[string]string{
			"iv":   hex.EncodeToString(nonce),
			"salt": hex.EncodeToString(salt),
		},
	}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return nil, err
	}
	headerGzip, err := gzipBytes(headerJSON)
	if err != nil {
		return nil, err
	}
	payloadGzip, err := gzipBytes(payload)
	if err != nil {
		return nil, err
	}
	secret := []byte(passphrase)
	key, err := scrypt.Key(secret, salt, exportScryptN, exportScryptR, exportScryptP, exportScryptKeyLen)
	wipe(secret)
	if err != nil {
		return nil, err
	}
	defer wipe(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCMWithNonceSize(block, exportNonceBytes)
	if err != nil {
		return nil, err
	}
	sealed := aead.Seal(nil, nonce, payloadGzip, nil)
	if len(sealed) < exportTagBytes {
		return nil, exportErr(codeCorrupt)
	}
	out := make([]byte, 0, exportHeaderBytes+len(headerGzip)+len(sealed))
	out = append(out, exportMagic...)
	var length [4]byte
	binary.LittleEndian.PutUint32(length[:], uint32(len(headerGzip)))
	out = append(out, length[:]...)
	out = append(out, headerGzip...)
	out = append(out, sealed...)
	return out, nil
}

func gzipBytes(payload []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	if _, err := writer.Write(payload); err != nil {
		_ = writer.Close()
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
