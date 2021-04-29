package rumor

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime/debug"
	"sync"
	"syscall"
	"time"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/state"
	tmtypes "github.com/tendermint/tendermint/types"

	cosmosBaseApp "github.com/enigmampc/cosmos-sdk/baseapp"

	"github.com/enigmampc/SecretNetwork/rumor-go/app/mantlemint"
	"github.com/enigmampc/SecretNetwork/rumor-go/db"
	"github.com/enigmampc/SecretNetwork/rumor-go/depsresolver"
	"github.com/enigmampc/SecretNetwork/rumor-go/subscriber"
	"github.com/enigmampc/SecretNetwork/rumor-go/types"
)

// This interface is an extension of the abci.Application interface, plus some methods
// from cosmos-sdk/baseapp.BaseApp
type MantleApplication interface {
	abci.Application
	LastBlockHeight() int64
}

type Mantle struct {
	app                  MantleApplication
	mantlemint           mantlemint.Mantlemint
	depsResolverInstance depsresolver.DepsResolver
	db                   db.DB
	m                    *sync.Mutex
}

type SyncConfiguration struct {
	TendermintEndpoint string
	SyncUntil          uint64
	Reconnect          bool
	OnWSError          func(err error)
	OnInjectError      func(err error)
}

func NewMantle(
	app MantleApplication,
	db db.DB,
	genesis *tmtypes.GenesisDoc,
) Mantle {
	tmdb := db.GetCosmosAdapter()
	mint := mantlemint.NewMantlemint(
		tmdb,
		app,
	)
	rumorApp := Mantle{
		app:                  app,
		mantlemint:           mint,
		depsResolverInstance: depsresolver.NewDepsResolver(),
		db:                   db,
		m:                    new(sync.Mutex),
	}

	// create a signal handler
	sigChannel := make(chan os.Signal, 1)
	rumorApp.gracefulShutdownOnSignal(
		sigChannel,
		func() { db.Purge(false) },
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)
	rumorApp.gracefulShutdownOnSignal(
		sigChannel,
		func() { db.Purge(true) },
		syscall.SIGILL,
		syscall.SIGABRT,
		syscall.SIGKILL,
		syscall.SIGHUP,
	)

	// initialize within transaction boundary
	rumorApp.db.SetCriticalZone()

	// initialize chain
	if initErr := rumorApp.mantlemint.Init(genesis); initErr != nil {
		panic(initErr)
	}

	if releaseErr := rumorApp.db.ReleaseCriticalZone(); releaseErr != nil {
		panic(releaseErr)
	}

	return rumorApp
}

func (mantle *Mantle) GetApp() *cosmosBaseApp.BaseApp {
	return mantle.app.(*cosmosBaseApp.BaseApp)
}

func (mantle *Mantle) SetBlockExecutor(be mantlemint.MantlemintExecutorCreator) {
	mantle.mantlemint.SetBlockExecutor(be)
}

func (mantle *Mantle) querySync(configuration SyncConfiguration, currentBlockHeight int64) {
	log.Println("Local blockchain is behind, syncing previous blocks...")
	remoteBlock, err := subscriber.GetBlock(fmt.Sprintf("http://%s/block", configuration.TendermintEndpoint))

	if err != nil {
		panic(fmt.Errorf("error during mantle sync: remote head fetch failed. fromHeight=%d, (%s)", currentBlockHeight, err))
	}

	remoteHeight := remoteBlock.Header.Height
	syncingBlockHeight := currentBlockHeight
	tStart := time.Now()

	for syncingBlockHeight < remoteHeight {
		// stop sync if SyncUntil is given
		if configuration.SyncUntil != 0 && uint64(syncingBlockHeight) == configuration.SyncUntil {
			for {
			}
		}

		remoteBlock, err := subscriber.GetBlock(fmt.Sprintf("http://%s/block?height=%d", configuration.TendermintEndpoint, syncingBlockHeight+1))
		if err != nil {
			panic(fmt.Errorf("error during mantle sync: remote block(%d) fetch failed", syncingBlockHeight))
		}

		// run round
		if _, err := mantle.Inject(remoteBlock); err != nil {
			if configuration.OnInjectError != nil {
				configuration.OnInjectError(err)
			} else {
				panic(err)
			}
		}

		syncingBlockHeight++
	}

	dur := time.Now().Sub(tStart)

	if dur > time.Second {
		log.Printf("[mantle] QuerySync: %d to %d, Elapsed: %dms", currentBlockHeight, remoteHeight, dur.Milliseconds())
	}
}

func (mantle *Mantle) Sync(configuration SyncConfiguration) {
	// subscribe to NewBlock event
	rpcSubscription, connRefused := subscriber.NewRpcSubscription(
		fmt.Sprintf("ws://%s/websocket", configuration.TendermintEndpoint),
		configuration.OnWSError,
	)

	// connRefused here is most likely triggered by ECONNREFUSED
	// in case reconnect flag is set, try reestablish the connection after 5 seconds.
	if connRefused != nil {
		if configuration.Reconnect {
			select {
			case <-time.NewTimer(5 * time.Second).C:
				mantle.Sync(configuration)
			}
			return

		} else {
			panic(connRefused)
		}
	}

	blockChannel := rpcSubscription.Subscribe(configuration.Reconnect)

	for {
		select {
		case block := <-blockChannel:
			lastBlockHeight := mantle.app.LastBlockHeight()

			// stop sync if SyncUntil is given
			if configuration.SyncUntil != 0 && uint64(lastBlockHeight) == configuration.SyncUntil {
				for {
				}
			}

			if block.Header.Height-lastBlockHeight != 1 {
				log.Printf("lastBlockHeight=%v, remoteBlockHeight=%v\n", lastBlockHeight, block.Header.Height)
				mantle.querySync(configuration, lastBlockHeight)
			} else {
				if _, err := mantle.Inject(&block); err != nil {
					// if OnInjectError is set,
					// relay injection error to the caller
					if configuration.OnInjectError != nil {
						configuration.OnInjectError(err)
					} else {
						panic(err)
					}
				}
			}
		}
	}
}

func (mantle *Mantle) Inject(block *types.Block) (*types.BlockState, error) {
	// handle any inevitable panic
	defer func() {
		if r := recover(); r != nil {
			log.Printf("!! mantle panicked w/ message: %s", r)
			debug.PrintStack()
			log.Print("[mantle] panic during inject, attempting graceful shutdown")

			// if mantle reaches this point, there was a panic during injection.
			// in such case db access is all gone, it is safe to NOT get a lock.
			// but doing it, just in case :)
			mantle.m.Lock() // never unlock
			mantle.db.Purge(true)
			log.Print("[mantle] shutdown done")
			os.Exit(0)
		}
	}()

	mantle.m.Lock()
	defer mantle.m.Unlock()

	// set global global_transaction boundary for
	// tendermint, cosmos, mantle
	mantle.db.SetCriticalZone()

	// time
	tStart := time.Now()

	// inject this block
	blockState, injectErr := mantle.mantlemint.Inject(block)

	// flush to db after injection is done
	// never call this in defer, as in panic cases we need to be able to revert this commit
	mantle.db.ReleaseCriticalZone()

	// if injection was successful,
	// flush all to disk
	if injectErr != nil {
		return blockState, injectErr
	}

	mantle.db.ReleaseCriticalZone()

	// time end
	tEnd := time.Now()

	log.Printf(
		"[mantle] Indexing finished for block(%d), processed in %dms",
		block.Header.Height,
		tEnd.Sub(tStart).Milliseconds(),
	)

	return blockState, injectErr
}

func (mantle *Mantle) ExportStates() map[string]interface{} {
	return mantle.depsResolverInstance.GetState()
}

func (mantle *Mantle) GetLastState() state.State {
	return mantle.mantlemint.GetCurrentState()
}

func (mantle *Mantle) GetLastHeight() int64 {
	return mantle.mantlemint.GetCurrentHeight()
}

func (mantle *Mantle) GetLastBlock() *types.Block {
	return mantle.mantlemint.GetCurrentBlock()
}

func (mantle *Mantle) gracefulShutdownOnSignal(
	sig chan os.Signal,
	callback func(),
	signalTypes ...os.Signal,
) {
	// handle
	signal.Notify(
		sig,
		signalTypes...,
	)

	go func() {
		received := <-sig
		log.Printf("[mantle] received %v, cleanup...", received.String())
		// wait until inject is cleared
		log.Printf("[mantle] attempting graceful shutdown...")
		mantle.m.Lock() // never unlock
		callback()
		log.Printf("[mantle] shutdown done")
		os.Exit(0)
	}()
}
