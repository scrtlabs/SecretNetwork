//go:build !secretcli
// +build !secretcli

package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"math"

	"github.com/scrtlabs/SecretNetwork/go-cosmwasm/types"
)

// Stream opcodes for ocall operations
const (
	OpcodeSet        byte = 0x01
	OpcodeDelete     byte = 0x02
	OpcodeGet        byte = 0x03
	OpcodeQuery      byte = 0x04
	OpcodeTerminator byte = 0xFF
)

// EcallResult holds the result of an ecall execution
type EcallResult struct {
	Result     []byte
	GasUsed    uint64 // WASM gas from the enclave
	SDKGasUsed uint64 // Total SDK gas consumed during the ecall (store ops + consumeGas)
	HasError   bool
	ErrorMsg   string
}

// OcallStreamWriter records ocall operations and the ecall result into a binary stream.
// Used in SGX mode during ecall execution.
//
// OcallStreamWriter records ocall operations and the ecall result into a binary stream.
// Used in SGX mode during ecall execution.
//
// Stream format:
//
//	[OcallEntry]* [Terminator] [EcallResult]
//
// OcallEntry:
//
//	Opcode(1 byte) | KeyLen(4 bytes LE) | Key(KeyLen bytes) [| ValueLen(4 bytes LE) | Value(ValueLen bytes)]
//
// Terminator:
//
//	0xFF (1 byte)
//
// EcallResult:
//
//	ResultLen(4 bytes LE) | Result(ResultLen bytes) | GasUsed(8 bytes LE) | HasError(1 byte) [| ErrorLen(4 bytes LE) | Error(ErrorLen bytes)]
type OcallStreamWriter struct {
	buf bytes.Buffer
}

// NewOcallStreamWriter creates a new stream writer
func NewOcallStreamWriter() *OcallStreamWriter {
	return &OcallStreamWriter{}
}

// WriteSet records a Set ocall (key + value)
func (w *OcallStreamWriter) WriteSet(key, value []byte) {
	w.buf.WriteByte(OpcodeSet)
	w.writeBytes(key)
	w.writeBytes(value)
}

// WriteDelete records a Delete ocall (key only)
func (w *OcallStreamWriter) WriteDelete(key []byte) {
	w.buf.WriteByte(OpcodeDelete)
	w.writeBytes(key)
}

// WriteGet records a Get ocall (key only) to force the replay node's Memory Cache to sync
func (w *OcallStreamWriter) WriteGet(key []byte) {
	w.buf.WriteByte(OpcodeGet)
	w.writeBytes(key)
}

// WriteQuery records a Query external ocall (request only) to force the replay node's Memory Cache to sync
func (w *OcallStreamWriter) WriteQuery(request []byte) {
	w.buf.WriteByte(OpcodeQuery)
	w.writeBytes(request)
}

// Finalize writes the terminator and ecall result, returning the complete stream bytes
func (w *OcallStreamWriter) Finalize(result EcallResult) []byte {
	// Write terminator
	w.buf.WriteByte(OpcodeTerminator)

	// Write result data
	w.writeBytes(result.Result)

	// Write gas used (8 bytes LE)
	binary.Write(&w.buf, binary.LittleEndian, result.GasUsed)

	// Write SDK gas used (8 bytes LE)
	binary.Write(&w.buf, binary.LittleEndian, result.SDKGasUsed)

	// Write error flag
	if result.HasError {
		w.buf.WriteByte(1)
		w.writeBytes([]byte(result.ErrorMsg))
	} else {
		w.buf.WriteByte(0)
	}

	return w.buf.Bytes()
}

// writeBytes writes a length-prefixed byte slice
func (w *OcallStreamWriter) writeBytes(data []byte) {
	if data == nil {
		binary.Write(&w.buf, binary.LittleEndian, uint32(math.MaxUint32))
		return
	}
	length := uint32(len(data))
	binary.Write(&w.buf, binary.LittleEndian, length)
	w.buf.Write(data)
}

// OcallStreamReader reads ocall operations and the ecall result from a binary stream.
// Used in non-SGX mode to replay a recorded execution.
type OcallStreamReader struct {
	reader *bytes.Reader
}

// NewOcallStreamReader creates a new stream reader from serialized bytes
func NewOcallStreamReader(data []byte) *OcallStreamReader {
	return &OcallStreamReader{
		reader: bytes.NewReader(data),
	}
}

// OcallOp represents a single ocall operation read from the stream
type OcallOp struct {
	Opcode byte
	Key    []byte
	Value  []byte // nil for Delete
}

// ReadOcallOp reads the next ocall operation from the stream.
// Returns (op, false) for SET/DELETE operations.
// Returns (empty, true) when the terminator is reached.
func (r *OcallStreamReader) ReadOcallOp() (OcallOp, bool, error) {
	var opcode byte
	if err := binary.Read(r.reader, binary.LittleEndian, &opcode); err != nil {
		if err == io.EOF {
			return OcallOp{}, false, fmt.Errorf("unexpected end of stream")
		}
		return OcallOp{}, false, fmt.Errorf("reading opcode: %w", err)
	}

	if opcode == OpcodeTerminator {
		return OcallOp{}, true, nil
	}

	switch opcode {
	case OpcodeSet:
		key, err := r.readBytes()
		if err != nil {
			return OcallOp{}, false, fmt.Errorf("reading SET key: %w", err)
		}
		value, err := r.readBytes()
		if err != nil {
			return OcallOp{}, false, fmt.Errorf("reading SET value: %w", err)
		}
		return OcallOp{Opcode: OpcodeSet, Key: key, Value: value}, false, nil

	case OpcodeDelete:
		key, err := r.readBytes()
		if err != nil {
			return OcallOp{}, false, fmt.Errorf("reading DELETE key: %w", err)
		}
		return OcallOp{Opcode: OpcodeDelete, Key: key}, false, nil

	case OpcodeGet:
		key, err := r.readBytes()
		if err != nil {
			return OcallOp{}, false, fmt.Errorf("reading GET key: %w", err)
		}
		return OcallOp{Opcode: OpcodeGet, Key: key}, false, nil

	case OpcodeQuery:
		request, err := r.readBytes()
		if err != nil {
			return OcallOp{}, false, fmt.Errorf("reading QUERY request: %w", err)
		}
		return OcallOp{Opcode: OpcodeQuery, Key: request}, false, nil

	default:
		return OcallOp{}, false, fmt.Errorf("unknown opcode 0x%02x", opcode)
	}
}

// ReadEcallResult reads the ecall result after the terminator
func (r *OcallStreamReader) ReadEcallResult() (EcallResult, error) {
	result := EcallResult{}

	// Read result data
	resultBytes, err := r.readBytes()
	if err != nil {
		return result, fmt.Errorf("reading result: %w", err)
	}
	result.Result = resultBytes

	// Read gas used (8 bytes LE)
	if err := binary.Read(r.reader, binary.LittleEndian, &result.GasUsed); err != nil {
		return result, fmt.Errorf("reading gas used: %w", err)
	}

	// Read SDK gas used (8 bytes LE)
	if err := binary.Read(r.reader, binary.LittleEndian, &result.SDKGasUsed); err != nil {
		return result, fmt.Errorf("reading sdk gas used: %w", err)
	}

	// Read error flag
	var hasError byte
	if err := binary.Read(r.reader, binary.LittleEndian, &hasError); err != nil {
		return result, fmt.Errorf("reading error flag: %w", err)
	}
	result.HasError = hasError != 0

	if result.HasError {
		errBytes, err := r.readBytes()
		if err != nil {
			return result, fmt.Errorf("reading error message: %w", err)
		}
		result.ErrorMsg = string(errBytes)
	}

	return result, nil
}

// readBytes reads a length-prefixed byte slice
func (r *OcallStreamReader) readBytes() ([]byte, error) {
	var length uint32
	if err := binary.Read(r.reader, binary.LittleEndian, &length); err != nil {
		return nil, fmt.Errorf("reading length: %w", err)
	}

	if length == math.MaxUint32 {
		return nil, nil
	}

	result := make([]byte, length)
	if _, err := io.ReadFull(r.reader, result); err != nil {
		return nil, fmt.Errorf("reading bytes: %w", err)
	}

	return result, nil
}

// ReplayStream replays ocall operations from a stream onto a KVStore and returns the ecall result.
// This is the main entry point for non-SGX replay.
// Returns (result, wasmGasUsed, sdkGasUsed, error)
func ReplayStream(store KVStore, querier *Querier, streamBytes []byte) ([]byte, uint64, uint64, error) {
	reader := NewOcallStreamReader(streamBytes)

	// Replay all ocall operations
	for {
		op, isTerminator, err := reader.ReadOcallOp()
		if err != nil {
			return nil, 0, 0, fmt.Errorf("replay stream error: %w", err)
		}
		if isTerminator {
			break
		}

		switch op.Opcode {
		case OpcodeSet:
			keyHash := sha256.Sum256(op.Key)
			valHash := sha256.Sum256(op.Value)
			log.Printf("[STATE-MUTATION REPLAY] SET KeyHash:%s ValHash:%s KeyLen:%d ValLen:%d", hex.EncodeToString(keyHash[:]), hex.EncodeToString(valHash[:]), len(op.Key), len(op.Value))
			store.Set(op.Key, op.Value)
		case OpcodeDelete:
			keyHash := sha256.Sum256(op.Key)
			log.Printf("[STATE-MUTATION REPLAY] DEL KeyHash:%s KeyLen:%d", hex.EncodeToString(keyHash[:]), len(op.Key))
			store.Delete(op.Key)
		case OpcodeGet:
			// Simply reading the key forces the parent KVStore CacheContext to populate!
			// We discard the value since Replay only cares about the state footprint.
			store.Get(op.Key)
		case OpcodeQuery:
			if querier != nil {
				// We pass math.MaxUint64 because QueryHandler wraps the call in a sub-gas-meter.
				// Since Replay ctx uses an InfiniteGasMeter, this will safely drop the consumed gas without a panic.
				types.RustQuery(*querier, op.Key, 0, math.MaxUint64)
			}
		}
	}

	// Read the ecall result
	ecallResult, err := reader.ReadEcallResult()
	if err != nil {
		return nil, 0, 0, fmt.Errorf("replay stream: reading ecall result: %w", err)
	}

	if ecallResult.HasError {
		return nil, ecallResult.GasUsed, ecallResult.SDKGasUsed, fmt.Errorf("%s", ecallResult.ErrorMsg)
	}

	return ecallResult.Result, ecallResult.GasUsed, ecallResult.SDKGasUsed, nil
}

// ReplayStreamForBlockSignatures replays a stream that contains SubmitBlockSignatures output.
// The result contains random (32 bytes) || validator_set_evidence (remaining bytes).
func ReplayStreamForBlockSignatures(streamBytes []byte) (random []byte, evidence []byte, err error) {
	reader := NewOcallStreamReader(streamBytes)

	// SubmitBlockSignatures has no ocalls, skip straight to terminator
	_, isTerminator, err := reader.ReadOcallOp()
	if err != nil {
		return nil, nil, fmt.Errorf("reading block signatures stream: %w", err)
	}
	if !isTerminator {
		return nil, nil, fmt.Errorf("expected terminator at start of block signatures stream, got ocall")
	}

	ecallResult, err := reader.ReadEcallResult()
	if err != nil {
		return nil, nil, fmt.Errorf("reading block signatures result: %w", err)
	}

	if ecallResult.HasError {
		return nil, nil, fmt.Errorf("block signatures ecall error: %s", ecallResult.ErrorMsg)
	}

	// Result format: random_len(4 LE) | random | evidence
	if len(ecallResult.Result) < 4 {
		return nil, nil, fmt.Errorf("block signatures result too short: %d bytes", len(ecallResult.Result))
	}
	randomLen := binary.LittleEndian.Uint32(ecallResult.Result[:4])
	if int(randomLen)+4 > len(ecallResult.Result) {
		return nil, nil, fmt.Errorf("block signatures result too short for random: need %d, have %d", randomLen+4, len(ecallResult.Result))
	}
	random = ecallResult.Result[4 : 4+randomLen]
	evidence = ecallResult.Result[4+randomLen:]
	return random, evidence, nil
}

// PackBlockSignaturesResult packs random and evidence into the stream result format.
// Used on the SGX side when recording SubmitBlockSignatures.
func PackBlockSignaturesResult(random, evidence []byte) []byte {
	var buf []byte
	var lenBuf [4]byte
	binary.LittleEndian.PutUint32(lenBuf[:], uint32(len(random)))
	buf = append(buf, lenBuf[:]...)
	buf = append(buf, random...)
	buf = append(buf, evidence...)
	return buf
}
