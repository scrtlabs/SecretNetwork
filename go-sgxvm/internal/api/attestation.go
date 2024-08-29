//go:build !nosgx && attestationServer
// +build !nosgx,attestationServer

package api

// #include <stdlib.h>
// #include "bindings.h"
import "C"

import (
	"fmt"
	"github.com/SigmaGmbH/librustgo/types"
	"google.golang.org/protobuf/proto"
	"net"
	"runtime"
)

// StartAttestationServer starts attestation server with 2 port (EPID and DCAP attestation)
func StartAttestationServer(epidAddress, dcapAddress string) error {
	fmt.Println("[Attestation Server] Trying to start attestation server")

	if epidAddress == dcapAddress {
		return fmt.Errorf("[Attestation Server] Provide different addresses for DCAP and EPID attestation")
	}

	epidListener, err := net.Listen("tcp", epidAddress)
	if err != nil {
		fmt.Println("[Attestation Server] Cannot start listener for EPID attestation")
		return err
	}

	dcapListener, err := net.Listen("tcp", dcapAddress)
	if err != nil {
		fmt.Println("[Attestation Server] Cannot start listener for DCAP attestation")
		return err
	}

	// Wait for incoming connections to EPID listener
	go func() {
		for {
			connection, err := epidListener.Accept()
			if err != nil {
				fmt.Println("[Attestation Server] EPID listener: Got error ", err.Error(), ", connection: ", connection.RemoteAddr().String())
				connection.Close()
				continue
			}

			// provide param attestation type
			if err := handleIncomingRARequest(connection, false); err != nil {
				fmt.Println("[Attestation Server] EPID listener: Attestation failed. Reason: ", err)
				connection.Close()
				continue
			}
		}
	}()

	// Wait for incoming connections to DCAP listener
	go func() {
		for {
			connection, err := dcapListener.Accept()
			if err != nil {
				fmt.Println("[Attestation Server] DCAP listener: Got error ", err.Error(), ", connection: ", connection.RemoteAddr().String())
				connection.Close()
				continue
			}

			if err := handleIncomingRARequest(connection, true); err != nil {
				fmt.Println("[Attestation Server] DCAP listener: Attestation failed. Reason: ", err)
				connection.Close()
				continue
			}
		}
	}()

	fmt.Printf("[Attestation Server] Started Attestation Server\nEPID attestation: %s\nDCAP attestation: %s", epidAddress, dcapAddress)

	return nil
}

// Handles incoming request for Remote Attestation
func handleIncomingRARequest(connection net.Conn, isDCAP bool) error {
	defer connection.Close()
	println("[Attestation Server] Attesting peer: ", connection.RemoteAddr().String())

	// Extract file descriptor for socket
	file, err := connection.(*net.TCPConn).File()
	if err != nil {
		fmt.Println("[Attestation Server] Cannot get access to the connection. Reason: ", err.Error())
		return err
	}

	// Create protobuf encoded request
	req := types.SetupRequest{Req: &types.SetupRequest_PeerAttestationRequest{
		PeerAttestationRequest: &types.PeerAttestationRequest{
			Fd:     int32(file.Fd()),
			IsDCAP: isDCAP,
		},
	}}
	reqBytes, err := proto.Marshal(&req)
	if err != nil {
		fmt.Println("[Attestation Server] Failed to encode req:", err)
		return err
	}

	_, err = SendProtobufRequest(reqBytes)
	return err
}

// SendProtobufRequest sends protobuf-encoded request to Rust side
func SendProtobufRequest(data []byte) (C.UnmanagedVector, error) {
	// Pass request to Rust
	d := MakeView(data)
	defer runtime.KeepAlive(data)

	errmsg := NewUnmanagedVector(nil)
	ptr, err := C.handle_initialization_request(d, &errmsg)
	if err != nil {
		return NewUnmanagedVector(nil), ErrorWithMessage(err, errmsg)
	}

	return ptr, nil
}
