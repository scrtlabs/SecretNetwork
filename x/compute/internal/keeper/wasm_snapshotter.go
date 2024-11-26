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
	"github.com/scrtlabs/SecretNetwork/x/compute/internal/types"
)

/*
API to implement:

// ExtensionSnapshotter is an extension Snapshotter that is appended to the snapshot stream.
// ExtensionSnapshotter has an unique name and manages it's own internal formats.
type ExtensionSnapshotter interface {
	// SnapshotName returns the name of snapshotter, it should be unique in the manager.
	SnapshotName() string

	// SnapshotFormat returns the default format the extension snapshotter use to encode the
	// payloads when taking a snapshot.
	// It's defined within the extension, different from the global format for the whole state-sync snapshot.
	SnapshotFormat() uint32

	// SupportedFormats returns a list of formats it can restore from.
	SupportedFormats() []uint32

	// SnapshotExtension writes extension payloads into the underlying protobuf stream.
	SnapshotExtension(height uint64, payloadWriter ExtensionPayloadWriter) error

	// RestoreExtension restores an extension state snapshot,
	// the payload reader returns `io.EOF` when reached the extension boundaries.
	RestoreExtension(height uint64, format uint32, payloadReader ExtensionPayloadReader) error
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

func (ws *WasmSnapshotter) SnapshotExtension(height uint64, payloadWriter snapshottypes.ExtensionPayloadWriter) error {
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

		err = payloadWriter(wasmBytes)
		if err != nil {
			rerr = err
			return true
		}

		return false
	})

	return rerr
}

func (ws *WasmSnapshotter) RestoreExtension(
	height uint64, format uint32, payloadReader snapshottypes.ExtensionPayloadReader, //nolint:all
) error {
	if format != 1 {
		return snapshottypes.ErrUnknownFormat
	}

	for {
		wasmBytes, err := payloadReader()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return errorsmod.Wrap(err, "invalid protobuf message")
		}

		codeHash := sha256.Sum256(wasmBytes)
		wasmFileName := hex.EncodeToString(codeHash[:])
		wasmFilePath := filepath.Join(ws.wasmDirectory, wasmFileName)

		err = os.WriteFile(wasmFilePath, wasmBytes, 0o600 /* -rw------- */)
		if err != nil {
			return errorsmod.Wrapf(err, "failed to write wasm file '%v' to disk", wasmFilePath)
		}
	}
}
