package keeper

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/log"
	snapshottypes "cosmossdk.io/store/snapshots/types"
	storetypes "cosmossdk.io/store/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	protoio "github.com/cosmos/gogoproto/io"
	"github.com/scrtlabs/SecretNetwork/x/compute/internal/types"
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

var _ snapshottypes.ExtensionSnapshotter = (*WasmSnapshotter)(nil)

type WasmSnapshotter struct {
	cms           storetypes.MultiStore
	keeper        *Keeper
	wasmDirectory string
}

func NewWasmSnapshotter(cms storetypes.MultiStore, keeper *Keeper, wasmDirectory string) *WasmSnapshotter {
	return &WasmSnapshotter{
		cms:           cms,
		keeper:        keeper,
		wasmDirectory: wasmDirectory,
	}
}

func (ws *WasmSnapshotter) SnapshotName() string {
	return types.ModuleName
}

func (ws *WasmSnapshotter) SnapshotFormat() uint32 {
	// Format 1 is just the wasm byte code for each item payload. No protobuf envelope, no metadata.
	return 1
}

func (ws *WasmSnapshotter) SupportedFormats() []uint32 {
	// If we support older formats, add them here and handle them in Restore
	return []uint32{1}
}

func (ws *WasmSnapshotter) Snapshot(height uint64, protoWriter protoio.Writer) error {
	// TODO: This seems more correct (historical info), but kills my tests
	// Since codeIDs and wasm are immutible, it is never wrong to return new wasm data than the
	// user requests
	// ------
	cacheMS, err := ws.cms.CacheMultiStoreWithVersion(int64(height))
	if err != nil {
		return err
	}
	// cacheMS := ws.cms.CacheMultiStore()

	ctx := sdk.NewContext(cacheMS, tmproto.Header{}, false, log.NewNopLogger())

	seen := make(map[string]bool)

	var rerr error
	ws.keeper.IterateCodeInfos(ctx, func(id uint64, info types.CodeInfo) bool {
		// Many code ids may point to the same code hash... only sync it once
		hexHash := hex.EncodeToString(info.CodeHash)
		// if seen, just skip this one and move to the next
		if seen[hexHash] {
			return false
		}
		seen[hexHash] = true

		// load code and abort on error
		wasmBytes, err := ws.keeper.GetWasm(ctx, id)
		if err != nil {
			rerr = err
			return true
		}

		err = snapshottypes.WriteExtensionPayload(protoWriter, wasmBytes)
		if err != nil {
			rerr = err
			return true
		}

		return false
	})

	return rerr
}

func (ws *WasmSnapshotter) Restore(
	height uint64, format uint32, protoReader protoio.Reader, //nolint:all
) (snapshottypes.SnapshotItem, error) {
	if format != 1 {
		return snapshottypes.SnapshotItem{}, snapshottypes.ErrUnknownFormat
	}

	for {
		item := snapshottypes.SnapshotItem{}
		err := protoReader.ReadMsg(&item)
		if err == io.EOF {
			return snapshottypes.SnapshotItem{}, nil
		} else if err != nil {
			return snapshottypes.SnapshotItem{}, errorsmod.Wrap(err, "invalid protobuf message")
		}

		// if it is not another ExtensionPayload message, then it is not for us.
		// we should return it an let the manager handle this one
		payload := item.GetExtensionPayload()
		if payload == nil {
			return item, nil
		}

		wasmBytes := payload.Payload

		codeHash := sha256.Sum256(wasmBytes)
		wasmFileName := hex.EncodeToString(codeHash[:])
		wasmFilePath := filepath.Join(ws.wasmDirectory, wasmFileName)

		err = os.WriteFile(wasmFilePath, wasmBytes, 0o600 /* -rw------- */)
		if err != nil {
			return snapshottypes.SnapshotItem{}, errorsmod.Wrapf(err, "failed to write wasm file '%v' to disk", wasmFilePath)
		}
	}
}

func (ws *WasmSnapshotter) PruneSnapshotHeight(_ int64) {
	panic("not implemented")
}

func (ws *WasmSnapshotter) SetSnapshotInterval(_ uint64) {
	panic("not implemented")
}

func (ws *WasmSnapshotter) RestoreExtension(_ uint64, _ uint32, _ snapshottypes.ExtensionPayloadReader) error {
	panic("not implemented")
}

func (ws *WasmSnapshotter) SnapshotExtension(_ uint64, _ snapshottypes.ExtensionPayloadWriter) error {
	panic("not implemented")
}
