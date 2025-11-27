//go:build !secretcli
// +build !secretcli

package api

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// EcallClient fetches ecall records from a remote SGX node via gRPC
type EcallClient struct {
	mu       sync.Mutex // Protects conn during reconnect
	grpcAddr string
	conn     *grpc.ClientConn
	timeout  time.Duration
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

// QueryBlockTracesRequest matches QueryBlockTracesRequest proto
type QueryBlockTracesRequest struct {
	Height int64 `protobuf:"varint,1,opt,name=height,proto3" json:"height,omitempty"`
}

func (m *QueryBlockTracesRequest) Reset()         { *m = QueryBlockTracesRequest{} }
func (m *QueryBlockTracesRequest) String() string { return fmt.Sprintf("{Height:%d}", m.Height) }
func (m *QueryBlockTracesRequest) ProtoMessage()  {}

// ExecutionTraceProto matches the proto definition
type ExecutionTraceProto struct {
	Index       int64             `protobuf:"varint,1,opt,name=index,proto3" json:"index,omitempty"`
	Ops         []*StorageOpProto `protobuf:"bytes,2,rep,name=ops,proto3" json:"ops,omitempty"`
	Result      []byte            `protobuf:"bytes,3,opt,name=result,proto3" json:"result,omitempty"`
	GasUsed     uint64            `protobuf:"varint,4,opt,name=gas_used,json=gasUsed,proto3" json:"gas_used,omitempty"`
	CallbackGas uint64            `protobuf:"varint,7,opt,name=callback_gas,json=callbackGas,proto3" json:"callback_gas,omitempty"`
	HasError    bool              `protobuf:"varint,5,opt,name=has_error,json=hasError,proto3" json:"has_error,omitempty"`
	ErrorMsg    string            `protobuf:"bytes,6,opt,name=error_msg,json=errorMsg,proto3" json:"error_msg,omitempty"`
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

const (
	methodEcallRecord   = "/secret.compute.v1beta1.Query/EcallRecord"
	methodEncryptedSeed = "/secret.compute.v1beta1.Query/EncryptedSeed"
	methodBlockTraces   = "/secret.compute.v1beta1.Query/BlockTraces"
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
	_ proto.Message = (*QueryBlockTracesRequest)(nil)
	_ proto.Message = (*QueryBlockTracesResponse)(nil)
	_ proto.Message = (*ExecutionTraceProto)(nil)
)

// GetEcallClient returns the global ecall client instance
func GetEcallClient() *EcallClient {
	clientOnce.Do(func() {
		grpcAddr := os.Getenv("SECRET_SGX_NODE_GRPC")
		if grpcAddr == "" {
			grpcAddr = "localhost:9090"
		}

		globalClient = &EcallClient{
			grpcAddr: grpcAddr,
			timeout:  30 * time.Second,
		}

		if err := globalClient.connect(); err != nil {
			fmt.Printf("[EcallClient] Warning: failed to connect to gRPC server: %v\n", err)
		} else {
			fmt.Printf("[EcallClient] Connected to gRPC server: %s\n", grpcAddr)
		}
	})
	return globalClient
}

// connect establishes the gRPC connection
func (c *EcallClient) connect() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(
		ctx,
		c.grpcAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", c.grpcAddr, err)
	}

	c.conn = conn
	return nil
}

// Close closes the gRPC connection
func (c *EcallClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// SetGrpcAddr updates the gRPC address and reconnects
func (c *EcallClient) SetGrpcAddr(addr string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		c.conn.Close()
	}

	c.grpcAddr = addr
	if err := c.connect(); err != nil {
		return err
	}

	fmt.Printf("[EcallClient] Reconnected to gRPC server: %s\n", addr)
	return nil
}

// FetchEcallRecord fetches a single ecall record from the remote SGX node
func (c *EcallClient) FetchEcallRecord(height int64) (*EcallRecordData, error) {
	if c.conn == nil {
		if err := c.connect(); err != nil {
			return nil, fmt.Errorf("not connected to gRPC server: %w", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	req := &QueryEcallRecordRequest{Height: height}
	resp := &QueryEcallRecordResponse{}

	if err := c.conn.Invoke(ctx, methodEcallRecord, req, resp); err != nil {
		return nil, fmt.Errorf("gRPC EcallRecord failed for height %d: %w", height, err)
	}

	if height%1000 == 0 {
		fmt.Printf("[EcallClient] Fetched ecall record for height %d\n", height)
	}

	return &EcallRecordData{
		Height:               resp.Height,
		RandomSeed:           resp.RandomSeed,
		ValidatorSetEvidence: resp.ValidatorSetEvidence,
	}, nil
}

// FetchEncryptedSeed fetches encrypted seed data from the remote SGX node
func (c *EcallClient) FetchEncryptedSeed(certHashHex string) ([]byte, error) {
	if c.conn == nil {
		if err := c.connect(); err != nil {
			return nil, fmt.Errorf("not connected to gRPC server: %w", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	req := &QueryEncryptedSeedRequest{CertHash: certHashHex}
	resp := &QueryEncryptedSeedResponse{}

	if err := c.conn.Invoke(ctx, methodEncryptedSeed, req, resp); err != nil {
		return nil, fmt.Errorf("gRPC EncryptedSeed failed: %w", err)
	}

	fmt.Printf("[EcallClient] Fetched encrypted seed (%d bytes)\n", len(resp.EncryptedSeed))
	return resp.EncryptedSeed, nil
}

// FetchBlockTraces fetches all execution traces for a block from the remote SGX node
func (c *EcallClient) FetchBlockTraces(height int64) ([]*ExecutionTrace, error) {
	if c.conn == nil {
		if err := c.connect(); err != nil {
			return nil, fmt.Errorf("not connected to gRPC server: %w", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	req := &QueryBlockTracesRequest{Height: height}
	resp := &QueryBlockTracesResponse{}

	if err := c.conn.Invoke(ctx, methodBlockTraces, req, resp); err != nil {
		return nil, fmt.Errorf("gRPC BlockTraces failed for height %d: %w", height, err)
	}

	// Convert proto response to ExecutionTrace slice
	traces := make([]*ExecutionTrace, len(resp.Traces))
	for i, t := range resp.Traces {
		fmt.Printf("[EcallClient] DEBUG: Proto trace callbackGas=%d (from gRPC response)\n", t.CallbackGas)
		ops := make([]StorageOp, len(t.Ops))
		for j, op := range t.Ops {
			ops[j] = StorageOp{
				IsDelete: op.IsDelete,
				Key:      op.Key,
				Value:    op.Value,
			}
		}
		traces[i] = &ExecutionTrace{
			Index:       t.Index,
			Ops:         ops,
			Result:      t.Result,
			GasUsed:     t.GasUsed,
			CallbackGas: t.CallbackGas,
			HasError:    t.HasError,
			ErrorMsg:    t.ErrorMsg,
		}
		fmt.Printf("[EcallClient] DEBUG: Converted trace callbackGas=%d\n", traces[i].CallbackGas)
	}

	if len(traces) > 0 {
		for _, t := range traces {
			fmt.Printf("[EcallClient] Fetched trace: height=%d index=%d ops=%d resultLen=%d gasUsed=%d callbackGas=%d hasError=%v\n",
				height, t.Index, len(t.Ops), len(t.Result), t.GasUsed, t.CallbackGas, t.HasError)
		}
	} else if height%1000 == 0 {
		fmt.Printf("[EcallClient] Fetched %d traces for block %d\n", len(traces), height)
	}
	return traces, nil
}

// IsConnected returns true if the client is connected to the gRPC server
func (c *EcallClient) IsConnected() bool {
	return c.conn != nil
}
