package utils

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"golang.org/x/crypto/pbkdf2"
	"io"
	"io/ioutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

func (ps *PasteSpec) SpecArray() []interface{} {
	return []interface{}{
		ps.IV,
		ps.Salt,
		ps.Iterations,
		ps.KeySize,
		ps.TagSize,
		ps.Algorithm,
		ps.Mode,
		ps.Compression,
	}
}

func NewPBClient(uri *url.URL, username, password string) *PBClient {
	return &PBClient{URL: *uri, Username: username, Password: password}
}

func (c *PBClient) CreatePaste(message, expire, formatter string, openDiscussion, burnAfterReading bool) (*CreatePasteResponse, error) {
	masterKey, err := generateRandomBytes(32)
	if err != nil {
		return nil, fmt.Errorf("cannot generate random bytes: %w", err)
	}

	pasteContent, err := json.Marshal(&PasteContent{Paste: message})
	if err != nil {
		return nil, fmt.Errorf("cannot marshal paste content: %w", err)
	}

	pasteData, err := encrypt(masterKey, pasteContent, formatter, openDiscussion, burnAfterReading)
	if err != nil {
		return nil, fmt.Errorf("cannot encrypt data: %w", err)
	}

	createPasteReq := &CreatePasteRequest{
		V:     2,
		AData: pasteData.adata(),
		Meta:  CreatePasteRequestMeta{Expire: expire},
		CT:    base64.RawStdEncoding.EncodeToString(pasteData.Data),
	}

	body, err := json.Marshal(createPasteReq)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal paste request: %w", err)
	}

	req, err := http.NewRequest("POST", c.URL.String(), bytes.NewBuffer(body))
	if err != nil {
		fmt.Printf("unable to create new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Content-Length", strconv.Itoa(len(body)))
	req.Header.Set("X-Requested-With", "JSONHttpRequest")
	req.SetBasicAuth(c.Username, c.Password)

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot execute http request: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Printf("unable to close connection: %v", err)
		}
	}(res.Body)

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pastebin server responds with %q status code", res.Status)
	}

	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("cannot read response body: %w", err)
	}

	pasteResponse := CreatePasteResponse{}
	err = json.Unmarshal(resBody, &pasteResponse)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal response: %w", err)
	}

	if pasteResponse.Status != 0 {
		return nil, fmt.Errorf("status of the paste is not zero: %s", pasteResponse.Message)
	}

	pasteId, err := url.Parse(pasteResponse.URL)
	if err != nil {
		return nil, fmt.Errorf("cannot parse paste url: %w", err)
	}

	var uri url.URL
	uri.Scheme = c.URL.Scheme
	uri.Host = c.URL.Host
	uri.RawQuery = pasteId.RawQuery
	uri.Fragment = Encode(masterKey)

	pasteResponse.URL = uri.String()

	return &pasteResponse, nil
}

func generateRandomBytes(n uint32) ([]byte, error) {
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

func (p *PasteData) adata() []interface{} {
	var b2i = map[bool]int8{false: 0, true: 1}

	return []interface{}{
		p.SpecArray(),
		p.Formatter,
		b2i[p.OpenDiscussion],
		b2i[p.BurnAfterReading],
	}
}

func encrypt(masterKey []byte, message []byte, formatter string, openDiscussion, burnAfterReading bool) (*PasteData, error) {
	iv, err := generateRandomBytes(12)
	if err != nil {
		return nil, err
	}

	salt, err := generateRandomBytes(8)
	if err != nil {
		return nil, err
	}

	paste := &PasteData{
		Formatter:        formatter,
		OpenDiscussion:   openDiscussion,
		BurnAfterReading: burnAfterReading,
		PasteSpec: &PasteSpec{
			IV:          base64.RawStdEncoding.EncodeToString(iv),
			Salt:        base64.RawStdEncoding.EncodeToString(salt),
			Iterations:  100000,
			KeySize:     256,
			TagSize:     128,
			Algorithm:   "aes",
			Mode:        "gcm",
			Compression: "none",
		},
	}

	key := pbkdf2.Key(masterKey, salt, paste.Iterations, 32, sha256.New)

	adata, err := json.Marshal(paste.adata())
	if err != nil {
		return nil, err
	}

	c, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(c)
	if err != nil {
		return nil, err
	}

	data := gcm.Seal(nil, iv, message, adata)

	paste.Data = data

	return paste, nil
}

func GeneratePrivateBinPaste(labs map[string]interface{}) map[string][]string {
	var labIds []string
	for _, info := range labs {
		labId := info.(map[string]interface{})["details"].([]string)[0]
		labIds = append(labIds, labId)
	}

	labIdWithPastes := make(map[string][]string)

	kc := K8sAuthenticate()

	config := Cfg{
		Name:             "default",
		Host:             "https://bin.apps.eng.partner-lab.rhecoeng.com",
		Username:         "dev",
		Password:         "dev",
		Expire:           "5min",
		OpenDiscussion:   false,
		BurnAfterReading: true,
		Formatter:        "plaintext",
	}

	uri, err := url.Parse(config.Host)
	if err != nil {
		log.Printf("Cannot parse %q bin host: %v", config.Name, config.Host)
	}

	pbc := NewPBClient(uri, config.Username, config.Password)

	for _, info := range labs {
		var pasteData []string

		adminPasswordSecretRef, err := kc.CoreV1().Secrets("hive").Get(context.Background(),
			info.(map[string]interface{})["details"].([]string)[2], metav1.GetOptions{})
		ErrorCheck("Unable to get admin password secret reference: ", err)

		kubeConfigSecretRef, err := kc.CoreV1().Secrets("hive").Get(context.Background(),
			info.(map[string]interface{})["details"].([]string)[3], metav1.GetOptions{})
		ErrorCheck("Unable to get kubeconfig secret reference: ", err)

		pasteData = append(pasteData, string(adminPasswordSecretRef.Data["password"]),
			string(kubeConfigSecretRef.Data["kubeconfig"]))

		for _, paste := range pasteData {
			resp, err := pbc.CreatePaste(
				paste,
				config.Expire,
				config.Formatter,
				config.OpenDiscussion,
				config.BurnAfterReading)
			ErrorCheck("Unable to create paste: %v", err)
			labIdWithPastes[info.(map[string]interface{})["details"].([]string)[0]] = append(
				labIdWithPastes[info.(map[string]interface{})["details"].([]string)[0]], resp.URL)
		}
	}

	return labIdWithPastes
}
