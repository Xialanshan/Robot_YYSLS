package qqapi

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	OpDispatch           = 0
	OpHTTPCallbackACK    = 12
	OpHTTPCallbackVerify = 13
)

type Payload struct {
	ID string          `json:"id,omitempty"`
	Op int             `json:"op"`
	D  json.RawMessage `json:"d,omitempty"`
	S  int64           `json:"s,omitempty"`
	T  string          `json:"t,omitempty"`
}

type ValidationRequest struct {
	PlainToken string `json:"plain_token"`
	EventTS    string `json:"event_ts"`
}

type ValidationResponse struct {
	PlainToken string `json:"plain_token"`
	Signature  string `json:"signature"`
}

func VerifyWebhookSignature(botSecret, timestamp string, body []byte, signatureHex string) bool {
	if botSecret == "" || timestamp == "" || len(body) == 0 || signatureHex == "" {
		return false
	}

	signature, err := hex.DecodeString(signatureHex)
	if err != nil {
		return false
	}
	if len(signature) != ed25519.SignatureSize || signature[63]&224 != 0 {
		return false
	}

	publicKey, err := publicKeyFromSecret(botSecret)
	if err != nil {
		return false
	}

	var msg bytes.Buffer
	msg.WriteString(timestamp)
	msg.Write(body)
	return ed25519.Verify(publicKey, msg.Bytes(), signature)
}

func BuildValidationResponse(botSecret string, payload Payload) (ValidationResponse, error) {
	if payload.Op != OpHTTPCallbackVerify {
		return ValidationResponse{}, fmt.Errorf("payload op %d is not callback validation", payload.Op)
	}

	var request ValidationRequest
	if err := json.Unmarshal(payload.D, &request); err != nil {
		return ValidationResponse{}, err
	}
	if request.PlainToken == "" || request.EventTS == "" {
		return ValidationResponse{}, fmt.Errorf("validation payload requires plain_token and event_ts")
	}

	privateKey, err := privateKeyFromSecret(botSecret)
	if err != nil {
		return ValidationResponse{}, err
	}

	var msg bytes.Buffer
	msg.WriteString(request.EventTS)
	msg.WriteString(request.PlainToken)
	signature := ed25519.Sign(privateKey, msg.Bytes())
	return ValidationResponse{
		PlainToken: request.PlainToken,
		Signature:  hex.EncodeToString(signature),
	}, nil
}

func CallbackACK() Payload {
	return Payload{Op: OpHTTPCallbackACK}
}

func publicKeyFromSecret(secret string) (ed25519.PublicKey, error) {
	publicKey, _, err := ed25519.GenerateKey(strings.NewReader(secretSeed(secret)))
	return publicKey, err
}

func privateKeyFromSecret(secret string) (ed25519.PrivateKey, error) {
	_, privateKey, err := ed25519.GenerateKey(strings.NewReader(secretSeed(secret)))
	return privateKey, err
}

func secretSeed(secret string) string {
	seed := secret
	for len(seed) < ed25519.SeedSize {
		seed = strings.Repeat(seed, 2)
	}
	return seed[:ed25519.SeedSize]
}
