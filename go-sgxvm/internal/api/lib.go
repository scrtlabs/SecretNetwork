//go:build !nosgx
// +build !nosgx

package api

// #include <stdlib.h>
// #include "bindings.h"
import "C"

import (
	"fmt"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"google.golang.org/protobuf/proto"
	"log"
	"net"
	"runtime"

	"github.com/SigmaGmbH/librustgo/types"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

// Value types
type (
	cint   = C.int
	cbool  = C.bool
	cusize = C.size_t
	cu8    = C.uint8_t
	cu32   = C.uint32_t
	cu64   = C.uint64_t
	ci8    = C.int8_t
	ci32   = C.int32_t
	ci64   = C.int64_t
)

// Pointers
type cu8_ptr = *C.uint8_t

// Connector is our custom connector
type Connector = types.Connector

func CheckNodeStatus() error {
	req := types.SetupRequest{Req: &types.SetupRequest_NodeStatus{}}
	reqBytes, err := proto.Marshal(&req)
	if err != nil {
		log.Fatalln("Failed to encode req:", err)
		return err
	}

	// Pass request to Rust
	d := MakeView(reqBytes)
	defer runtime.KeepAlive(reqBytes)

	errmsg := NewUnmanagedVector(nil)
	ptr, err := C.handle_initialization_request(d, &errmsg)
	if err != nil {
		return ErrorWithMessage(err, errmsg)
	}

	// Recover returned value
	executionResult := CopyAndDestroyUnmanagedVector(ptr)
	response := types.IsInitializedResponse{}
	if err := proto.Unmarshal(executionResult, &response); err != nil {
		log.Fatalln("Failed to decode execution result:", err)
		return err
	}

	return nil
}

// IsNodeInitialized checks if node was initialized and key manager state was sealed
func IsNodeInitialized() (bool, error) {
	// Create protobuf encoded request
	req := types.SetupRequest{Req: &types.SetupRequest_IsInitialized{
		IsInitialized: &types.IsInitializedRequest{},
	}}

	reqBytes, err := proto.Marshal(&req)
	if err != nil {
		log.Fatalln("Failed to encode req:", err)
		return false, err
	}

	// Pass request to Rust
	d := MakeView(reqBytes)
	defer runtime.KeepAlive(reqBytes)

	errmsg := NewUnmanagedVector(nil)

	ptr, err := C.handle_initialization_request(d, &errmsg)
	if err != nil {
		return false, ErrorWithMessage(err, errmsg)
	}

	// Recover returned value
	executionResult := CopyAndDestroyUnmanagedVector(ptr)
	response := types.IsInitializedResponse{}
	if err := proto.Unmarshal(executionResult, &response); err != nil {
		log.Fatalln("Failed to decode execution result:", err)
		return false, err
	}

	return response.IsInitialized, nil
}

// SetupSeedNode handles initialization of attestation server which will share epoch keys with other nodes
func InitializeEnclave(shouldReset bool) error {
	// Create protobuf encoded request
	req := types.SetupRequest{Req: &types.SetupRequest_InitializeEnclave{
		InitializeEnclave: &types.InitializeEnclaveRequest{ShouldReset: shouldReset},
	}}
	reqBytes, err := proto.Marshal(&req)
	if err != nil {
		log.Fatalln("Failed to encode req:", err)
		return err
	}

	// Pass request to Rust
	d := MakeView(reqBytes)
	defer runtime.KeepAlive(reqBytes)

	errmsg := NewUnmanagedVector(nil)
	_, err = C.handle_initialization_request(d, &errmsg)
	if err != nil {
		return ErrorWithMessage(err, errmsg)
	}

	return nil
}

// RequestEpochKeys handles request of epoch keys from attestation server
func RequestEpochKeys(hostname string, port int, isDCAP bool) error {
	address := fmt.Sprintf("%s:%d", hostname, port)
	conn, err := net.Dial("tcp", address)
	if err != nil {
		fmt.Println("Cannot establish connection with attestation server. Reason: ", err.Error())
		return err
	}

	file, err := conn.(*net.TCPConn).File()
	if err != nil {
		fmt.Println("Cannot get access to the connection. Reason: ", err.Error())
		conn.Close()
		return err
	}

	// Create protobuf encoded request
	req := types.SetupRequest{Req: &types.SetupRequest_RemoteAttestationRequest{
		RemoteAttestationRequest: &types.RemoteAttestationRequest{
			Fd:       int32(file.Fd()),
			Hostname: hostname,
			IsDCAP:   isDCAP,
		},
	}}
	reqBytes, err := proto.Marshal(&req)
	if err != nil {
		log.Fatalln("Failed to encode req:", err)
		conn.Close()
		return err
	}

	// Pass request to Rust
	d := MakeView(reqBytes)
	defer runtime.KeepAlive(reqBytes)

	errmsg := NewUnmanagedVector(nil)
	_, err = C.handle_initialization_request(d, &errmsg)
	if err != nil {
		conn.Close()
		return ErrorWithMessage(err, errmsg)
	}

	return nil
}

// GetNodePublicKey handles request for node public key
func GetNodePublicKey(blockNumber uint64) (*types.NodePublicKeyResponse, error) {
	// Construct mocked querier
	c := buildEmptyConnector()

	// Create protobuf-encoded request
	req := &types.FFIRequest{Req: &types.FFIRequest_PublicKeyRequest{
		PublicKeyRequest: &types.NodePublicKeyRequest{
			BlockNumber: blockNumber,
		},
	}}
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		log.Fatalln("Failed to encode req:", err)
		return nil, err
	}

	// Pass request to Rust
	d := MakeView(reqBytes)
	defer runtime.KeepAlive(reqBytes)

	errmsg := NewUnmanagedVector(nil)
	ptr, err := C.make_pb_request(c, d, &errmsg)
	if err != nil {
		return &types.NodePublicKeyResponse{}, ErrorWithMessage(err, errmsg)
	}

	// Recover returned value
	executionResult := CopyAndDestroyUnmanagedVector(ptr)
	response := types.NodePublicKeyResponse{}
	if err := proto.Unmarshal(executionResult, &response); err != nil {
		log.Fatalln("Failed to decode node public key result:", err)
		return nil, err
	}

	return &response, nil
}

// DumpDCAPQuote generates DCAP quote for the enclave and writes it to the disk
func DumpDCAPQuote(filepath string) error {
	// Create protobuf encoded request
	req := types.SetupRequest{Req: &types.SetupRequest_DumpQuote{
		DumpQuote: &types.DumpQuoteRequest{Filepath: filepath},
	}}
	reqBytes, err := proto.Marshal(&req)
	if err != nil {
		log.Fatalln("Failed to encode req:", err)
		return err
	}

	// Pass request to Rust
	d := MakeView(reqBytes)
	defer runtime.KeepAlive(reqBytes)

	errmsg := NewUnmanagedVector(nil)
	_, err = C.handle_initialization_request(d, &errmsg)
	if err != nil {
		return ErrorWithMessage(err, errmsg)
	}

	return nil
}

// VerifyDCAPQuote verifies DCAP quote written to disk
func VerifyDCAPQuote(filepath string) error {
	// Create protobuf encoded request
	req := types.SetupRequest{Req: &types.SetupRequest_VerifyQuote{
		VerifyQuote: &types.VerifyQuoteRequest{Filepath: filepath},
	}}
	reqBytes, err := proto.Marshal(&req)
	if err != nil {
		log.Fatalln("Failed to encode req:", err)
		return err
	}

	// Pass request to Rust
	d := MakeView(reqBytes)
	defer runtime.KeepAlive(reqBytes)

	errmsg := NewUnmanagedVector(nil)
	_, err = C.handle_initialization_request(d, &errmsg)
	if err != nil {
		return ErrorWithMessage(err, errmsg)
	}

	return nil
}

// Call handles incoming call to contract or transfer of value
func Call(
	connector Connector,
	from, to, data, value []byte,
	accessList ethtypes.AccessList,
	gasLimit, nonce uint64,
	txContext *types.TransactionContext,
	commit bool,
	isUnencrypted bool,
) (*types.HandleTransactionResponse, error) {
	// Construct mocked querier
	c := BuildConnector(connector)

	// Create protobuf-encoded transaction data
	params := &types.SGXVMCallParams{
		From:        from,
		To:          to,
		Data:        data,
		GasLimit:    gasLimit,
		Value:       value,
		AccessList:  convertAccessList(accessList),
		Commit:      commit,
		Nonce:       nonce,
		Unencrypted: isUnencrypted,
	}

	// Create protobuf encoded request
	req := types.FFIRequest{Req: &types.FFIRequest_CallRequest{
		CallRequest: &types.SGXVMCallRequest{
			Params:  params,
			Context: txContext,
		},
	}}
	reqBytes, err := proto.Marshal(&req)
	if err != nil {
		log.Fatalln("Failed to encode req:", err)
		return nil, err
	}

	// Pass request to Rust
	d := MakeView(reqBytes)
	defer runtime.KeepAlive(reqBytes)

	errmsg := NewUnmanagedVector(nil)
	ptr, err := C.make_pb_request(c, d, &errmsg)
	if err != nil {
		return &types.HandleTransactionResponse{}, ErrorWithMessage(err, errmsg)
	}

	// Recover returned value
	executionResult := CopyAndDestroyUnmanagedVector(ptr)
	response := types.HandleTransactionResponse{}
	if err := proto.Unmarshal(executionResult, &response); err != nil {
		log.Fatalln("Failed to decode execution result:", err)
		return nil, err
	}

	return &response, nil
}

// Create handles incoming request for creation of new contract
func Create(
	connector Connector,
	from, data, value []byte,
	accessList ethtypes.AccessList,
	gasLimit, nonce uint64,
	txContext *types.TransactionContext,
	commit bool,
) (*types.HandleTransactionResponse, error) {
	// Construct mocked querier
	c := BuildConnector(connector)

	// Create protobuf-encoded transaction data
	params := &types.SGXVMCreateParams{
		From:       from,
		Data:       data,
		GasLimit:   gasLimit,
		Value:      value,
		AccessList: convertAccessList(accessList),
		Commit:     commit,
		Nonce:      nonce,
	}

	// Create protobuf encoded request
	req := types.FFIRequest{Req: &types.FFIRequest_CreateRequest{
		CreateRequest: &types.SGXVMCreateRequest{
			Params:  params,
			Context: txContext,
		},
	}}
	reqBytes, err := proto.Marshal(&req)
	if err != nil {
		log.Fatalln("Failed to encode req:", err)
		return nil, err
	}

	// Pass request to Rust
	d := MakeView(reqBytes)
	defer runtime.KeepAlive(reqBytes)

	errmsg := NewUnmanagedVector(nil)
	ptr, err := C.make_pb_request(c, d, &errmsg)
	if err != nil {
		return &types.HandleTransactionResponse{}, ErrorWithMessage(err, errmsg)
	}

	// Recover returned value
	executionResult := CopyAndDestroyUnmanagedVector(ptr)
	response := types.HandleTransactionResponse{}
	if err := proto.Unmarshal(executionResult, &response); err != nil {
		log.Fatalln("Failed to decode execution result:", err)
		return nil, err
	}

	return &response, nil
}

func AddEpoch(startingBlock uint64) error {
	// Create protobuf encoded request
	req := types.SetupRequest{Req: &types.SetupRequest_AddEpoch{
		AddEpoch: &types.AddNewEpochRequest{StartingBlock: startingBlock},
	}}
	reqBytes, err := proto.Marshal(&req)
	if err != nil {
		log.Fatalln("Failed to encode req:", err)
		return err
	}

	// Pass request to Rust
	d := MakeView(reqBytes)
	defer runtime.KeepAlive(reqBytes)

	errmsg := NewUnmanagedVector(nil)
	_, err = C.handle_initialization_request(d, &errmsg)
	if err != nil {
		return ErrorWithMessage(err, errmsg)
	}

	return nil
}

func RemoveLatestEpoch() error {
	// Create protobuf encoded request
	req := types.SetupRequest{Req: &types.SetupRequest_RemoveEpoch{
		RemoveEpoch: &types.RemoveLatestEpochRequest{},
	}}
	reqBytes, err := proto.Marshal(&req)
	if err != nil {
		log.Fatalln("Failed to encode req:", err)
		return err
	}

	// Pass request to Rust
	d := MakeView(reqBytes)
	defer runtime.KeepAlive(reqBytes)

	errmsg := NewUnmanagedVector(nil)
	_, err = C.handle_initialization_request(d, &errmsg)
	if err != nil {
		return ErrorWithMessage(err, errmsg)
	}

	return nil
}

func ListEpochs() ([]*types.EpochData, error) {
	// Create protobuf encoded request
	req := types.SetupRequest{Req: &types.SetupRequest_ListEpochs{
		ListEpochs: &types.ListEpochsRequest{},
	}}
	reqBytes, err := proto.Marshal(&req)
	if err != nil {
		log.Fatalln("Failed to encode req:", err)
		return nil, err
	}

	// Pass request to Rust
	d := MakeView(reqBytes)
	defer runtime.KeepAlive(reqBytes)

	errmsg := NewUnmanagedVector(nil)
	ptr, err := C.handle_initialization_request(d, &errmsg)
	if err != nil {
		return nil, ErrorWithMessage(err, errmsg)
	}

	// Recover returned value
	executionResult := CopyAndDestroyUnmanagedVector(ptr)
	response := types.ListEpochsResponse{}
	if err := proto.Unmarshal(executionResult, &response); err != nil {
		log.Fatalln("Failed to decode execution result:", err)
		return nil, err
	}

	return response.Epochs, nil
}

// Converts AccessList type from ethtypes to protobuf-compatible type
func convertAccessList(accessList ethtypes.AccessList) []*types.AccessListItem {
	var converted []*types.AccessListItem
	for _, item := range accessList {
		accessListItem := &types.AccessListItem{
			StorageSlot: convertAccessListStorageSlots(item.StorageKeys),
			Address:     item.Address.Bytes(),
		}

		converted = append(converted, accessListItem)
	}
	return converted
}

// Converts storage slots of access list in [][]byte format
func convertAccessListStorageSlots(slots []ethcommon.Hash) [][]byte {
	var converted [][]byte
	for _, slot := range slots {
		converted = append(converted, slot.Bytes())
	}
	return converted
}

/**** To error module ***/

func ErrorWithMessage(err error, b C.UnmanagedVector) error {
	msg := CopyAndDestroyUnmanagedVector(b)
	if msg == nil {
		return err
	}
	return fmt.Errorf("%s", string(msg))
}
