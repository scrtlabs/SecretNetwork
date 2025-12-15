//go:build secretcli
// +build secretcli

package api

// Stub implementations for secretcli builds (no SGX support)

// EcallClient stub for secretcli
type EcallClient struct{}

// EcallRecordData stub
type EcallRecordData struct {
	Height               int64
	RandomSeed           []byte
	ValidatorSetEvidence []byte
}

func GetEcallClient() *EcallClient                                       { return nil }
func (c *EcallClient) FetchEcallRecord(int64) (*EcallRecordData, error)  { return nil, nil }
func (c *EcallClient) FetchEncryptedSeed(string) ([]byte, error)         { return nil, nil }
func (c *EcallClient) FetchBlockTraces(int64) ([]*ExecutionTrace, error) { return nil, nil }
func (c *EcallClient) Close() error                                      { return nil }
func (c *EcallClient) SetGrpcAddr(string) error                          { return nil }
func (c *EcallClient) IsConnected() bool                                 { return false }
