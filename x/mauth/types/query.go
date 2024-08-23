package types

// NewQueryInterchainAccountRequest creates and returns a new QueryInterchainAccountFromAddressRequest
func NewQueryInterchainAccountRequest(owner, connectionID, _ string) *QueryInterchainAccountFromAddressRequest {
	return &QueryInterchainAccountFromAddressRequest{
		Owner:        owner,
		ConnectionId: connectionID,
	}
}

// NewQueryInterchainAccountResponse creates and returns a new QueryInterchainAccountFromAddressResponse
func NewQueryInterchainAccountResponse(interchainAccAddr string) *QueryInterchainAccountFromAddressResponse {
	return &QueryInterchainAccountFromAddressResponse{
		InterchainAccountAddress: interchainAccAddr,
	}
}
