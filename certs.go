package main

import (
	"bytes"
	"crypto/tls"
	"encoding/pem"
	"fmt"
)

func fetchCerts() (string, error) {
	conn, err := tls.Dial("tcp", ContentfulCDNURL+":443", &tls.Config{})
	if err != nil {
		return "", fmt.Errorf("failed to connect: " + err.Error())
	}
	if err := conn.Close(); err != nil {
		return "", err
	}

	out := bytes.Buffer{}
	for _, crt := range conn.ConnectionState().PeerCertificates {
		pem.Encode(&out, &pem.Block{Type: "CERTIFICATE", Bytes: crt.Raw})
	}
	return string(out.Bytes()), nil
}
