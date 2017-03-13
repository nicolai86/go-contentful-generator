package main

import (
	"bytes"
	"crypto/tls"
	"encoding/pem"
	"fmt"
)

func fetchCerts() (string, error) {
	out := bytes.Buffer{}

	endpoints := []string{cdaEndpoint, cpaEndpoint, cmaEndpoint}
	for _, endpoint := range endpoints {
		conn, err := tls.Dial("tcp", endpoint+":443", &tls.Config{})
		if err != nil {
			return "", fmt.Errorf("failed to connect: " + err.Error())
		}
		if err := conn.Close(); err != nil {
			return "", err
		}

		for _, crt := range conn.ConnectionState().PeerCertificates {
			pem.Encode(&out, &pem.Block{Type: "CERTIFICATE", Bytes: crt.Raw})
		}
	}

	return string(out.Bytes()), nil
}
