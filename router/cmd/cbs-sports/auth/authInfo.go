package auth

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"slices"
	"strings"

	"github.com/wundergraph/cosmo/router/core"
)

const (
	AccessTokenParamName    = "cbs-app-access-token"
	AuthorizationHeaderName = "Authorization"
	PidCookieName           = "pid"
	CbsAppTokenCookieName   = "picks-cbs-app-token"
)

func pkcs7Unpad(data []byte) ([]byte, error) {
	length := len(data)
	if length == 0 {
		return nil, fmt.Errorf("invalid padding size")
	}

	padding := int(data[length-1])
	if padding > length || padding == 0 {
		return nil, fmt.Errorf("invalid padding")
	}

	for _, p := range data[length-padding:] {
		if int(p) != padding {
			return nil, fmt.Errorf("invalid padding")
		}
	}

	return data[:length-padding], nil
}

type authorizer interface {
	userLogin() (string, error)
}

type pidCookie struct {
	isEncrypted bool
	status      string
	pid         string
}

func (c pidCookie) isValid() bool {
	return len(c.status) > 0 && len(c.pid) > 0
}

func (c pidCookie) isLoggedIn() bool {
	return c.status == "L"
}

func (c pidCookie) userLogin() (string, error) {
	if !c.isValid() {
		return "", fmt.Errorf("invalid cookie found")
	}

	if !c.isLoggedIn() {
		return "", nil
	}

	if c.isEncrypted {
		return c.decrypt()
	}
	return c.decryptLegacy()
}

func (c pidCookie) decrypt() (string, error) {
	encryptedPidTokenKey := os.Getenv("SAPI_ENCRYPTED_PID_TOKEN_KEY")
	if encryptedPidTokenKey == "" {
		return "", errors.New("SAPI_ENCRYPTED_PID_TOKEN_KEY env variable not found")
	}

	encryptedPidTokenIv := os.Getenv("SAPI_ENCRYPTED_PID_TOKEN_IV")
	if encryptedPidTokenKey == "" {
		return "", errors.New("SAPI_ENCRYPTED_PID_TOKEN_IV env variable not found")
	}

	decoded, err := base64.StdEncoding.DecodeString(c.pid)
	if err != nil {
		return "", fmt.Errorf("error when decoding encrypted pid cookie: %v", err)
	}

	md5Bytes := md5.Sum([]byte(encryptedPidTokenIv))
	hexBytes := hex.EncodeToString(md5Bytes[:])
	ivBytes := hexBytes[:aes.BlockSize]

	block, err := aes.NewCipher([]byte(encryptedPidTokenKey))
	if err != nil {
		return "", fmt.Errorf("error when creating encrypted pid cypher: %v", err)
	}

	mode := cipher.NewCBCDecrypter(block, []byte(ivBytes))
	mode.CryptBlocks(decoded, decoded)

	decoded, err = pkcs7Unpad(decoded)
	if err != nil {
		return "", fmt.Errorf("error while unpadding ciper text: %v", err)
	}

	return string(decoded), nil
}

func (c pidCookie) decryptLegacy() (string, error) {
	return "", errors.New("LEGACY COOKIE NOT IMPLEMENTED")
}

func newPidCookie(cookieValue string) (pidCookie, error) {
	decoded, err := url.QueryUnescape(cookieValue)
	if err != nil {
		return pidCookie{}, fmt.Errorf("could not unscape cookie value: %v", err)
	}

	parts := strings.Split(decoded, ":")
	status, pid, isEncrypted := "", "", false
	if len(parts) > 0 {
		status = parts[0]
	}

	if len(parts) > 2 {
		pid = parts[2]
	}

	if len(parts) > 3 {
		isEncrypted = parts[3] == "1"
	}

	return pidCookie{
		isEncrypted: isEncrypted,
		pid:         pid,
		status:      status,
	}, nil
}

type accessToken struct {
	kind  string
	value string
}

func (t accessToken) isValid() bool {
	return len(t.value) > 0
}

func generateAccessTokenHash(salt, passphrase []byte, keyLen, ivLen int) []byte {
	desiredLen := keyLen + ivLen
	data := make([]byte, 0)
	d := make([]byte, 0)

	for len(data) < desiredLen {
		temp := append([]byte{}, d...)
		temp = append(temp, passphrase...)
		temp = append(temp, salt...)

		hasher := md5.New()
		hasher.Write(temp)
		d = hasher.Sum(nil)

		data = append(data, d...)
	}

	return data
}

func (t accessToken) userLogin() (string, error) {
	passphrase := os.Getenv("SAPI_ENCRYPTED_PID_TOKEN_KEY")
	if passphrase == "" {
		return "", errors.New("SAPI_ENCRYPTED_PID_TOKEN_KEY env variable not found")
	}

	decodedToken := strings.ReplaceAll(t.value, "-", "/")
	decodedToken = strings.ReplaceAll(decodedToken, "_", "+")

	raw, err := base64.RawStdEncoding.DecodeString(decodedToken)
	if err != nil {
		return "", fmt.Errorf("error when decoding access token: %v", err)
	}

	salted := raw[0:16]
	salt := salted[8:]
	message := raw[16:]

	hash := generateAccessTokenHash(salt, []byte(passphrase), 32, 16)
	key := hash[0:32]
	iv := hash[32:48]

	block, err := aes.NewCipher(key)
	if err != nil {
		log.Fatal(err)
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(message, message)

	parts := bytes.Split(message, []byte("|"))
	if len(parts) > 5 {
		return string(parts[5]), nil
	}

	return "", errors.New("error when reading userLogin from access token")
}

func newAccessToken(accessTokenValue string) (accessToken, error) {
	decoded, err := url.QueryUnescape(accessTokenValue)
	if err != nil {
		return accessToken{}, fmt.Errorf("could not unscape access token value: %v", err)
	}

	parts := strings.Split(decoded, " ")
	slices.Reverse(parts)
	kind, value := "Bearer", ""

	if len(parts) > 0 {
		value = parts[0]
	}

	if len(parts) > 1 {
		kind = parts[1]
	}

	return accessToken{
		kind:  kind,
		value: value,
	}, nil
}

func findAuthorizer(ctx core.RequestContext) (authorizer, error) {
	// Try to find token in authorization header
	authorizationHeaderValue := ctx.Request().Header.Get(AuthorizationHeaderName)
	if authorizationHeaderValue != "" {
		ctx.Logger().Debug("Found access token in authorization header")
		accessTokenAuth, err := newAccessToken(authorizationHeaderValue)
		if err != nil {
			return nil, err
		}
		return accessTokenAuth, nil
	}

	// Try to find token in the query params
	accessTokenParamValue := ctx.Request().URL.Query().Get(AccessTokenParamName)
	if accessTokenParamValue != "" {
		ctx.Logger().Debug("Found access token in query param")
		accessTokenAuth, err := newAccessToken(accessTokenParamValue)
		if err != nil {
			return nil, err
		}
		return accessTokenAuth, nil
	}

	// Try to find the pid cookie
	pidCookie, err := ctx.Request().Cookie(PidCookieName)
	if err == nil && pidCookie != nil {
		ctx.Logger().Debug("Found pid cookie")
		pidTokenAuth, err := newPidCookie(pidCookie.Value)
		if err != nil {
			return nil, err
		}
		return pidTokenAuth, nil
	}

	// Try to find token in the picks-cbs-app-token
	picksCbsAppTokenCookie, err := ctx.Request().Cookie(CbsAppTokenCookieName)
	if err == nil && picksCbsAppTokenCookie != nil {
		ctx.Logger().Debug("Found access token in picks-cbs-app-token cookie")
		picksCbsAppToken, err := newAccessToken(picksCbsAppTokenCookie.Value)
		if err != nil {
			return nil, err
		}
		return picksCbsAppToken, nil
	}

	// At this point we could not find any auth information in the http request
	return nil, nil
}
