package remote_attestation

import (
	"bytes"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"log"
)

/*
 Verifies the remote attestation certificate, which is comprised of a the attestation report, intel signature, and enclave signature

 We verify that:
	- the report is valid, that no outstanding issues exist (todo: match enclave hash or something?)
	- Intel's certificate signed the report
	- The public key of the enclave/node exists, so we can use that to encrypt the seed

*/

/*
 Verifies the remote attestation certificate, which is comprised of a the attestation report, intel signature, and enclave signature

 We verify that:
	- the report is valid, that no outstanding issues exist (todo: match enclave hash or something?)
	- Intel's certificate signed the report
	- The public key of the enclave/node exists, so we can use that to encrypt the seed

*/
func VerifyRaCert(rawCert []byte) ([]byte, error) {
	// printCert(rawCert)

	// get the pubkey and payload from raw data
	pubK, payload := unmarshalCert(rawCert)

	// Load Intel CA, Verify Cert and Signature
	attnReportRaw, err := verifyCert(payload)
	if err != nil {
		log.Fatalln(err)
		return nil, err
	}

	// Verify attestation report
	pubK, err = verifyAttReport(attnReportRaw, pubK)
	if err != nil {
		log.Fatalln(err)
		return nil, err
	}

	return pubK, nil
}

func unmarshalCert(rawbyte []byte) ([]byte, []byte) {
	// Search for Public Key prime256v1 OID
	prime256v1Oid := []byte{0x06, 0x08, 0x2A, 0x86, 0x48, 0xCE, 0x3D, 0x03, 0x01, 0x07}
	offset := uint(bytes.Index(rawbyte, prime256v1Oid))
	offset += 11 // 10 + TAG (0x03)

	// Obtain Public Key length
	length := uint(rawbyte[offset])
	if length > 0x80 {
		length = uint(rawbyte[offset+1])*uint(0x100) + uint(rawbyte[offset+2])
		offset += 2
	}

	// Obtain Public Key
	offset += 1
	pubK := rawbyte[offset+2 : offset+length] // skip "00 04"

	// Search for Netscape Comment OID
	nsCmtOid := []byte{0x06, 0x09, 0x60, 0x86, 0x48, 0x01, 0x86, 0xF8, 0x42, 0x01, 0x0D}
	offset = uint(bytes.Index(rawbyte, nsCmtOid))
	offset += 12 // 11 + TAG (0x04)

	// Obtain Netscape Comment length
	length = uint(rawbyte[offset])
	if length > 0x80 {
		length = uint(rawbyte[offset+1])*uint(0x100) + uint(rawbyte[offset+2])
		offset += 2
	}

	// Obtain Netscape Comment
	offset += 1
	payload := rawbyte[offset : offset+length]
	return pubK, payload
}

func verifyCert(payload []byte) ([]byte, error) {
	// Extract each field
	plSplit := bytes.Split(payload, []byte{0x7C}) // '|'
	attnReportRaw := plSplit[0]
	sigRaw := plSplit[1]

	var sig, sigCertDec []byte
	sig, err := base64.StdEncoding.DecodeString(string(sigRaw))
	if err != nil {
		log.Fatalln(err)
		return nil, err
	}

	sigCertRaw := plSplit[2]
	sigCertDec, err = base64.StdEncoding.DecodeString(string(sigCertRaw))
	if err != nil {
		log.Fatalln(err)
		return nil, err
	}

	certServer, err := x509.ParseCertificate(sigCertDec)
	if err != nil {
		log.Fatalln(err)
		return nil, err
	}

	roots := x509.NewCertPool()
	//cacert, err := readFile("./remote_attestation/Intel_SGX_Attestation_RootCA.pem")
	//if err != nil {
	//	log.Fatalln(err)
	//	return nil, err
	//}
	ok := roots.AppendCertsFromPEM([]byte(rootIntelPEM))
	if !ok {
		panic("failed to parse root certificate")
	}

	opts := x509.VerifyOptions{
		Roots: roots,
	}

	if _, err := certServer.Verify(opts); err != nil {
		log.Fatalln(err)
		return nil, err
	} else {
		//fmt.Println("Cert is good")
	}

	// Verify the signature against the signing cert
	err = certServer.CheckSignature(certServer.SignatureAlgorithm, attnReportRaw, sig)
	if err != nil {
		log.Fatalln(err)
		return nil, err
	} else {
		//fmt.Println("Signature good")
	}
	return attnReportRaw, nil
}

func verifyAttReport(attnReportRaw []byte, pubK []byte) ([]byte, error) {
	var qr QuoteReport
	err := json.Unmarshal(attnReportRaw, &qr)
	if err != nil {
		return nil, err
	}

	// 1. Check timestamp is within 24H
	if qr.Timestamp != "" {
		//timeFixed := qr.Timestamp + "+0000"
		//timeFixed := qr.Timestamp + "Z"
		//ts, _ := time.Parse(time.RFC3339, timeFixed)
		//now := time.Now().Unix()
		//fmt.Println("Time diff = ", now-ts.Unix())
	} else {
		return nil, errors.New("Failed to fetch timestamp from attestation report")
	}

	// 2. Verify quote status (mandatory field)
	if qr.IsvEnclaveQuoteStatus != "" {
		//fmt.Println("isvEnclaveQuoteStatus = ", qr.IsvEnclaveQuoteStatus)
		switch qr.IsvEnclaveQuoteStatus {
		case "OK":
			break
		case "GROUP_OUT_OF_DATE", "GROUP_REVOKED", "CONFIGURATION_NEEDED", "CONFIGURATION_AND_SW_HARDENING_NEEDED":
			// Verify platformInfoBlob for further info if status not OK
			if qr.PlatformInfoBlob != "" {
				platInfo, err := hex.DecodeString(qr.PlatformInfoBlob)
				if err != nil && len(platInfo) != 105 {
					return nil, errors.New("illegal PlatformInfoBlob")
				}
				platInfo = platInfo[4:]

				//piBlob := parsePlatform(platInfo)
				//piBlobJson, err := json.Marshal(piBlob)
				//if err != nil {
				//	return nil, err
				//}
				//fmt.Println("Platform info is: " + string(piBlobJson))
			} else {
				return nil, errors.New("Failed to fetch platformInfoBlob from attestation report")
			}
			if len(qr.AdvisoryIDs) != 0 {
				_, err := json.Marshal(qr.AdvisoryIDs)
				if err != nil {
					return nil, err
				}
				//fmt.Println("Warning - Advisory IDs: " + string(cves))
			}
			//return nil, errors.New("Quote status invalid")
		case "SW_HARDENING_NEEDED":
			if len(qr.AdvisoryIDs) != 0 {
				_, err := json.Marshal(qr.AdvisoryIDs)
				if err != nil {
					return nil, err
				}
				// fmt.Println("Advisory IDs: " + string(cves))
				// return nil, errors.New("Platform is vulnerable, and requires updates before authorization: " + string(cves))
			} else {
				return nil, errors.New("Failed to fetch advisory IDs even though platform is vulnerable")
			}

		default:
			return nil, errors.New("SGX_ERROR_UNEXPECTED")
		}
	} else {
		err := errors.New("Failed to fetch isvEnclaveQuoteStatus from attestation report")
		return nil, err
	}

	// 3. Verify quote body (mandatory field)
	if qr.IsvEnclaveQuoteBody != "" {
		qb, err := base64.StdEncoding.DecodeString(qr.IsvEnclaveQuoteBody)
		if err != nil {
			return nil, err
		}

		var quoteBytes, quoteHex, pubHex string
		for _, b := range qb {
			quoteBytes += fmt.Sprint(int(b), ", ")
			quoteHex += fmt.Sprintf("%02x", int(b))
		}

		for _, b := range pubK {
			pubHex += fmt.Sprintf("%02x", int(b))
		}

		qrData := parseReport(qb, quoteHex)

		// todo: possibly verify mr signer/enclave?
		//fmt.Println("Quote = [" + quoteBytes[:len(quoteBytes)-2] + "]")
		//fmt.Println("sgx quote version = ", qrData.version)
		//fmt.Println("sgx quote signature type = ", qrData.signType)
		//fmt.Println("sgx quote report_data = ", qrData.reportBody.reportData)
		//fmt.Println("sgx quote mr_enclave = ", qrData.reportBody.mrEnclave)
		//fmt.Println("sgx quote mr_signer = ", qrData.reportBody.mrSigner)
		//fmt.Println("Anticipated public key = ", pubHex)

		if qrData.reportBody.reportData != pubHex {
			// err := errors.New("Failed to authenticate certificate public key")
			reportPubKey, err := hex.DecodeString(qrData.reportBody.reportData)
			if err != nil {
				return nil, err
			}
			return reportPubKey, nil
		}
	} else {
		err := errors.New("Failed to fetch isvEnclaveQuoteBody from attestation report")
		return nil, err
	}
	return pubK, nil
}
