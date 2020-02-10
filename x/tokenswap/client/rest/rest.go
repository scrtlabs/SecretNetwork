package rest

import (
	"fmt"
	"net/http"

	"github.com/cosmos/cosmos-sdk/client/context"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/rest"
	"github.com/cosmos/cosmos-sdk/x/auth/client/utils"

	"github.com/gorilla/mux"

	"github.com/enigmampc/Enigmachain/x/tokenswap/types"
)

const (
	restEthereumTxHash = "ethereumTxHash"
)

type getTokenSwapReq struct {
	BaseReq        rest.BaseReq `json:"base_req"`
	EthereumTxHash string       `json:"ethereum_tx_hash"`
}

type createTokenSwapReq struct {
	BaseReq        rest.BaseReq `json:"base_req"`
	EthereumTxHash string       `json:"ethereum_tx_hash"`
	EthereumSender string       `json:"ethereum_sender"`
	Receiver       string       `json:"amount_uscrt"`
	AmountENG      string       `json:"receiver"`
}

// RegisterRESTRoutes - Central function to define routes that get registered by the main application
func RegisterRESTRoutes(cliCtx context.CLIContext, r *mux.Router, storeName string) {
	r.HandleFunc(
		fmt.Sprintf(
			"/%s/get/{%s}",
			storeName,
			restEthereumTxHash,
		),
		getTokenSwapHandler(cliCtx, storeName),
	).Methods("GET")
	r.HandleFunc(
		fmt.Sprintf("/%s/create", storeName),
		createTokenSwapHandler(cliCtx),
	).Methods("POST")
}

func getTokenSwapHandler(cliCtx context.CLIContext, storeName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		ethereumTxHash := vars[restEthereumTxHash]

		bz, err := cliCtx.Codec.MarshalJSON(types.NewGetTokenSwapParams(ethereumTxHash))
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}

		route := fmt.Sprintf("custom/%s/%s", storeName, types.GetTokenSwapRoute)
		res, _, err := cliCtx.QueryWithData(route, bz)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}

		rest.PostProcessResponse(w, cliCtx, res)
	}
}

func createTokenSwapHandler(cliCtx context.CLIContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createTokenSwapReq

		if !rest.ReadRESTReq(w, r, cliCtx.Codec, &req) {
			rest.WriteErrorResponse(w, http.StatusBadRequest, "failed to parse request")
			return
		}

		baseReq := req.BaseReq.Sanitize()
		if !baseReq.ValidateBasic(w) {
			return
		}

		ethereumTxHash := req.EthereumTxHash
		ethereumSender := req.EthereumSender
		amountENG := req.AmountENG

		receiver, err := sdk.AccAddressFromBech32(req.Receiver)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}

		msg := types.NewMsgTokenSwap(
			ethereumTxHash,
			ethereumSender,
			receiver,
			amountENG,
		)
		if err := msg.ValidateBasic(); err != nil {
			rest.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}

		utils.WriteGenerateStdTxResponse(w, cliCtx, baseReq, []sdk.Msg{msg})
	}
}
