package rest

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/rest"
	"github.com/cosmwasm/wasmd/x/wasm/internal/keeper"
	"github.com/cosmwasm/wasmd/x/wasm/internal/types"

	"github.com/cosmos/cosmos-sdk/client/context"
	"github.com/gorilla/mux"
)

func registerQueryRoutes(cliCtx context.CLIContext, r *mux.Router) {
	r.HandleFunc("/wasm/code", listCodesHandlerFn(cliCtx)).Methods("GET")
	r.HandleFunc("/wasm/code/{codeID}", queryCodeHandlerFn(cliCtx)).Methods("GET")
	r.HandleFunc("/wasm/code/{codeID}/contracts", listContractsByCodeHandlerFn(cliCtx)).Methods("GET")
	r.HandleFunc("/wasm/contract", listAllContractsHandlerFn(cliCtx)).Methods("GET")
	r.HandleFunc("/wasm/contract/{contractAddr}", queryContractHandlerFn(cliCtx)).Methods("GET")
	r.HandleFunc("/wasm/contract/{contractAddr}/state", queryContractStateAllHandlerFn(cliCtx)).Methods("GET")
	r.HandleFunc("/wasm/contract/{contractAddr}/smart/{query}", queryContractStateSmartHandlerFn(cliCtx)).Queries("encoding", "{encoding}").Methods("GET")
	r.HandleFunc("/wasm/contract/{contractAddr}/raw/{key}", queryContractStateRawHandlerFn(cliCtx)).Queries("encoding", "{encoding}").Methods("GET")
}

func listCodesHandlerFn(cliCtx context.CLIContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		route := fmt.Sprintf("custom/%s/%s", types.QuerierRoute, keeper.QueryListCode)
		res, _, err := cliCtx.Query(route)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		rest.PostProcessResponse(w, cliCtx, string(res))
	}
}

func queryCodeHandlerFn(cliCtx context.CLIContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		codeID, err := strconv.ParseUint(mux.Vars(r)["codeID"], 10, 64)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		route := fmt.Sprintf("custom/%s/%s/%d", types.QuerierRoute, keeper.QueryGetCode, codeID)
		res, _, err := cliCtx.Query(route)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		if len(res) == 0 {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, "contract not found")
			return
		}
		var code keeper.GetCodeResponse
		err = json.Unmarshal(res, &code)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		if len(code.Code) == 0 {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, "contract not found")
			return
		}

		rest.PostProcessResponse(w, cliCtx, code)
	}
}

func listContractsByCodeHandlerFn(cliCtx context.CLIContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		codeID, err := strconv.ParseUint(mux.Vars(r)["codeID"], 10, 64)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		route := fmt.Sprintf("custom/%s/%s/%d", types.QuerierRoute, keeper.QueryListContractByCode, codeID)
		res, _, err := cliCtx.Query(route)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		rest.PostProcessResponse(w, cliCtx, string(res))
	}
}

func listAllContractsHandlerFn(cliCtx context.CLIContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		route := fmt.Sprintf("custom/%s/%s", types.QuerierRoute, keeper.QueryListContracts)
		res, _, err := cliCtx.Query(route)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())

			return
		}
		rest.PostProcessResponse(w, cliCtx, string(res))
	}
}

func queryContractHandlerFn(cliCtx context.CLIContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		addr, err := sdk.AccAddressFromBech32(mux.Vars(r)["contractAddr"])
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		route := fmt.Sprintf("custom/%s/%s/%s", types.QuerierRoute, keeper.QueryGetContract, addr.String())
		res, _, err := cliCtx.Query(route)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		rest.PostProcessResponse(w, cliCtx, string(res))
	}
}

func queryContractStateAllHandlerFn(cliCtx context.CLIContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		addr, err := sdk.AccAddressFromBech32(mux.Vars(r)["contractAddr"])
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		route := fmt.Sprintf("custom/%s/%s/%s/%s", types.QuerierRoute, keeper.QueryGetContractState, addr.String(), keeper.QueryMethodContractStateAll)
		res, _, err := cliCtx.Query(route)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		// parse res
		var resultData []types.Model
		err = json.Unmarshal(res, &resultData)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		rest.PostProcessResponse(w, cliCtx, resultData)
	}
}

func queryContractStateRawHandlerFn(cliCtx context.CLIContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		decoder := newArgDecoder(hex.DecodeString)
		addr, err := sdk.AccAddressFromBech32(mux.Vars(r)["contractAddr"])
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		decoder.encoding = mux.Vars(r)["encoding"]
		queryData, err := decoder.DecodeString(mux.Vars(r)["key"])
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		route := fmt.Sprintf("custom/%s/%s/%s/%s", types.QuerierRoute, keeper.QueryGetContractState, addr.String(), keeper.QueryMethodContractStateRaw)

		res, _, err := cliCtx.QueryWithData(route, queryData)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		// parse res
		var resultData []types.Model
		err = json.Unmarshal(res, &resultData)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		rest.PostProcessResponse(w, cliCtx, resultData)
	}
}

type smartResponse struct {
	Smart []byte `json:"smart"`
}

func queryContractStateSmartHandlerFn(cliCtx context.CLIContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		decoder := newArgDecoder(hex.DecodeString)

		addr, err := sdk.AccAddressFromBech32(mux.Vars(r)["contractAddr"])
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		decoder.encoding = mux.Vars(r)["encoding"]

		route := fmt.Sprintf("custom/%s/%s/%s/%s", types.QuerierRoute, keeper.QueryGetContractState, addr.String(), keeper.QueryMethodContractStateSmart)

		queryData, err := decoder.DecodeString(mux.Vars(r)["query"])
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		res, _, err := cliCtx.QueryWithData(route, queryData)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		// return as raw bytes (to be base64-encoded)
		responseData := smartResponse{Smart: res}
		rest.PostProcessResponse(w, cliCtx, responseData)
	}
}

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
