package eip191

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	signingv1beta1 "cosmossdk.io/api/cosmos/tx/signing/v1beta1"
	"cosmossdk.io/x/tx/signing"
	"cosmossdk.io/x/tx/signing/aminojson"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

const EIP191MessagePrefix = "\x19Ethereum Signed Message:\n"

var _ signing.SignModeHandler = SignModeHandler{}

// SignModeHandler is the SIGN_MODE_EIP_191 implementation of signing.SignModeHandler.
type SignModeHandler struct {
	aminoJsonSignModeHandler *aminojson.SignModeHandler
}

type SignModeHandlerOptions struct {
	AminoJsonSignModeHandler *aminojson.SignModeHandler
}

func (s SignModeHandler) Mode() signingv1beta1.SignMode {
	return signingv1beta1.SignMode_SIGN_MODE_EIP_191 //nolint:staticcheck
}

func NewSignModeHandler(options SignModeHandlerOptions) *SignModeHandler {
	h := &SignModeHandler{
		aminoJsonSignModeHandler: options.AminoJsonSignModeHandler,
	}
	return h
}

func (s SignModeHandler) GetSignBytes(ctx context.Context, signerData signing.SignerData, txData signing.TxData) ([]byte, error) {
	aminoJSONBz, err := s.aminoJsonSignModeHandler.GetSignBytes(ctx, signerData, txData)
	if err != nil {
		return nil, err
	}

	aminoJSONPrettyString, err := prettyAmino(string(aminoJSONBz))
	if err != nil {
		return nil, sdkerrors.ErrInvalidRequest.Wrap(fmt.Sprintf("SignMode_SIGN_MODE_EIP_191 cannot parse into pretty amino json: '%v': '%+v'", string(aminoJSONBz), err))
	}

	aminoJSONBz = []byte(aminoJSONPrettyString)

	bz := append(
		[]byte(EIP191MessagePrefix),
		[]byte(strconv.Itoa(len(aminoJSONBz)))...,
	)

	bz = append(bz, aminoJSONBz...)

	return bz, nil
}

func prettyAmino(str string) (string, error) {
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, []byte(str), "", "    "); err != nil {
		return "", err
	}
	return prettyJSON.String(), nil
}

var _ signing.SignModeHandler = (*SignModeHandler)(nil)
