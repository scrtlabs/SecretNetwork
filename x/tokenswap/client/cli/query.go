package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client/context"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/enigmampc/SecretNetwork/x/tokenswap/types"
)

// GetQueryCmd queries information about a Ethereum tx hash
func GetQueryCmd(queryRoute string, cdc *codec.Codec) *cobra.Command {
	return &cobra.Command{
		Use:   "get [ethereum-tx-hash]",
		Short: "Query past token swap request by Ethereum tx hash",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cliCtx := context.NewCLIContext().WithCodec(cdc)

			ethereumTxHash, err := types.HexToTxHash(args[0])
			if err != nil {
				return err
			}

			bz, err := cdc.MarshalJSON(types.NewGetTokenSwapParams(ethereumTxHash))
			if err != nil {
				return err
			}

			route := fmt.Sprintf("custom/%s/%s", queryRoute, types.GetTokenSwapRoute)
			res, _, err := cliCtx.QueryWithData(route, bz)
			if err != nil {
				return err
			}

			var out types.TokenSwapRecord
			err = cdc.UnmarshalJSON(res, &out)
			if err != nil {
				return err
			}

			return cliCtx.PrintOutput(out)
		},
	}
}
