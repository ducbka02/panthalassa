package scrypt

import (
	"crypto/rand"
	"encoding/json"
	"errors"

	aes "github.com/Bit-Nation/panthalassa/crypto/aes"
	scrypt "golang.org/x/crypto/scrypt"
)

const n = 16384
const r = 8
const p = 1
const saltLength = 50
const keyLength = 32

type Key struct {
	N      int    `json:"n"`
	R      int    `json:"r"`
	P      int    `json:"p"`
	KeyLen int    `json:"key_len"`
	Salt   []byte `json:"salt"`
	key    aes.Secret
}

type CipherText struct {
	CipherText aes.CipherText `json:"cipher_text"`
	ScryptKey  Key            `json:"scrypt_key"`
}

// exports CipherText as json
func (s *CipherText) Export() (string, error) {

	jsonData, err := json.Marshal(s)

	if err != nil {
		return "", err
	}

	return string(jsonData), nil

}

// derives a key from password
func makeScryptKey(pw []byte) (Key, error) {

	// create salt for scrypt
	salt := make([]byte, saltLength)
	_, err := rand.Read(salt)
	if err != nil {
		return Key{}, err
	}

	// derive new key
	key, err := scrypt.Key(pw, salt, n, r, p, keyLength)
	if err != nil {
		return Key{}, err
	}
	if len(key) != 32 {
		return Key{}, errors.New("key must be of length 32 in order to be used with AES")
	}

	var aesSecret aes.Secret
	copy(aesSecret[:], key[:])

	sV := Key{
		N:      n,
		R:      r,
		P:      p,
		KeyLen: keyLength,
		Salt:   salt,
		key:    aesSecret,
	}

	return sV, nil
}

//Create new ScryptCipherText
func NewCipherText(plainText []byte, password []byte) (CipherText, error) {

	derivedKey, err := makeScryptKey(password)
	if err != nil {
		return CipherText{}, err
	}

	cipherText, err := aes.Encrypt(plainText, derivedKey.key)

	return CipherText{
		CipherText: cipherText,
		ScryptKey:  derivedKey,
	}, nil

}

// decrypt scrypt cipher
func DecryptCipherText(cipherText CipherText, password []byte) (aes.PlainText, error) {

	key, err := scrypt.Key(password, cipherText.ScryptKey.Salt, cipherText.ScryptKey.N, cipherText.ScryptKey.R, cipherText.ScryptKey.P, cipherText.ScryptKey.KeyLen)
	if err != nil {
		return aes.PlainText{}, err
	}

	var AESSecret aes.Secret
	copy(AESSecret[:], key[:32])

	return aes.Decrypt(cipherText.CipherText, AESSecret)
}
