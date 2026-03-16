//go:build !secretcli

package api

import (
	"fmt"
	"time"
)

// replayExecution handles replay of a recorded execution trace.
// We trust the SGX node's trace data completely.
//
// Gas is consumed in two steps to exactly match the SGX node:
//  1. callbackGas: charged on the original SDK meter directly (callbackGas/1000 SDK gas)
//     This matches the SGX node where DB callbacks charge gas on ctx.GasMeter() during C.handle.
//  2. gasUsed (compute gas): returned to the caller so the keeper's consumeGas handles it,
//     adding (gasUsed/1000 + 1) SDK gas — same as on the SGX node.
//
// These must be separate divisions to avoid integer rounding differences.
func replayExecution(store KVStore, gasMeter *GasMeter, execIndex int64) ([]byte, uint64, error, bool) {
	recorder := GetRecorder()
	height := recorder.GetCurrentBlockHeight()

	trace, found := recorder.GetTraceFromMemory(execIndex)
	if !found {
		logDebug("replayExecution", "TRACE NOT FOUND in memory: height=%d index=%d, waiting for SGX node", height, execIndex)

		client := GetEcallClient()
		retryDelay := 2 * time.Second
		attempt := 0

		for {
			allTraces, err := client.FetchBlockTraces(height)
			if err == nil {
				recorder.SetBlockTraces(allTraces)
				trace, found = recorder.GetTraceFromMemory(execIndex)
				if found {
					logInfo("replayExecution", "Fetched trace: height=%d index=%d (attempt %d)", height, execIndex, attempt+1)
					break
				}
			}

			attempt++
			if attempt%15 == 1 { // Log every ~30 seconds (15 * 2s)
				logWarn("replayExecution", "Waiting for SGX node trace: height=%d index=%d attempt=%d err=%v", height, execIndex, attempt, err)
			}
			time.Sleep(retryDelay)
		}

		if !found {
			logWarn("replayExecution", "TRACE NOT FOUND after retries: height=%d index=%d", height, execIndex)
			return nil, 0, nil, false
		}
	}

	logDebug("replayExecution", "Found trace: height=%d index=%d ops=%d resultLen=%d gasUsed=%d callbackGas=%d hasError=%v",
		height, execIndex, len(trace.Ops), len(trace.Result), trace.GasUsed, trace.CallbackGas, trace.HasError)

	// Apply recorded storage ops (trusted data from SGX node).
	// The store uses an InfiniteGasMeter (set by keeper), so these
	// ops do NOT charge the real gas meter.
	replayer := NewReplayingKVStore(store)
	replayer.ApplyOps(trace.Ops)

	// Stash cross-module ops for the keeper to apply on the real ctx.MultiStore().
	// These are mutations that query handlers made to other modules' stores
	// (e.g., distribution rewards withdrawal during a staking query).
	if len(trace.CrossOps) > 0 {
		logDebug("replayExecution", "Stashing %d cross-module ops for keeper", len(trace.CrossOps))
		recorder.SetPendingCrossModuleOps(trace.CrossOps)
	}

	// Consume exactly the CallbackGas recorded by the SGX node.
	// Since the store is gas-free, we don't need to reconcile with opsGas.
	// This makes the replay node's gas meter match the SGX node exactly.
	//
	// We wrap this in a deferred recovery because the consumption may panic
	// with ErrorOutOfGas. On the SGX node, the equivalent panic is caught by
	// recoverPanic (callbacks.go) inside the CGo boundary and converted to
	// GoResult_OutOfGas, so Handle returns normally with types.OutOfGasError.
	// Without this recovery, the panic would propagate directly to runTx,
	// producing a different ResponseDeliverTx and LastResultsHash mismatch.
	if gasMeter != nil && trace.CallbackGas > 0 {
		logDebug("replayExecution", "Consuming exact CallbackGas=%d on real gas meter", trace.CallbackGas)
		func() {
			defer func() {
				if r := recover(); r != nil {
					logDebug("replayExecution",
						"Caught out-of-gas panic during CallbackGas consumption (expected for failed traces)")
				}
			}()
			(*gasMeter).ConsumeGas(trace.CallbackGas, "replay callback gas (matching SGX trace)")
		}()
	}

	// Return only compute gasUsed — the keeper's consumeGas will add (gasUsed/1000)+1,
	// exactly matching the SGX node's second gas consumption step.
	if trace.HasError {
		logDebug("replayExecution", "Returning error (gasUsed=%d): %s", trace.GasUsed, trace.ErrorMsg)
		return nil, trace.GasUsed, fmt.Errorf("%s", trace.ErrorMsg), true
	}

	return trace.Result, trace.GasUsed, nil, true
}
