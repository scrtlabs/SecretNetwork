//go:build !secretcli
// +build !secretcli

package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	dcrdecdsa "github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"github.com/gogo/protobuf/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// EcallClient fetches ecall records from remote SGX nodes via gRPC
// It maintains connections to multiple nodes and selects randomly for load distribution
type EcallClient struct {
	mu             sync.RWMutex
	nodes          []*nodeConn // Pool of node connections
	timeout        time.Duration
	rng            *rand.Rand
	billingPrivKey *secp256k1.PrivateKey // loaded from hex file for billing sidecar auth
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
	Height   int64  `protobuf:"varint,2,opt,name=height,proto3" json:"height,omitempty"`
}

func (m *QueryEncryptedSeedRequest) Reset() { *m = QueryEncryptedSeedRequest{} }
func (m *QueryEncryptedSeedRequest) String() string {
	return fmt.Sprintf("{CertHash:%s,Height:%d}", m.CertHash, m.Height)
}
func (m *QueryEncryptedSeedRequest) ProtoMessage() {}

// QueryEncryptedSeedResponse matches QueryEncryptedSeedResponse proto
type QueryEncryptedSeedResponse struct {
	EncryptedSeed  []byte `protobuf:"bytes,1,opt,name=encrypted_seed,json=encryptedSeed,proto3" json:"encrypted_seed,omitempty"`
	MachineBinding []byte `protobuf:"bytes,2,opt,name=machine_binding,json=machineBinding,proto3" json:"machine_binding,omitempty"`
}

func (m *QueryEncryptedSeedResponse) Reset() { *m = QueryEncryptedSeedResponse{} }
func (m *QueryEncryptedSeedResponse) String() string {
	return fmt.Sprintf("{len:%d}", len(m.EncryptedSeed))
}
func (m *QueryEncryptedSeedResponse) ProtoMessage() {}

// QueryNetworkPubkeyRequest matches QueryNetworkPubkeyRequest proto
type QueryNetworkPubkeyRequest struct {
	Height int64  `protobuf:"varint,1,opt,name=height,proto3" json:"height,omitempty"`
	ISeed  uint32 `protobuf:"varint,2,opt,name=i_seed,json=iSeed,proto3" json:"i_seed,omitempty"`
}

func (m *QueryNetworkPubkeyRequest) Reset() { *m = QueryNetworkPubkeyRequest{} }
func (m *QueryNetworkPubkeyRequest) String() string {
	return fmt.Sprintf("{Height:%d,ISeed:%d}", m.Height, m.ISeed)
}
func (m *QueryNetworkPubkeyRequest) ProtoMessage() {}

// QueryNetworkPubkeyResponse matches QueryNetworkPubkeyResponse proto
type QueryNetworkPubkeyResponse struct {
	NodePubkey []byte `protobuf:"bytes,1,opt,name=node_pubkey,json=nodePubkey,proto3" json:"node_pubkey,omitempty"`
	IoPubkey   []byte `protobuf:"bytes,2,opt,name=io_pubkey,json=ioPubkey,proto3" json:"io_pubkey,omitempty"`
}

func (m *QueryNetworkPubkeyResponse) Reset() { *m = QueryNetworkPubkeyResponse{} }
func (m *QueryNetworkPubkeyResponse) String() string {
	return fmt.Sprintf("{len(node):%d,len(io):%d}", len(m.NodePubkey), len(m.IoPubkey))
}
func (m *QueryNetworkPubkeyResponse) ProtoMessage() {}

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

func (m *QueryAnalyzeCodeResponse) Reset() { *m = QueryAnalyzeCodeResponse{} }
func (m *QueryAnalyzeCodeResponse) String() string {
	return fmt.Sprintf("{HasIBC:%v}", m.HasIBCEntryPoints)
}
func (m *QueryAnalyzeCodeResponse) ProtoMessage() {}

// QueryMachineIDProofRequest matches QueryMachineIDProofRequest proto
type QueryMachineIDProofRequest struct {
	Height    int64  `protobuf:"varint,1,opt,name=height,proto3" json:"height,omitempty"`
	MachineId string `protobuf:"bytes,2,opt,name=machine_id,json=machineId,proto3" json:"machine_id,omitempty"`
}

func (m *QueryMachineIDProofRequest) Reset() { *m = QueryMachineIDProofRequest{} }
func (m *QueryMachineIDProofRequest) String() string {
	return fmt.Sprintf("{Height:%d,MachineId:%s}", m.Height, m.MachineId)
}
func (m *QueryMachineIDProofRequest) ProtoMessage() {}

// QueryMachineIDProofResponse matches QueryMachineIDProofResponse proto
type QueryMachineIDProofResponse struct {
	Proof []byte `protobuf:"bytes,1,opt,name=proof,proto3" json:"proof,omitempty"`
}

func (m *QueryMachineIDProofResponse) Reset()         { *m = QueryMachineIDProofResponse{} }
func (m *QueryMachineIDProofResponse) String() string { return fmt.Sprintf("{len:%d}", len(m.Proof)) }
func (m *QueryMachineIDProofResponse) ProtoMessage()  {}

// CreateResultDataProto matches the proto definition for a Create (MsgStoreCode) result
type CreateResultDataProto struct {
	WasmHash []byte `protobuf:"bytes,1,opt,name=wasm_hash,json=wasmHash,proto3" json:"wasm_hash,omitempty"`
	CodeHash []byte `protobuf:"bytes,2,opt,name=code_hash,json=codeHash,proto3" json:"code_hash,omitempty"`
	HasError bool   `protobuf:"varint,3,opt,name=has_error,json=hasError,proto3" json:"has_error,omitempty"`
	ErrorMsg string `protobuf:"bytes,4,opt,name=error_msg,json=errorMsg,proto3" json:"error_msg,omitempty"`
}

func (m *CreateResultDataProto) Reset()         { *m = CreateResultDataProto{} }
func (m *CreateResultDataProto) String() string { return fmt.Sprintf("{WasmHash:%x}", m.WasmHash) }
func (m *CreateResultDataProto) ProtoMessage()  {}

// QueryBlockCreateResultsRequest matches QueryBlockCreateResultsRequest proto
type QueryBlockCreateResultsRequest struct {
	Height int64 `protobuf:"varint,1,opt,name=height,proto3" json:"height,omitempty"`
}

func (m *QueryBlockCreateResultsRequest) Reset()         { *m = QueryBlockCreateResultsRequest{} }
func (m *QueryBlockCreateResultsRequest) String() string { return fmt.Sprintf("{Height:%d}", m.Height) }
func (m *QueryBlockCreateResultsRequest) ProtoMessage()  {}

// QueryBlockCreateResultsResponse matches QueryBlockCreateResultsResponse proto
type QueryBlockCreateResultsResponse struct {
	Results []*CreateResultDataProto `protobuf:"bytes,1,rep,name=results,proto3" json:"results,omitempty"`
}

func (m *QueryBlockCreateResultsResponse) Reset() { *m = QueryBlockCreateResultsResponse{} }
func (m *QueryBlockCreateResultsResponse) String() string {
	return fmt.Sprintf("{NumResults:%d}", len(m.Results))
}
func (m *QueryBlockCreateResultsResponse) ProtoMessage() {}

const (
	methodEcallRecord        = "/secret.compute.v1beta1.Query/EcallRecord"
	methodEncryptedSeed      = "/secret.compute.v1beta1.Query/EncryptedSeed"
	methodBlockTraces        = "/secret.compute.v1beta1.Query/BlockTraces"
	methodAnalyzeCode        = "/secret.compute.v1beta1.Query/AnalyzeCode"
	methodMachineIDProof     = "/secret.compute.v1beta1.Query/MachineIDProof"
	methodBlockCreateResults = "/secret.compute.v1beta1.Query/BlockCreateResults"
	methodNetworkPubkey      = "/secret.compute.v1beta1.Query/NetworkPubkey"
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
	_ proto.Message = (*CreateResultDataProto)(nil)
	_ proto.Message = (*QueryBlockCreateResultsRequest)(nil)
	_ proto.Message = (*QueryBlockCreateResultsResponse)(nil)
	_ proto.Message = (*QueryNetworkPubkeyRequest)(nil)
	_ proto.Message = (*QueryNetworkPubkeyResponse)(nil)
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
			// logInfo("EcallClient", "Loaded %d nodes from config file: %s", len(addrs), configPath)
		} else {
			// Fallback to env var
			grpcAddr := os.Getenv("SECRET_SGX_NODE_GRPC")
			if grpcAddr == "" {
				grpcAddr = "localhost:9090"
			}
			addrs = []string{grpcAddr}
			// logInfo("EcallClient", "Using single node from env: %s", grpcAddr)
		}

		nodes := make([]*nodeConn, len(addrs))
		for i, addr := range addrs {
			nodes[i] = &nodeConn{addr: addr}
		}

		// Load billing key from hex file
		var billingPrivKey *secp256k1.PrivateKey
		keyFile := os.Getenv("SECRET_BILLING_KEY_FILE")
		if keyFile == "" {
			homeDir := os.Getenv("HOME")
			defaultPath := filepath.Join(homeDir, ".secretd-billing", "key.hex")
			if _, err := os.Stat(defaultPath); err == nil {
				keyFile = defaultPath
			}
		}
		if keyFile != "" {
			data, err := os.ReadFile(keyFile)
			if err != nil {
				logWarn("EcallClient", "Failed to read billing key file %s: %v", keyFile, err)
			} else {
				keyHex := strings.TrimSpace(string(data))
				keyBytes, err := hex.DecodeString(keyHex)
				if err != nil {
					logWarn("EcallClient", "Invalid hex in billing key file %s: %v", keyFile, err)
				} else {
					billingPrivKey = secp256k1.PrivKeyFromBytes(keyBytes)
					logInfo("EcallClient", "Loaded billing key from %s", keyFile)
				}
			}
		}

		globalClient = &EcallClient{
			nodes:          nodes,
			timeout:        30 * time.Second,
			rng:            rand.New(rand.NewSource(time.Now().UnixNano())),
			billingPrivKey: billingPrivKey,
		}

		// logInfo("EcallClient", "Initialized with %d SGX nodes", len(addrs))
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

	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	if c.billingPrivKey != nil {
		dialOpts = append(dialOpts, grpc.WithUnaryInterceptor(c.billingAuthInterceptor()))
	}

	conn, err := grpc.DialContext(
		ctx,
		node.addr,
		dialOpts...,
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

		// Don't retry on non-transient gRPC errors
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.FailedPrecondition, codes.NotFound, codes.InvalidArgument, codes.PermissionDenied:
				return err // Return immediately, don't retry semantic errors
			}
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
		// logInfo("EcallClient", "Fetched ecall record for height %d", height)
	}

	return &EcallRecordData{
		Height:               resp.Height,
		RandomSeed:           resp.RandomSeed,
		ValidatorSetEvidence: resp.ValidatorSetEvidence,
	}, nil
}

// FetchEncryptedSeed fetches encrypted seed data from a random SGX node
func (c *EcallClient) FetchEncryptedSeed(height int64, certHashHex string) ([]byte, []byte, error) {
	req := &QueryEncryptedSeedRequest{CertHash: certHashHex, Height: height}
	resp := &QueryEncryptedSeedResponse{}

	if err := c.invokeWithRetry(methodEncryptedSeed, req, resp); err != nil {
		return nil, nil, err // Return raw error to preserve gRPC status codes
	}

	// logInfo("EcallClient", "Fetched encrypted seed (%d bytes) at height %d", len(resp.EncryptedSeed), height)
	return resp.EncryptedSeed, resp.MachineBinding, nil
}

// FetchMachineIDProof fetches a machine ID proof for a given height and machine ID from a random SGX node
func (c *EcallClient) FetchMachineIDProof(height int64, machineIDHex string) ([]byte, error) {
	req := &QueryMachineIDProofRequest{Height: height, MachineId: machineIDHex}
	resp := &QueryMachineIDProofResponse{}

	if err := c.invokeWithRetry(methodMachineIDProof, req, resp); err != nil {
		return nil, fmt.Errorf("gRPC MachineIDProof failed for height %d: %w", height, err)
	}

	// logInfo("EcallClient", "Fetched machine ID proof (%d bytes) for height %d", len(resp.Proof), height)
	return resp.Proof, nil
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
		// logInfo("EcallClient", "Fetched %d traces for block %d", len(traces), height)
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

	// logInfo("EcallClient", "Fetched AnalyzeCode for %x: hasIBC=%v features=%s", codeHash, resp.HasIBCEntryPoints, resp.RequiredFeatures)
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

// FetchBlockCreateResults fetches all Create (MsgStoreCode) results for a block from a random SGX node
func (c *EcallClient) FetchBlockCreateResults(height int64) ([]*CreateResult, [][]byte, error) {
	req := &QueryBlockCreateResultsRequest{Height: height}
	resp := &QueryBlockCreateResultsResponse{}

	if err := c.invokeWithRetry(methodBlockCreateResults, req, resp); err != nil {
		return nil, nil, fmt.Errorf("gRPC BlockCreateResults failed for height %d: %w", height, err)
	}

	results := make([]*CreateResult, len(resp.Results))
	wasmHashes := make([][]byte, len(resp.Results))
	for i, r := range resp.Results {
		wasmHashes[i] = r.WasmHash
		results[i] = &CreateResult{
			CodeHash: r.CodeHash,
			HasError: r.HasError,
			ErrorMsg: r.ErrorMsg,
		}
	}

	if len(results) > 0 {
		// logInfo("EcallClient", "Fetched %d Create results for block %d", len(results), height)
	}
	return results, wasmHashes, nil
}

// FetchNetworkPubkey fetches a stored network pubkey from a random SGX node
func (c *EcallClient) FetchNetworkPubkey(height int64, iSeed uint32) ([]byte, []byte, error) {
	req := &QueryNetworkPubkeyRequest{Height: height, ISeed: iSeed}
	resp := &QueryNetworkPubkeyResponse{}

	if err := c.invokeWithRetry(methodNetworkPubkey, req, resp); err != nil {
		return nil, nil, fmt.Errorf("gRPC NetworkPubkey failed for height %d seed %d: %w", height, iSeed, err)
	}

	// logInfo("EcallClient", "Fetched NetworkPubkey for height %d seed %d", height, iSeed)
	return resp.NodePubkey, resp.IoPubkey, nil
}

// billingAuthInterceptor automatically signs the request payload using the loaded private key.
// The signature allows the billing sidecar to authenticate the client and debit their subscription balance.
func (c *EcallClient) billingAuthInterceptor() grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		if c.billingPrivKey != nil {
			timestamp := strconv.FormatInt(time.Now().Unix(), 10)
			payload := timestamp + "|" + method
			hash := sha256.Sum256([]byte(payload))

			// Sign with secp256k1
			sig := dcrdecdsa.Sign(c.billingPrivKey, hash[:])

			// 64-byte compact R || S format
			var sigBytes [64]byte
			r := sig.R()
			s := sig.S()
			rBytes := r.Bytes()
			sBytes := s.Bytes()
			copy(sigBytes[0:32], rBytes[:])
			copy(sigBytes[32:64], sBytes[:])

			// 33-byte compressed pubkey
			pubKey := c.billingPrivKey.PubKey()
			pubKeyBytes := pubKey.SerializeCompressed()

			md := metadata.Pairs(
				"x-sub-timestamp", timestamp,
				"x-sub-pubkey", hex.EncodeToString(pubKeyBytes),
				"x-sub-signature", hex.EncodeToString(sigBytes[:]),
			)
			ctx = metadata.NewOutgoingContext(ctx, md)
		}

		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
