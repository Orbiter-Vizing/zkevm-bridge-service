package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
)

func aesDecrypt(cryted string, key string) string {
	if key == "" {
		return cryted
	}
	crytedByte, _ := base64.StdEncoding.DecodeString(cryted)
	k := []byte(key)
	block, _ := aes.NewCipher(k)
	blockSize := block.BlockSize()
	blockMode := cipher.NewCBCDecrypter(block, k[:blockSize])
	orig := make([]byte, len(crytedByte))
	blockMode.CryptBlocks(orig, crytedByte)
	length := len(orig)
	unpadding := int(orig[length-1])
	orig = orig[:(length - unpadding)]
	return string(orig)
}
