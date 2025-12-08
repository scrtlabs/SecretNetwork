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

func (m *QueryEncryptedSeedResponse) Reset()         { *m = QueryEncryptedSeedResponse{} }
func (m *QueryEncryptedSeedResponse) String() string { return fmt.Sprintf("{len:%d}", len(m.EncryptedSeed)) }
func (m *QueryEncryptedSeedResponse) ProtoMessage()  {}

const (
	methodEcallRecord    = "/secret.compute.v1beta1.Query/EcallRecord"
	methodEncryptedSeed  = "/secret.compute.v1beta1.Query/EncryptedSeed"
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

// IsConnected returns true if the client is connected to the gRPC server
func (c *EcallClient) IsConnected() bool {
	return c.conn != nil
}
