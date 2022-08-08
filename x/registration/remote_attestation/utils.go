package remote_attestation

import (
	"fmt"
	io "io"
	"log"
	"os"
)

func isSgxHardwareMode() bool {
	return os.Getenv("SGX_MODE") != "SW"
}

func printCert(rawByte []byte) {
	print("--received-server cert: [Certificate(b\"")
	for _, b := range rawByte {
		if b == '\n' { //nolint:gocritic
			print("\\n")
		} else if b == '\r' {
			print("\\r")
		} else if b == '\t' {
			print("\\t")
		} else if b == '\\' || b == '"' {
			print("\\", string(rune(b)))
		} else if b >= 0x20 && b < 0x7f {
			print(string(rune(b)))
		} else {
			fmt.Printf("\\x%02x\n", int(b))
		}
	}
	println("\")]")
}

func loadCert() (string, string) {
	certPem, err := readFile("./../../cert/client.crt")
	if err != nil {
		log.Fatalln(err)
	}

	keyPEM, err := readFile("./../../cert/client.pkcs8")
	if err != nil {
		log.Fatalln(err)
	}
	return certPem, keyPEM
}

func readFile(filePth string) (string, error) {
	f, err := os.Open(filePth)
	if err != nil {
		return "", err
	}
	content, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}
	return string(content), nil
}
