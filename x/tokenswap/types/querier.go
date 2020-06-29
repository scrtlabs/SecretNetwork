package types

// query endpoints supported by the Querier
const (
	GetTokenSwapRoute = "get"
)

// GetTokenSwapParams defines the params for the following queries:
// - 'custom/tokenswap/get/'
type GetTokenSwapParams struct {
	EthereumTxHash EthereumTxHash `json:"ethereum_tx_hash"`
}

// NewGetTokenSwapParams creates a new GetTokenSwapParams
func NewGetTokenSwapParams(ethereumTxHash EthereumTxHash) GetTokenSwapParams {
	return GetTokenSwapParams{
		EthereumTxHash: ethereumTxHash,
	}
}
