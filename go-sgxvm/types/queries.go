package types

// Connector contains single method Query that is used to allow EVM to interact with Cosmos SDK
type Connector interface {
	// Query is called by EVM to obtain necessary data or make some changes in DB
	Query(request []byte) ([]byte, error)
}
