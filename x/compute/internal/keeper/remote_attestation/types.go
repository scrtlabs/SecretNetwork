package remote_attestation

import (
	"fmt"
	"strconv"
)

type QuoteReport struct {
	ID                    string   `json:"id"`
	Timestamp             string   `json:"timestamp"`
	Version               int      `json:"version"`
	IsvEnclaveQuoteStatus string   `json:"isvEnclaveQuoteStatus"`
	PlatformInfoBlob      string   `json:"platformInfoBlob"`
	IsvEnclaveQuoteBody   string   `json:"isvEnclaveQuoteBody"`
	AdvisoryIDs           []string `json:"advisoryIDs"`
}

//TODO: add more origin field if needed
type QuoteReportData struct {
	version    int
	signType   int
	reportBody QuoteReportBody
}

//TODO: add more origin filed if needed
type QuoteReportBody struct {
	mrEnclave  string
	mrSigner   string
	reportData string
}

type PlatformInfoBlob struct {
	SgxEpidGroupFlags       uint8             `json:"sgx_epid_group_flags"`
	SgxTcbEvaluationFlags   uint32            `json:"sgx_tcb_evaluation_flags"`
	PseEvaluationFlags      uint32            `json:"pse_evaluation_flags"`
	LatestEquivalentTcbPsvn string            `json:"latest_equivalent_tcb_psvn"`
	LatestPseIsvsvn         string            `json:"latest_pse_isvsvn"`
	LatestPsdaSvn           string            `json:"latest_psda_svn"`
	Xeid                    uint32            `json:"xeid"`
	Gid                     uint32            `json:"gid"`
	SgxEc256SignatureT      SGXEC256Signature `json:"sgx_ec256_signature_t"`
}

type SGXEC256Signature struct {
	Gx string `json:"gx"`
	Gy string `json:"gy"`
}

// directly read from []byte
func parseReport(quoteBytes []byte, quoteHex string) *QuoteReportData {
	qrData := &QuoteReportData{reportBody: QuoteReportBody{}}
	qrData.version = int(quoteBytes[0])
	qrData.signType = int(quoteBytes[2])
	qrData.reportBody.mrEnclave = quoteHex[224:288]
	qrData.reportBody.mrSigner = quoteHex[352:416]
	qrData.reportBody.reportData = quoteHex[736:864]
	return qrData
}

// directly read from []byte
func parsePlatform(piBlobByte []byte) *PlatformInfoBlob {
	piBlob := &PlatformInfoBlob{SgxEc256SignatureT: SGXEC256Signature{}}
	piBlob.SgxEpidGroupFlags = uint8(piBlobByte[0])
	piBlob.SgxTcbEvaluationFlags = computeDec(piBlobByte[1:3])
	piBlob.PseEvaluationFlags = computeDec(piBlobByte[3:5])
	piBlob.LatestEquivalentTcbPsvn = bytesToString(piBlobByte[5:23])
	piBlob.LatestPseIsvsvn = bytesToString(piBlobByte[23:25])
	piBlob.LatestPsdaSvn = bytesToString(piBlobByte[25:29])
	piBlob.Xeid = computeDec(piBlobByte[29:33])
	piBlob.Gid = computeDec(piBlobByte[33:37])
	piBlob.SgxEc256SignatureT.Gx = bytesToString(piBlobByte[37:69])
	piBlob.SgxEc256SignatureT.Gy = bytesToString(piBlobByte[69:])

	return piBlob
}

func computeDec(piBlobSlice []byte) uint32 {
	var hexString string
	for i := len(piBlobSlice) - 1; i >= 0; i-- {
		hexString += fmt.Sprintf("%02x", piBlobSlice[i])
	}
	s, _ := strconv.ParseInt(hexString, 16, 32)

	return uint32(s)
}

func bytesToString(byteSlice []byte) string {
	var byteString string
	for i := 0; i < len(byteSlice); i++ {
		byteString += strconv.Itoa(int(byteSlice[i])) + ", "
	}
	byteString = "[" + byteString[:len(byteString)-2] + "]"
	return byteString
}
