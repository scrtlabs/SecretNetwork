package compute

import (
	"io"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/cosmos/cosmos-sdk/snapshots/types"
	snapshot "github.com/cosmos/cosmos-sdk/snapshots/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	protoio "github.com/gogo/protobuf/io"
)

/*
API to implement:

// Snapshotter is something that can create and restore snapshots, consisting of streamed binary
// chunks - all of which must be read from the channel and closed. If an unsupported format is
// given, it must return ErrUnknownFormat (possibly wrapped with fmt.Errorf).
type Snapshotter interface {
	// Snapshot writes snapshot items into the protobuf writer.
	Snapshot(height uint64, protoWriter protoio.Writer) error

	// Restore restores a state snapshot from the protobuf items read from the reader.
	// If the ready channel is non-nil, it returns a ready signal (by being closed) once the
	// restorer is ready to accept chunks.
	Restore(height uint64, format uint32, protoReader protoio.Reader) (SnapshotItem, error)
}

// ExtensionSnapshotter is an extension Snapshotter that is appended to the snapshot stream.
// ExtensionSnapshotter has an unique name and manages it's own internal formats.
type ExtensionSnapshotter interface {
	Snapshotter

	// SnapshotName returns the name of snapshotter, it should be unique in the manager.
	SnapshotName() string

	// SnapshotFormat returns the default format the extension snapshotter use to encode the
	// payloads when taking a snapshot.
	// It's defined within the extension, different from the global format for the whole state-sync snapshot.
	SnapshotFormat() uint32

	// SupportedFormats returns a list of formats it can restore from.
	SupportedFormats() []uint32
}
*/

type WasmSnapshotter struct {
	wasmDirectory string
}

func NewWasmSnapshotter(wasmDirectory string) *WasmSnapshotter {
	return &WasmSnapshotter{
		wasmDirectory,
	}
}

func (ws *WasmSnapshotter) SnapshotName() string {
	return "WASM Files Snapshot"
}

func (ws *WasmSnapshotter) SnapshotFormat() uint32 {
	return 1
}

func (ws *WasmSnapshotter) SupportedFormats() []uint32 {
	return []uint32{1}
}

var wasmFileNameRegex = regexp.MustCompile(`^[a-f0-9]{64}$`)

func (ws *WasmSnapshotter) Snapshot(height uint64, protoWriter protoio.Writer) error {
	wasmFiles, err := ioutil.ReadDir(ws.wasmDirectory)
	if err != nil {
		return err
	}

	// In case snapshotting needs to be deterministic
	sort.SliceStable(wasmFiles, func(i, j int) bool {
		return strings.Compare(wasmFiles[i].Name(), wasmFiles[j].Name()) < 0
	})

	for _, wasmFile := range wasmFiles {
		if !wasmFileNameRegex.MatchString(wasmFile.Name()) {
			continue
		}

		wasmFilePath := path.Join(ws.wasmDirectory, wasmFile.Name())
		wasmBytes, err := ioutil.ReadFile(wasmFilePath)
		if err != nil {
			return err
		}

		// snapshotItem is 64 bytes of the file name, then the actual WASM bytes
		snapshotItem := append([]byte(wasmFile.Name()), wasmBytes...)

		snapshot.WriteExtensionItem(protoWriter, snapshotItem)
	}

	return nil
}

func (ws *WasmSnapshotter) Restore(
	height uint64, format uint32, protoReader protoio.Reader,
) (snapshot.SnapshotItem, error) {
	if format != 1 {
		return snapshot.SnapshotItem{}, types.ErrUnknownFormat
	}

	// Create .compute directory if it doesn't exist already
	err := os.MkdirAll(ws.wasmDirectory, os.ModePerm)
	if err != nil {
		return snapshot.SnapshotItem{}, sdkerrors.Wrapf(err, "failed to create directory '%s'", ws.wasmDirectory)
	}

	for {
		item := &snapshot.SnapshotItem{}
		err = protoReader.ReadMsg(item)
		if err == io.EOF {
			break
		} else if err != nil {
			return snapshot.SnapshotItem{}, sdkerrors.Wrap(err, "invalid protobuf message")
		}

		payload := item.GetExtensionPayload()
		if payload == nil {
			return snapshot.SnapshotItem{}, sdkerrors.Wrap(err, "invalid protobuf message")
		}

		// snapshotItem is 64 bytes of the file name, then the actual WASM bytes
		if len(payload.Payload) < 64 {
			return snapshot.SnapshotItem{}, sdkerrors.Wrapf(err, "wasm snapshot must be at least 64 bytes, got %v bytes", len(payload.Payload))
		}

		wasmFileName := string(payload.Payload[0:64])
		wasmBytes := payload.Payload[64:]

		wasmFilePath := path.Join(ws.wasmDirectory, wasmFileName)

		err = ioutil.WriteFile(wasmFilePath, wasmBytes, 0664 /* -rw-rw-r-- */)
		if err != nil {
			return snapshot.SnapshotItem{}, sdkerrors.Wrapf(err, "failed to write wasm file '%v' to disk", wasmFilePath)
		}
	}

	return snapshot.SnapshotItem{}, nil
}
