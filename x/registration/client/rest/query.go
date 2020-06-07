package rest

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	// sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/rest"
	"github.com/enigmampc/EnigmaBlockchain/x/registration/internal/keeper"
	"github.com/enigmampc/EnigmaBlockchain/x/registration/internal/types"

	"github.com/cosmos/cosmos-sdk/client/context"
	"github.com/gorilla/mux"
)

func registerQueryRoutes(cliCtx context.CLIContext, r *mux.Router) {
	r.HandleFunc("/reg/code", listCodesHandlerFn(cliCtx)).Methods("GET")
}

func listCodesHandlerFn(cliCtx context.CLIContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cliCtx, ok := rest.ParseQueryHeightOrReturnBadRequest(w, cliCtx, r)
		if !ok {
			return
		}

		route := fmt.Sprintf("custom/%s/%s", types.QuerierRoute, keeper.QueryEncryptedSeed)
		res, height, err := cliCtx.Query(route)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		cliCtx = cliCtx.WithHeight(height)
		rest.PostProcessResponse(w, cliCtx, json.RawMessage(res))
	}
}

//
//func queryCodeHandlerFn(cliCtx context.CLIContext) http.HandlerFunc {
//	return func(w http.ResponseWriter, r *http.Request) {
//		codeID, err := strconv.ParseUint(mux.Vars(r)["codeID"], 10, 64)
//		if err != nil {
//			rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
//			return
//		}
//
//		cliCtx, ok := rest.ParseQueryHeightOrReturnBadRequest(w, cliCtx, r)
//		if !ok {
//			return
//		}
//
//		route := fmt.Sprintf("custom/%s/%s/%d", types.QuerierRoute, keeper.QueryGetCode, codeID)
//		res, height, err := cliCtx.Query(route)
//		if err != nil {
//			rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
//			return
//		}
//		if len(res) == 0 {
//			rest.WriteErrorResponse(w, http.StatusNotFound, "contract not found")
//			return
//		}
//
//		cliCtx = cliCtx.WithHeight(height)
//		rest.PostProcessResponse(w, cliCtx, json.RawMessage(res))
//	}
//}

//type smartResponse struct {
//	Smart []byte `json:"smart"`
//}

type argumentDecoder struct {
	// dec is the default decoder
	dec      func(string) ([]byte, error)
	encoding string
}

func newArgDecoder(def func(string) ([]byte, error)) *argumentDecoder {
	return &argumentDecoder{dec: def}
}

func (a *argumentDecoder) DecodeString(s string) ([]byte, error) {

	switch a.encoding {
	case "hex":
		return hex.DecodeString(s)
	case "base64":
		return base64.StdEncoding.DecodeString(s)
	default:
		return a.dec(s)
	}
}
