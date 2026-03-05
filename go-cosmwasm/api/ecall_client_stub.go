//go:build secretcli
// +build secretcli

package api

// Stub implementations for secretcli builds (no SGX support)

// EcallClient stub for secretcli
type EcallClient struct{}

func GetEcallClient() *EcallClient                               { return nil }
func (c *EcallClient) FetchEncryptedSeed(string) ([]byte, error) { return nil, nil }
func (c *EcallClient) FetchBlockStreams(int64) (map[int64][]byte, error) {
	return nil, nil
}
func (c *EcallClient) Close() error             { return nil }
func (c *EcallClient) SetGrpcAddr(string) error { return nil }
func (c *EcallClient) IsConnected() bool        { return false }
