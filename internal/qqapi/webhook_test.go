package qqapi

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func TestVerifyWebhookSignature(t *testing.T) {
	secret := "naOC0ocQE3shWLAfffVLB1rhYPG7"
	body := []byte(`{ "op": 0,"d": {}, "t": "GATEWAY_EVENT_NAME"}`)
	timestamp := "1725442341"
	privateKey, err := privateKeyFromSecret(secret)
	if err != nil {
		t.Fatalf("privateKeyFromSecret() error = %v", err)
	}
	var msg bytes.Buffer
	msg.WriteString(timestamp)
	msg.Write(body)
	signature := hex.EncodeToString(ed25519.Sign(privateKey, msg.Bytes()))

	if !VerifyWebhookSignature(secret, timestamp, body, signature) {
		t.Fatal("VerifyWebhookSignature() = false, want true")
	}
	if VerifyWebhookSignature(secret, timestamp, []byte(`{"op":0}`), signature) {
		t.Fatal("VerifyWebhookSignature() accepted changed body")
	}
}

func TestBuildValidationResponse(t *testing.T) {
	data, err := json.Marshal(ValidationRequest{
		PlainToken: "plain",
		EventTS:    "12345",
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	payload := Payload{Op: OpHTTPCallbackVerify, D: data}

	resp, err := BuildValidationResponse("secret", payload)
	if err != nil {
		t.Fatalf("BuildValidationResponse() error = %v", err)
	}
	if resp.PlainToken != "plain" || resp.Signature == "" {
		t.Fatalf("response = %+v", resp)
	}
}

func TestCallbackACK(t *testing.T) {
	ack := CallbackACK()
	if ack.Op != OpHTTPCallbackACK {
		t.Fatalf("ack op = %d", ack.Op)
	}
}
