package rest

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/types/rest"

	"github.com/scrtlabs/SecretNetwork/x/registration/internal/keeper"
	"github.com/scrtlabs/SecretNetwork/x/registration/internal/types"

	"github.com/gorilla/mux"
)

func registerQueryRoutes(cliCtx client.Context, r *mux.Router) {
	r.HandleFunc("/reg/seed/{pubkey}", seedCheckHandlerFn(cliCtx)).Methods("GET")
	r.HandleFunc("/reg/tx-key", txPubkeyHandlerFn(cliCtx)).Methods("GET")
	r.HandleFunc("/reg/registration-key", seedCertificateHandlerFn(cliCtx)).Methods("GET")
}

func seedCheckHandlerFn(cliCtx client.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cliCtx, ok := rest.ParseQueryHeightOrReturnBadRequest(w, cliCtx, r)
		if !ok {
			return
		}

		pubkey := mux.Vars(r)["pubkey"]

		if len(pubkey) != 64 {
			rest.WriteErrorResponse(w, http.StatusBadRequest, "Malformed public key")
			return
		}

		route := fmt.Sprintf("custom/%s/%s/%s", types.QuerierRoute, keeper.QueryEncryptedSeed, pubkey)
		seed, height, err := cliCtx.Query(route)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		cliCtx = cliCtx.WithHeight(height)

		// todo: add this to types
		res := []byte(fmt.Sprintf(`{"Seed":"%s"}`, hex.EncodeToString(seed)))

		rest.PostProcessResponse(w, cliCtx, json.RawMessage(res))
	}
}

func txPubkeyHandlerFn(cliCtx client.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cliCtx, ok := rest.ParseQueryHeightOrReturnBadRequest(w, cliCtx, r)
		if !ok {
			return
		}

		route := fmt.Sprintf("custom/%s/%s", types.QuerierRoute, keeper.QueryMasterKey)
		res, height, err := cliCtx.Query(route)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		cliCtx = cliCtx.WithHeight(height)

		var keys types.GenesisState

		err = json.Unmarshal(res, &keys)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		// todo: add this to types
		res = []byte(fmt.Sprintf(`{"TxKey":"%s"}`, base64.StdEncoding.EncodeToString(keys.IoMasterKey.Bytes)))

		rest.PostProcessResponse(w, cliCtx, json.RawMessage(res))
	}
}

func seedCertificateHandlerFn(cliCtx client.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cliCtx, ok := rest.ParseQueryHeightOrReturnBadRequest(w, cliCtx, r)
		if !ok {
			return
		}

		route := fmt.Sprintf("custom/%s/%s", types.QuerierRoute, keeper.QueryMasterKey)
		res, height, err := cliCtx.Query(route)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		cliCtx = cliCtx.WithHeight(height)

		var keys types.GenesisState

		err = json.Unmarshal(res, &keys)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		// todo: add this to types
		res = []byte(fmt.Sprintf(`{"RegistrationKey":"%s"}`, base64.StdEncoding.EncodeToString(keys.NodeExchMasterKey.Bytes)))

		rest.PostProcessResponse(w, cliCtx, json.RawMessage(res))
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
