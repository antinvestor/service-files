package storage

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/antinvestor/service-files/apps/default/service/types"
)

const (
	encryptionVersion = 1
	encryptionAlg     = "AES-256-GCM-CHUNKED"
	defaultChunkSize  = 64 * 1024
)

// EncryptStream encrypts data from src to dst using chunked AES-GCM.
// It returns encryption metadata required for decryption.
func EncryptStream(ctx context.Context, src io.Reader, dst io.Writer, masterKey []byte) (*types.EncryptionInfo, error) {
	if len(masterKey) != 32 {
		return nil, fmt.Errorf("invalid master key length: %d", len(masterKey))
	}

	dataKey := make([]byte, 32)
	if _, err := rand.Read(dataKey); err != nil {
		return nil, err
	}

	wrapNonce := make([]byte, 12)
	if _, err := rand.Read(wrapNonce); err != nil {
		return nil, err
	}

	masterGCM, err := newGCM(masterKey)
	if err != nil {
		return nil, err
	}
	wrappedKey := masterGCM.Seal(nil, wrapNonce, dataKey, nil)

	noncePrefix := make([]byte, 4)
	if _, err := rand.Read(noncePrefix); err != nil {
		return nil, err
	}

	dataGCM, err := newGCM(dataKey)
	if err != nil {
		return nil, err
	}

	chunkSize := defaultChunkSize
	buf := make([]byte, chunkSize)
	var counter uint64

	for {
		n, readErr := src.Read(buf)
		if n > 0 {
			nonce := makeNonce(noncePrefix, counter)
			counter++
			ciphertext := dataGCM.Seal(nil, nonce, buf[:n], nil)
			if err := writeChunk(dst, ciphertext); err != nil {
				return nil, err
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, readErr
		}
	}

	return &types.EncryptionInfo{
		Version:         encryptionVersion,
		Algorithm:       encryptionAlg,
		ChunkSizeBytes:  chunkSize,
		WrappedKey:      base64.RawURLEncoding.EncodeToString(wrappedKey),
		WrappedKeyNonce: base64.RawURLEncoding.EncodeToString(wrapNonce),
		NoncePrefix:     base64.RawURLEncoding.EncodeToString(noncePrefix),
	}, nil
}

// NewDecryptingReader returns a reader that decrypts content encrypted by EncryptStream.
func NewDecryptingReader(src io.Reader, masterKey []byte, info *types.EncryptionInfo) (io.Reader, error) {
	if info == nil {
		return src, nil
	}
	if len(masterKey) != 32 {
		return nil, fmt.Errorf("invalid master key length: %d", len(masterKey))
	}

	wrappedKey, err := base64.RawURLEncoding.DecodeString(info.WrappedKey)
	if err != nil {
		return nil, err
	}
	wrappedNonce, err := base64.RawURLEncoding.DecodeString(info.WrappedKeyNonce)
	if err != nil {
		return nil, err
	}
	noncePrefix, err := base64.RawURLEncoding.DecodeString(info.NoncePrefix)
	if err != nil {
		return nil, err
	}
	if len(noncePrefix) != 4 {
		return nil, fmt.Errorf("invalid nonce prefix")
	}

	masterGCM, err := newGCM(masterKey)
	if err != nil {
		return nil, err
	}
	dataKey, err := masterGCM.Open(nil, wrappedNonce, wrappedKey, nil)
	if err != nil {
		return nil, err
	}

	dataGCM, err := newGCM(dataKey)
	if err != nil {
		return nil, err
	}

	return &decryptingReader{
		src:         src,
		gcm:         dataGCM,
		noncePrefix: noncePrefix,
	}, nil
}

type decryptingReader struct {
	src         io.Reader
	gcm         cipher.AEAD
	noncePrefix []byte
	counter     uint64
	buf         []byte
	closed      bool
}

func (dr *decryptingReader) Read(p []byte) (int, error) {
	for len(dr.buf) == 0 {
		chunk, err := dr.readNextChunk()
		if err != nil {
			return 0, err
		}
		dr.buf = chunk
	}

	n := copy(p, dr.buf)
	dr.buf = dr.buf[n:]
	return n, nil
}

func (dr *decryptingReader) readNextChunk() ([]byte, error) {
	if dr.closed {
		return nil, io.EOF
	}

	lengthBuf := make([]byte, 4)
	_, err := io.ReadFull(dr.src, lengthBuf)
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		dr.closed = true
		return nil, io.EOF
	}
	if err != nil {
		return nil, err
	}

	chunkLen := binary.BigEndian.Uint32(lengthBuf)
	if chunkLen == 0 {
		return nil, fmt.Errorf("invalid encrypted chunk length")
	}

	ciphertext := make([]byte, chunkLen)
	if _, err := io.ReadFull(dr.src, ciphertext); err != nil {
		return nil, err
	}

	nonce := makeNonce(dr.noncePrefix, dr.counter)
	dr.counter++
	plaintext, err := dr.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func writeChunk(dst io.Writer, ciphertext []byte) error {
	lengthBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBuf, uint32(len(ciphertext)))
	if _, err := dst.Write(lengthBuf); err != nil {
		return err
	}
	_, err := dst.Write(ciphertext)
	return err
}

func makeNonce(prefix []byte, counter uint64) []byte {
	nonce := make([]byte, 12)
	copy(nonce[:4], prefix)
	binary.BigEndian.PutUint64(nonce[4:], counter)
	return nonce
}
