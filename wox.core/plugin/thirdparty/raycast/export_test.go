package raycast

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"os"
	"testing"

	"golang.org/x/crypto/scrypt"
)

func TestRecognizeMagic(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/export.rayconfig"
	if err := os.WriteFile(path, []byte("not-a-raycast-export"), 0o644); err != nil {
		t.Fatal(err)
	}
	ok, detail := Recognize(path)
	if ok || detail != codeNotExport {
		t.Fatalf("recognize = %v %s", ok, detail)
	}
	blob, err := Seal([]byte(`{"ok":true}`), "secret")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, blob, 0o644); err != nil {
		t.Fatal(err)
	}
	ok, detail = Recognize(path)
	if !ok || detail != "" {
		t.Fatalf("recognize sealed = %v %s", ok, detail)
	}
}

func TestFrameErrorsDoNotNeedAKey(t *testing.T) {
	ctx := context.Background()
	limits := DefaultLimits()
	if _, err := Decrypt(ctx, []byte("not-raycast"), DecryptOptions{Limits: limits}); ExportCode(err) != codeNotExport {
		t.Fatalf("magic = %v", err)
	}
	short := append([]byte(exportMagic), 1, 0, 0, 0)
	if _, err := Decrypt(ctx, short, DecryptOptions{Limits: limits}); ExportCode(err) != codeCorrupt {
		t.Fatalf("truncated = %v", err)
	}
	header, err := gzipJSON(map[string]any{"schemaVersion": 2})
	if err != nil {
		t.Fatal(err)
	}
	blob := container(header, []byte("ciphertext-and-tag!!"))
	if _, err := Decrypt(ctx, blob, DecryptOptions{Limits: limits}); ExportCode(err) != codeUnsupportedVersion {
		t.Fatalf("schema = %v", err)
	}
	huge := make([]byte, exportHeaderBytes+exportTagBytes)
	copy(huge, exportMagic)
	binary.LittleEndian.PutUint32(huge[8:12], uint32(limits.MaxHeaderBytes)+1)
	if _, err := Decrypt(ctx, huge, DecryptOptions{Limits: limits}); ExportCode(err) != codeLimit {
		t.Fatalf("header limit = %v", err)
	}
}

func TestPayloadLimitUsesTheCallerCap(t *testing.T) {
	payload := bytes.Repeat([]byte("a"), 4096)
	blob, err := Seal(payload, "secret")
	if err != nil {
		t.Fatal(err)
	}
	limits := DefaultLimits()
	limits.MaxPayloadBytes = 32
	_, err = Decrypt(context.Background(), blob, DecryptOptions{Passphrase: "secret", Limits: limits})
	if ExportCode(err) != codeLimit {
		t.Fatalf("payload limit = %v", err)
	}
}

func TestAuthFailuresShareOneDerivedKey(t *testing.T) {
	payload := []byte(`{"ok":true}`)
	blob, err := Seal(payload, "secret")
	if err != nil {
		t.Fatal(err)
	}
	frame, err := readFrame(blob, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	key, err := scrypt.Key([]byte("secret"), frame.salt, exportScryptN, exportScryptR, exportScryptP, exportScryptKeyLen)
	if err != nil {
		t.Fatal(err)
	}
	defer wipe(key)
	got, err := Decrypt(context.Background(), blob, DecryptOptions{Key: key})
	if err != nil || string(got) != string(payload) {
		t.Fatalf("open = %s %v", got, err)
	}
	wrong := append([]byte(nil), key...)
	wrong[0] ^= 0xff
	if _, err := Decrypt(context.Background(), blob, DecryptOptions{Key: wrong}); ExportCode(err) != codeAuth {
		t.Fatalf("wrong key = %v", err)
	}
	damaged := append([]byte(nil), blob...)
	damaged[len(damaged)-1] ^= 0xff
	if _, err := Decrypt(context.Background(), damaged, DecryptOptions{Key: key}); ExportCode(err) != codeAuth {
		t.Fatalf("bad tag = %v", err)
	}
}

func TestIndependentVector(t *testing.T) {
	const encoded = "UkFZQ0ZHMwp4AAAAH4sIAAAAAAAACh3HTQrCMBAG0Lt86y6STP4P4n4ymWBBqzRFkNK7C77dOzHlrk++6T7X14ZKC3ST/fs+/j2xflAxhmrvIq0xl5JzSjGG4D2Rc9YagwWTHwcqjLHWOSLvQ4gxpZxLYW5NpHfVMXBdP8Bwuhx0AAAANf3yGBBtbEs0y1RKqg3IZM0kUp3SfO1PQlt4pxVL0hF1hI+gU19De8NtODNqthCWgM/Q++wl5YV2ZMp7"
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Decrypt(context.Background(), raw, DecryptOptions{Passphrase: "wox-raycast-vector"})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"marker":"independent"}` {
		t.Fatalf("payload = %s", got)
	}
}

func TestEntryLimit(t *testing.T) {
	raw := []byte(`{"snippets":{"snippets":[{"title":"a"},{"title":"b"}]}}`)
	limits := DefaultLimits()
	limits.MaxEntries = 1
	if _, err := ParsePayload(raw, limits); ExportCode(err) != codeLimit {
		t.Fatalf("entries = %v", err)
	}
}

func gzipJSON(value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	if _, err := writer.Write(raw); err != nil {
		_ = writer.Close()
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func container(header, body []byte) []byte {
	out := append([]byte(exportMagic), 0, 0, 0, 0)
	binary.LittleEndian.PutUint32(out[8:12], uint32(len(header)))
	out = append(out, header...)
	return append(out, body...)
}
