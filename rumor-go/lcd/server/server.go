package server

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/gorilla/mux"

	abci "github.com/tendermint/tendermint/abci/types"

	client "github.com/enigmampc/cosmos-sdk/client/context"
	"github.com/enigmampc/cosmos-sdk/codec"
	cosmosmodule "github.com/enigmampc/cosmos-sdk/types/module"
)

type MantleLCDProxy struct{}

func NewMantleLCDServer() *MantleLCDProxy {
	return &MantleLCDProxy{}
}

func (lcd *MantleLCDProxy) Server(port int, app abci.Application, cdc *codec.Codec, mod *cosmosmodule.BasicManager) {
	router := mux.NewRouter().SkipClean(true)
	m := &sync.Mutex{}

	//localClient := compatlocalclient.NewLocalClient(app, m)
	localClient := NewLocalClient(app, m)
	ctx := client.
		NewCLIContext().
		WithTrustNode(true).
		//WithCodec(terra.MakeCodec()).
		WithCodec(cdc).
		WithClient(localClient)

	//terra.ModuleBasics.RegisterRESTRoutes(ctx, router)
	mod.RegisterRESTRoutes(ctx, router)
	http.ListenAndServe(fmt.Sprintf(":%d", port), router)
}
