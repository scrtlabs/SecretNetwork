//go:build !secretcli
// +build !secretcli

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// EcallClient fetches ecall records from remote SGX nodes via gRPC
// It maintains connections to multiple nodes and selects randomly for load distribution
type EcallClient struct {
	mu      sync.RWMutex
	nodes   []*nodeConn // Pool of node connections
	timeout time.Duration
	rng     *rand.Rand
}

// nodeConn represents a connection to a single SGX node
type nodeConn struct {
	addr   string
	conn   *grpc.ClientConn
	mu     sync.Mutex
	failed bool // Mark node as failed to avoid repeated connection attempts
}

// sgxNodesConfig represents the JSON configuration file format
type sgxNodesConfig struct {
	Nodes []string `json:"nodes"` // List of gRPC addresses (host:port)
}

// EcallRecordData represents the ecall record for a block
type EcallRecordData struct {
	Height               int64
	RandomSeed           []byte
	ValidatorSetEvidence []byte
}

// Proto message types (matching secret.compute.v1beta1.Query*)
// Defined locally to avoid import cycles

// QueryEcallRecordRequest matches QueryEcallRecordRequest proto
type QueryEcallRecordRequest struct {
	Height int64 `protobuf:"varint,1,opt,name=height,proto3" json:"height,omitempty"`
}

func (m *QueryEcallRecordRequest) Reset()         { *m = QueryEcallRecordRequest{} }
func (m *QueryEcallRecordRequest) String() string { return fmt.Sprintf("{Height:%d}", m.Height) }
func (m *QueryEcallRecordRequest) ProtoMessage()  {}

// QueryEcallRecordResponse matches QueryEcallRecordResponse proto
type QueryEcallRecordResponse struct {
	Height               int64  `protobuf:"varint,1,opt,name=height,proto3" json:"height,omitempty"`
	RandomSeed           []byte `protobuf:"bytes,2,opt,name=random_seed,json=randomSeed,proto3" json:"random_seed,omitempty"`
	ValidatorSetEvidence []byte `protobuf:"bytes,3,opt,name=validator_set_evidence,json=validatorSetEvidence,proto3" json:"validator_set_evidence,omitempty"`
}

func (m *QueryEcallRecordResponse) Reset()         { *m = QueryEcallRecordResponse{} }
func (m *QueryEcallRecordResponse) String() string { return fmt.Sprintf("{Height:%d}", m.Height) }
func (m *QueryEcallRecordResponse) ProtoMessage()  {}

// QueryEncryptedSeedRequest matches QueryEncryptedSeedRequest proto
type QueryEncryptedSeedRequest struct {
	CertHash string `protobuf:"bytes,1,opt,name=cert_hash,json=certHash,proto3" json:"cert_hash,omitempty"`
}

func (m *QueryEncryptedSeedRequest) Reset()         { *m = QueryEncryptedSeedRequest{} }
func (m *QueryEncryptedSeedRequest) String() string { return fmt.Sprintf("{CertHash:%s}", m.CertHash) }
func (m *QueryEncryptedSeedRequest) ProtoMessage()  {}

// QueryEncryptedSeedResponse matches QueryEncryptedSeedResponse proto
type QueryEncryptedSeedResponse struct {
	EncryptedSeed []byte `protobuf:"bytes,1,opt,name=encrypted_seed,json=encryptedSeed,proto3" json:"encrypted_seed,omitempty"`
}

func (m *QueryEncryptedSeedResponse) Reset() { *m = QueryEncryptedSeedResponse{} }
func (m *QueryEncryptedSeedResponse) String() string {
	return fmt.Sprintf("{len:%d}", len(m.EncryptedSeed))
}
func (m *QueryEncryptedSeedResponse) ProtoMessage() {}

// StorageOpProto matches the proto definition for storage operation
type StorageOpProto struct {
	IsDelete bool   `protobuf:"varint,1,opt,name=is_delete,json=isDelete,proto3" json:"is_delete,omitempty"`
	Key      []byte `protobuf:"bytes,2,opt,name=key,proto3" json:"key,omitempty"`
	Value    []byte `protobuf:"bytes,3,opt,name=value,proto3" json:"value,omitempty"`
}

func (m *StorageOpProto) Reset()         { *m = StorageOpProto{} }
func (m *StorageOpProto) String() string { return fmt.Sprintf("{IsDelete:%v}", m.IsDelete) }
func (m *StorageOpProto) ProtoMessage()  {}

// CrossModuleOpProto matches the proto definition for cross-module storage operation
type CrossModuleOpProto struct {
	StoreKey string `protobuf:"bytes,1,opt,name=store_key,json=storeKey,proto3" json:"store_key,omitempty"`
	Key      []byte `protobuf:"bytes,2,opt,name=key,proto3" json:"key,omitempty"`
	Value    []byte `protobuf:"bytes,3,opt,name=value,proto3" json:"value,omitempty"`
	IsDelete bool   `protobuf:"varint,4,opt,name=is_delete,json=isDelete,proto3" json:"is_delete,omitempty"`
}

func (m *CrossModuleOpProto) Reset()         { *m = CrossModuleOpProto{} }
func (m *CrossModuleOpProto) String() string { return fmt.Sprintf("{StoreKey:%s}", m.StoreKey) }
func (m *CrossModuleOpProto) ProtoMessage()  {}

// QueryBlockTracesRequest matches QueryBlockTracesRequest proto
type QueryBlockTracesRequest struct {
	Height int64 `protobuf:"varint,1,opt,name=height,proto3" json:"height,omitempty"`
}

func (m *QueryBlockTracesRequest) Reset()         { *m = QueryBlockTracesRequest{} }
func (m *QueryBlockTracesRequest) String() string { return fmt.Sprintf("{Height:%d}", m.Height) }
func (m *QueryBlockTracesRequest) ProtoMessage()  {}

// ExecutionTraceProto matches the proto definition
type ExecutionTraceProto struct {
	Index       int64                 `protobuf:"varint,1,opt,name=index,proto3" json:"index,omitempty"`
	Ops         []*StorageOpProto     `protobuf:"bytes,2,rep,name=ops,proto3" json:"ops,omitempty"`
	Result      []byte                `protobuf:"bytes,3,opt,name=result,proto3" json:"result,omitempty"`
	GasUsed     uint64                `protobuf:"varint,4,opt,name=gas_used,json=gasUsed,proto3" json:"gas_used,omitempty"`
	CallbackGas uint64                `protobuf:"varint,7,opt,name=callback_gas,json=callbackGas,proto3" json:"callback_gas,omitempty"`
	HasError    bool                  `protobuf:"varint,5,opt,name=has_error,json=hasError,proto3" json:"has_error,omitempty"`
	ErrorMsg    string                `protobuf:"bytes,6,opt,name=error_msg,json=errorMsg,proto3" json:"error_msg,omitempty"`
	CrossOps    []*CrossModuleOpProto `protobuf:"bytes,8,rep,name=cross_ops,json=crossOps,proto3" json:"cross_ops,omitempty"`
}

func (m *ExecutionTraceProto) Reset()         { *m = ExecutionTraceProto{} }
func (m *ExecutionTraceProto) String() string { return fmt.Sprintf("{Index:%d}", m.Index) }
func (m *ExecutionTraceProto) ProtoMessage()  {}

// QueryBlockTracesResponse matches QueryBlockTracesResponse proto
type QueryBlockTracesResponse struct {
	Traces []*ExecutionTraceProto `protobuf:"bytes,1,rep,name=traces,proto3" json:"traces,omitempty"`
}

func (m *QueryBlockTracesResponse) Reset() { *m = QueryBlockTracesResponse{} }
func (m *QueryBlockTracesResponse) String() string {
	return fmt.Sprintf("{NumTraces:%d}", len(m.Traces))
}
func (m *QueryBlockTracesResponse) ProtoMessage() {}

// QueryAnalyzeCodeRequest matches QueryAnalyzeCodeRequest proto
type QueryAnalyzeCodeRequest struct {
	CodeHash []byte `protobuf:"bytes,1,opt,name=code_hash,json=codeHash,proto3" json:"code_hash,omitempty"`
}

func (m *QueryAnalyzeCodeRequest) Reset()         { *m = QueryAnalyzeCodeRequest{} }
func (m *QueryAnalyzeCodeRequest) String() string { return fmt.Sprintf("{CodeHash:%x}", m.CodeHash) }
func (m *QueryAnalyzeCodeRequest) ProtoMessage()  {}

// QueryAnalyzeCodeResponse matches QueryAnalyzeCodeResponse proto
type QueryAnalyzeCodeResponse struct {
	HasIBCEntryPoints bool   `protobuf:"varint,1,opt,name=has_ibc_entry_points,json=hasIbcEntryPoints,proto3" json:"has_ibc_entry_points,omitempty"`
	RequiredFeatures  string `protobuf:"bytes,2,opt,name=required_features,json=requiredFeatures,proto3" json:"required_features,omitempty"`
}

func (m *QueryAnalyzeCodeResponse) Reset()         { *m = QueryAnalyzeCodeResponse{} }
func (m *QueryAnalyzeCodeResponse) String() string { return fmt.Sprintf("{HasIBC:%v}", m.HasIBCEntryPoints) }
func (m *QueryAnalyzeCodeResponse) ProtoMessage()  {}

const (
	methodEcallRecord   = "/secret.compute.v1beta1.Query/EcallRecord"
	methodEncryptedSeed = "/secret.compute.v1beta1.Query/EncryptedSeed"
	methodBlockTraces   = "/secret.compute.v1beta1.Query/BlockTraces"
	methodAnalyzeCode   = "/secret.compute.v1beta1.Query/AnalyzeCode"
)

var (
	globalClient *EcallClient
	clientOnce   sync.Once
)

// Ensure our types implement proto.Message
var (
	_ proto.Message = (*QueryEcallRecordRequest)(nil)
	_ proto.Message = (*QueryEcallRecordResponse)(nil)
	_ proto.Message = (*QueryEncryptedSeedRequest)(nil)
	_ proto.Message = (*QueryEncryptedSeedResponse)(nil)
	_ proto.Message = (*StorageOpProto)(nil)
	_ proto.Message = (*CrossModuleOpProto)(nil)
	_ proto.Message = (*QueryBlockTracesRequest)(nil)
	_ proto.Message = (*QueryBlockTracesResponse)(nil)
	_ proto.Message = (*ExecutionTraceProto)(nil)
	_ proto.Message = (*QueryAnalyzeCodeRequest)(nil)
	_ proto.Message = (*QueryAnalyzeCodeResponse)(nil)
)

// GetEcallClient returns the global ecall client instance
func GetEcallClient() *EcallClient {
	clientOnce.Do(func() {
		var addrs []string

		// Try to load from JSON file first
		configPath := os.Getenv("SECRET_SGX_NODES_CONFIG")
		if configPath == "" {
			// Default path: ~/.secretd/config/sgx_nodes.json
			homeDir := os.Getenv("HOME")
			secretHome := os.Getenv("SECRETD_HOME")
			if secretHome != "" {
				configPath = filepath.Join(secretHome, "config", "sgx_nodes.json")
			} else {
				configPath = filepath.Join(homeDir, ".secretd", "config", "sgx_nodes.json")
			}
		}

		if addrsFromFile := loadNodesFromJSON(configPath); len(addrsFromFile) > 0 {
			addrs = addrsFromFile
			logInfo("EcallClient", "Loaded %d nodes from config file: %s", len(addrs), configPath)
		} else {
			// Fallback to env var
			grpcAddr := os.Getenv("SECRET_SGX_NODE_GRPC")
			if grpcAddr == "" {
				grpcAddr = "localhost:9090"
			}
			addrs = []string{grpcAddr}
			logInfo("EcallClient", "Using single node from env: %s", grpcAddr)
		}

		nodes := make([]*nodeConn, len(addrs))
		for i, addr := range addrs {
			nodes[i] = &nodeConn{addr: addr}
		}

		globalClient = &EcallClient{
			nodes:   nodes,
			timeout: 30 * time.Second,
			rng:     rand.New(rand.NewSource(time.Now().UnixNano())),
		}

		logInfo("EcallClient", "Initialized with %d SGX nodes", len(addrs))
	})
	return globalClient
}

// loadNodesFromJSON loads gRPC node addresses from a JSON configuration file
// JSON format: {"nodes": ["node1:9090", "node2:9090", "node3:9090"]}
func loadNodesFromJSON(configPath string) []string {
	data, err := os.ReadFile(configPath)
	if err != nil {
		// File doesn't exist or can't be read - that's okay, use fallback
		return nil
	}

	var config sgxNodesConfig
	if err := json.Unmarshal(data, &config); err != nil {
		logWarn("EcallClient", "Failed to parse config file %s: %v", configPath, err)
		return nil
	}

	if len(config.Nodes) == 0 {
		return nil
	}

	// Validate and filter out empty addresses
	var validAddrs []string
	for _, addr := range config.Nodes {
		if addr != "" {
			validAddrs = append(validAddrs, addr)
		}
	}

	return validAddrs
}

// getRandomNode returns a random healthy node connection
// It tries up to len(nodes) times to find a working connection
func (c *EcallClient) getRandomNode() (*grpc.ClientConn, string, error) {
	c.mu.RLock()
	numNodes := len(c.nodes)
	if numNodes == 0 {
		c.mu.RUnlock()
		return nil, "", fmt.Errorf("no SGX nodes configured")
	}

	// Create shuffled indices for random selection without replacement
	indices := make([]int, numNodes)
	for i := range indices {
		indices[i] = i
	}
	c.rng.Shuffle(len(indices), func(i, j int) {
		indices[i], indices[j] = indices[j], indices[i]
	})
	c.mu.RUnlock()

	// Try each node in random order until one works
	var lastErr error
	for _, idx := range indices {
		c.mu.RLock()
		if idx >= len(c.nodes) {
			c.mu.RUnlock()
			continue
		}
		node := c.nodes[idx]
		c.mu.RUnlock()

		conn, err := c.ensureConnection(node)
		if err != nil {
			lastErr = err
			continue
		}
		return conn, node.addr, nil
	}

	return nil, "", fmt.Errorf("all SGX nodes unavailable: %w", lastErr)
}

// ensureConnection ensures the node has an active connection, creating one if needed
func (c *EcallClient) ensureConnection(node *nodeConn) (*grpc.ClientConn, error) {
	node.mu.Lock()
	defer node.mu.Unlock()

	// Already connected
	if node.conn != nil {
		return node.conn, nil
	}

	// Try to connect (with short timeout to not block too long)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(
		ctx,
		node.addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		node.failed = true
		return nil, fmt.Errorf("failed to connect to %s: %w", node.addr, err)
	}

	node.conn = conn
	node.failed = false
	return conn, nil
}

// markNodeFailed marks a node as failed after a request error
func (c *EcallClient) markNodeFailed(addr string) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, n := range c.nodes {
		if n.addr == addr {
			n.mu.Lock()
			n.failed = true
			if n.conn != nil {
				n.conn.Close()
				n.conn = nil
			}
			n.mu.Unlock()
			return
		}
	}
}

// Close closes all gRPC connections
func (c *EcallClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, n := range c.nodes {
		n.mu.Lock()
		if n.conn != nil {
			n.conn.Close()
			n.conn = nil
		}
		n.mu.Unlock()
	}
	return nil
}

// invokeWithRetry invokes a gRPC method with automatic retry on different nodes
func (c *EcallClient) invokeWithRetry(method string, req, resp proto.Message) error {
	c.mu.RLock()
	maxRetries := len(c.nodes)
	if maxRetries < 1 {
		maxRetries = 1
	}
	if maxRetries > 5 {
		maxRetries = 5 // Cap retries to avoid too many attempts
	}
	c.mu.RUnlock()

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		conn, nodeAddr, err := c.getRandomNode()
		if err != nil {
			lastErr = err
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
		err = conn.Invoke(ctx, method, req, resp)
		cancel()

		if err == nil {
			return nil
		}

		lastErr = err
		c.markNodeFailed(nodeAddr)
		logWarn("EcallClient", "Request to %s failed (attempt %d/%d): %v", nodeAddr, attempt+1, maxRetries, err)
	}

	return fmt.Errorf("all retry attempts failed: %w", lastErr)
}

// FetchEcallRecord fetches a single ecall record from a random SGX node
func (c *EcallClient) FetchEcallRecord(height int64) (*EcallRecordData, error) {
	req := &QueryEcallRecordRequest{Height: height}
	resp := &QueryEcallRecordResponse{}

	if err := c.invokeWithRetry(methodEcallRecord, req, resp); err != nil {
		return nil, fmt.Errorf("gRPC EcallRecord failed for height %d: %w", height, err)
	}

	if height%1000 == 0 {
		logInfo("EcallClient", "Fetched ecall record for height %d", height)
	}

	return &EcallRecordData{
		Height:               resp.Height,
		RandomSeed:           resp.RandomSeed,
		ValidatorSetEvidence: resp.ValidatorSetEvidence,
	}, nil
}

// FetchEncryptedSeed fetches encrypted seed data from a random SGX node
func (c *EcallClient) FetchEncryptedSeed(certHashHex string) ([]byte, error) {
	req := &QueryEncryptedSeedRequest{CertHash: certHashHex}
	resp := &QueryEncryptedSeedResponse{}

	if err := c.invokeWithRetry(methodEncryptedSeed, req, resp); err != nil {
		return nil, fmt.Errorf("gRPC EncryptedSeed failed: %w", err)
	}

	logInfo("EcallClient", "Fetched encrypted seed (%d bytes)", len(resp.EncryptedSeed))
	return resp.EncryptedSeed, nil
}

// FetchBlockTraces fetches all execution traces for a block from a random SGX node
func (c *EcallClient) FetchBlockTraces(height int64) ([]*ExecutionTrace, error) {
	req := &QueryBlockTracesRequest{Height: height}
	resp := &QueryBlockTracesResponse{}

	if err := c.invokeWithRetry(methodBlockTraces, req, resp); err != nil {
		return nil, fmt.Errorf("gRPC BlockTraces failed for height %d: %w", height, err)
	}

	// Convert proto response to ExecutionTrace slice
	traces := make([]*ExecutionTrace, len(resp.Traces))
	for i, t := range resp.Traces {
		logDebug("EcallClient", "Proto trace callbackGas=%d (from gRPC response)", t.CallbackGas)
		ops := make([]StorageOp, len(t.Ops))
		for j, op := range t.Ops {
			value := op.Value
			if !op.IsDelete && value == nil {
				value = []byte{}
			}
			ops[j] = StorageOp{
				IsDelete: op.IsDelete,
				Key:      op.Key,
				Value:    value,
			}
		}
		// Convert cross-module ops (e.g., distribution store writes from staking queries)
		crossOps := make([]CrossModuleOp, len(t.CrossOps))
		for j, cop := range t.CrossOps {
			crossOps[j] = CrossModuleOp{
				StoreKey: cop.StoreKey,
				Key:      cop.Key,
				Value:    cop.Value,
				IsDelete: cop.IsDelete,
			}
		}
		traces[i] = &ExecutionTrace{
			Index:       t.Index,
			Ops:         ops,
			CrossOps:    crossOps,
			Result:      t.Result,
			GasUsed:     t.GasUsed,
			CallbackGas: t.CallbackGas,
			HasError:    t.HasError,
			ErrorMsg:    t.ErrorMsg,
		}
		logDebug("EcallClient", "Converted trace callbackGas=%d crossOps=%d", traces[i].CallbackGas, len(crossOps))
	}

	if len(traces) > 0 {
		for _, t := range traces {
			logDebug("EcallClient", "Fetched trace: height=%d index=%d ops=%d resultLen=%d gasUsed=%d callbackGas=%d hasError=%v",
				height, t.Index, len(t.Ops), len(t.Result), t.GasUsed, t.CallbackGas, t.HasError)
		}
	} else if height%1000 == 0 {
		logInfo("EcallClient", "Fetched %d traces for block %d", len(traces), height)
	}
	return traces, nil
}

// FetchAnalyzeCode fetches the AnalyzeCode result for a code hash from a random SGX node
func (c *EcallClient) FetchAnalyzeCode(codeHash []byte) (bool, string, error) {
	req := &QueryAnalyzeCodeRequest{CodeHash: codeHash}
	resp := &QueryAnalyzeCodeResponse{}

	if err := c.invokeWithRetry(methodAnalyzeCode, req, resp); err != nil {
		return false, "", fmt.Errorf("gRPC AnalyzeCode failed for code hash %x: %w", codeHash, err)
	}

	logInfo("EcallClient", "Fetched AnalyzeCode for %x: hasIBC=%v features=%s", codeHash, resp.HasIBCEntryPoints, resp.RequiredFeatures)
	return resp.HasIBCEntryPoints, resp.RequiredFeatures, nil
}

// IsConnected returns true if at least one node is connected
func (c *EcallClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, n := range c.nodes {
		n.mu.Lock()
		connected := n.conn != nil
		n.mu.Unlock()
		if connected {
			return true
		}
	}
	return false
}
