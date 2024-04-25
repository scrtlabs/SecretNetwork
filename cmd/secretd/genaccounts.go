package main

// import (
// 	"bufio"
// 	"encoding/json"
// 	"errors"
// 	"fmt"
// 	"os"
// 	"runtime/debug"

// 	"cosmossdk.io/core/address"
// 	"github.com/cosmos/cosmos-sdk/client"
// 	"github.com/cosmos/cosmos-sdk/crypto/keyring"
// 	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
// 	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
// 	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
// 	"github.com/gogo/protobuf/codec"

// 	"github.com/spf13/cobra"

// 	"github.com/cosmos/cosmos-sdk/client/flags"
// 	"github.com/cosmos/cosmos-sdk/server"
// 	sdk "github.com/cosmos/cosmos-sdk/types"
// 	"github.com/cosmos/cosmos-sdk/x/genutil"
// )

/*
const (
	flagClientHome   = "home-client"
	flagVestingStart = "vesting-start-time"
	flagVestingEnd   = "vesting-end-time"
	flagVestingAmt   = "vesting-amount"
)
*/

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime/debug"

	//"cosmossdk.io/core/address"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	"github.com/scrtlabs/SecretNetwork/app"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/server"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/genutil"

	"github.com/cosmos/cosmos-sdk/codec"
	authvesting "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
)

const (
	flagClientHome   = "home-client"
	flagVestingStart = "vesting-start-time"
	flagVestingEnd   = "vesting-end-time"
	flagVestingAmt   = "vesting-amount"

	flagAppendMode = "append"
	flagModuleName = "module-name"
)

// AddGenesisAccountCmd returns add-genesis-account cobra Command.
func _AddGenesisAccountCmd(defaultNodeHome string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-genesis-account [address_or_key_name] [coin][,[coin]]",
		Short: "Add a genesis account to genesis.json",
		Long: `Add a genesis account to genesis.json. The provided account must specify
the account address or key name and a list of initial coins. If a key name is given,
the address will be looked up in the local Keybase. The list of initial tokens must
contain valid denominations. Accounts may optionally be supplied with vesting parameters.
`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			cdc := clientCtx.Codec

			serverCtx := server.GetServerContextFromCmd(cmd)
			config := serverCtx.Config

			config.SetRoot(clientCtx.HomeDir)

			addr, err := sdk.AccAddressFromBech32(args[0])
			if err != nil {
				inBuf := bufio.NewReader(cmd.InOrStdin())
				keyringBackend, err := cmd.Flags().GetString(flags.FlagKeyringBackend)
				if err != nil {
					return err
				}

				// attempt to lookup address from Keybase if no address was provided
				kb, err := keyring.New(sdk.KeyringServiceName(), keyringBackend, clientCtx.HomeDir, inBuf, clientCtx.Codec)
				if err != nil {
					return err
				}

				info, err := kb.Key(args[0])
				if err != nil {
					return fmt.Errorf("failed to get address from Keybase: %w", err)
				}

				addr, _ = info.GetAddress()
			}

			// create concrete account type based on input parameters
			var genAccount authtypes.GenesisAccount

			coins, err := sdk.ParseCoinsNormalized(args[1])
			if err != nil {
				return fmt.Errorf("failed to parse coins: %w", err)
			}

			balances := banktypes.Balance{Address: addr.String(), Coins: coins.Sort()}

			/*

				vestingStart, _ := cmd.Flags().GetInt64(flagVestingStart)
				vestingEnd, _ := cmd.Flags().GetInt64(flagVestingEnd)
				vestingAmtStr, _ := cmd.Flags().GetString(flagVestingAmt)

				vestingAmt, err := sdk.ParseCoins(vestingAmtStr)
				if err != nil {
					return fmt.Errorf("failed to parse vesting amount: %w", err)
				}

				baseAccount := authtypes.NewBaseAccount(addr, nil, 0, 0)

				if !vestingAmt.IsZero() {
					baseVestingAccount := authvesting.NewBaseVestingAccount(baseAccount, vestingAmt.Sort(), vestingEnd)

					if (balances.Coins.IsZero() && !baseVestingAccount.OriginalVesting.IsZero()) ||
						baseVestingAccount.OriginalVesting.IsAnyGT(balances.Coins) {
						return errors.New("vesting amount cannot be greater than total amount")
					}

					switch {
					case vestingStart != 0 && vestingEnd != 0:
						genAccount = authvesting.NewContinuousVestingAccountRaw(baseVestingAccount, vestingStart)

					case vestingEnd != 0:
						genAccount = authvesting.NewDelayedVestingAccountRaw(baseVestingAccount)

					default:
						return errors.New("invalid vesting parameters; must supply start and end time or end time")
					}
				} else {
					genAccount = baseAccount
				}
			*/

			// TODO: If vesting is enabled, should remove following lines:
			baseAccount := authtypes.NewBaseAccount(addr, nil, 0, 0)
			genAccount = baseAccount

			if err := genAccount.Validate(); err != nil {
				return fmt.Errorf("failed to validate new genesis account: %w", err)
			}

			genFile := config.GenesisFile()
			appState, genDoc, err := genutiltypes.GenesisStateFromGenFile(genFile)
			if err != nil {
				return fmt.Errorf("failed to unmarshal genesis state: %w", err)
			}

			authGenState := authtypes.GetGenesisStateFromAppState(cdc, appState)

			accs, err := authtypes.UnpackAccounts(authGenState.Accounts)
			if err != nil {
				return fmt.Errorf("failed to get accounts from any: %w", err)
			}

			if accs.Contains(addr) {
				return fmt.Errorf("cannot add account at existing address %s", addr)
			}

			// Add the new account to the set of genesis accounts and sanitize the
			// accounts afterwards.
			accs = append(accs, genAccount)
			accs = authtypes.SanitizeGenesisAccounts(accs)

			genAccs, err := authtypes.PackAccounts(accs)
			if err != nil {
				return fmt.Errorf("failed to convert accounts into any's: %w", err)
			}
			authGenState.Accounts = genAccs

			authGenStateBz, err := cdc.MarshalJSON(&authGenState)
			if err != nil {
				return fmt.Errorf("failed to marshal auth genesis state: %w", err)
			}

			appState[authtypes.ModuleName] = authGenStateBz

			bankGenState := banktypes.GetGenesisStateFromAppState(cdc, appState)
			bankGenState.Balances = append(bankGenState.Balances, balances)
			bankGenState.Balances = banktypes.SanitizeGenesisBalances(bankGenState.Balances)

			bankGenStateBz, err := cdc.MarshalJSON(bankGenState)
			if err != nil {
				return fmt.Errorf("failed to marshal bank genesis state: %w", err)
			}

			appState[banktypes.ModuleName] = bankGenStateBz

			appStateJSON, err := json.Marshal(appState)
			if err != nil {
				return fmt.Errorf("failed to marshal application genesis state: %w", err)
			}

			genDoc.AppState = appStateJSON
			return genutil.ExportGenesisFile(genDoc, genFile)
		},
	}

	cmd.Flags().String(flags.FlagHome, defaultNodeHome, "The application home directory")
	// cmd.Flags().String(flags.FlagKeyringBackend, flags.DefaultKeyringBackend, "Select keyring's backend (os|file|kwallet|pass|test)")
	// cmd.Flags().String(flagClientHome, defaultClientHome, "client's home directory")
	// cmd.Flags().String(flagVestingAmt, "", "amount of coins for vesting accounts")
	// cmd.Flags().Uint64(flagVestingStart, 0, "schedule start time (unix epoch) for vesting accounts")
	// cmd.Flags().Uint64(flagVestingEnd, 0, "schedule end time (unix epoch) for vesting accounts")
	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

func AddGenesisAccountCmd(defaultNodeHome string, eCfg app.EncodingConfig) *cobra.Command {
	addressCodec := eCfg.TxConfig.SigningContext().AddressCodec()
	cmd := &cobra.Command{
		Use:   "add-genesis-account [address_or_key_name] [coin][,[coin]]",
		Short: "Add a genesis account to genesis.json",
		Long: `Add a genesis account to genesis.json. The provided account must specify
the account address or key name and a list of initial coins. If a key name is given,
the address will be looked up in the local Keybase. The list of initial tokens must
contain valid denominations. Accounts may optionally be supplied with vesting parameters.
`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			defer func() {
				if r := recover(); r != nil {
					fmt.Fprintf(os.Stderr, "Panic debug stack:\n%s\n", string(debug.Stack()))
				}
			}()
			clientCtx := client.GetClientContextFromCmd(cmd)
			serverCtx := server.GetServerContextFromCmd(cmd)
			config := serverCtx.Config

			config.SetRoot(clientCtx.HomeDir)

			var kr keyring.Keyring
			addr, err := addressCodec.StringToBytes(args[0])
			// fmt.Fprintf(os.Stderr, "address: [%s]\n", string(addr))
			if err != nil {
				inBuf := bufio.NewReader(cmd.InOrStdin())
				keyringBackend, _ := cmd.Flags().GetString(flags.FlagKeyringBackend)

				if keyringBackend != "" && clientCtx.Keyring == nil {
					var err error
					kr, err = keyring.New(sdk.KeyringServiceName(), keyringBackend, clientCtx.HomeDir, inBuf, clientCtx.Codec)
					if err != nil {
						// panic(err)
						return err
					}
				} else {
					kr = clientCtx.Keyring
				}
				if kr != nil {
					k, err := kr.Key(args[0])
					if err != nil {
						// panic(err)
						return fmt.Errorf("failed to get address from Keyring: %w", err)
					}

					addr, err = k.GetAddress()
					if err != nil {
						// panic(err)
						return err
					}
				}
			}
			if len(string(addr)) == 0 {
				return fmt.Errorf("empty address")
			}
			appendflag, _ := cmd.Flags().GetBool(flagAppendMode)
			vestingStart, _ := cmd.Flags().GetInt64(flagVestingStart)
			vestingEnd, _ := cmd.Flags().GetInt64(flagVestingEnd)
			vestingAmtStr, _ := cmd.Flags().GetString(flagVestingAmt)
			moduleNameStr, _ := cmd.Flags().GetString(flagModuleName)

			return AddGenesisAccount(clientCtx.Codec, addr, appendflag, config.GenesisFile(), args[1], vestingAmtStr, vestingStart, vestingEnd, moduleNameStr)
		},
	}

	cmd.Flags().String(flags.FlagHome, defaultNodeHome, "The application home directory")
	cmd.Flags().String(flags.FlagKeyringBackend, flags.DefaultKeyringBackend, "Select keyring's backend (os|file|kwallet|pass|test)")
	cmd.Flags().String(flagVestingAmt, "", "amount of coins for vesting accounts")
	cmd.Flags().Int64(flagVestingStart, 0, "schedule start time (unix epoch) for vesting accounts")
	cmd.Flags().Int64(flagVestingEnd, 0, "schedule end time (unix epoch) for vesting accounts")
	cmd.Flags().Bool(flagAppendMode, false, "append the coins to an account already in the genesis.json file")
	cmd.Flags().String(flagModuleName, "", "module account name")
	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

func AddGenesisAccount(
	cdc codec.Codec,
	accAddr sdk.AccAddress,
	appendAcct bool,
	genesisFileURL, amountStr, vestingAmtStr string,
	vestingStart, vestingEnd int64,
	moduleName string,
) error {
	// Added for debug purposes
	// fmt.Fprintln(os.Stderr, "------------- AddGenesisAccount --------------")

	// if tCodec, ok := cdc.(*codec.ProtoCodec); ok {
	// 	ir := tCodec.InterfaceRegistry()
	// 	ia := ir.ListAllInterfaces()
	// 	fmt.Fprintf(os.Stderr, "* AddGenesisAccount Codec inspection: %s\n", reflect.ValueOf(cdc).Type())
	// 	fmt.Fprintf(os.Stderr, "* AddGenesisAccount Interface Regist: %s\n", reflect.ValueOf(ir).Type())
	// 	for _, i := range ia {
	// 		ii := ir.ListImplementations(i)
	// 		fmt.Fprintf(os.Stderr, "\t%s -> %s\n", i, ii)
	// 	}
	// 	fmt.Fprintln(os.Stderr, "----------------------------------------------")
	// }

	coins, err := sdk.ParseCoinsNormalized(amountStr)
	if err != nil {
		return fmt.Errorf("failed to parse coins: %w", err)
	}

	vestingAmt, err := sdk.ParseCoinsNormalized(vestingAmtStr)
	if err != nil {
		return fmt.Errorf("failed to parse vesting amount: %w", err)
	}

	// create concrete account type based on input parameters
	var genAccount authtypes.GenesisAccount

	balances := banktypes.Balance{Address: accAddr.String(), Coins: coins.Sort()}
	baseAccount := authtypes.NewBaseAccount(accAddr, nil, 0, 0)

	if !vestingAmt.IsZero() {
		baseVestingAccount, err := authvesting.NewBaseVestingAccount(baseAccount, vestingAmt.Sort(), vestingEnd)
		if err != nil {
			return fmt.Errorf("failed to create base vesting account: %w", err)
		}

		if (balances.Coins.IsZero() && !baseVestingAccount.OriginalVesting.IsZero()) ||
			baseVestingAccount.OriginalVesting.IsAnyGT(balances.Coins) {
			return errors.New("vesting amount cannot be greater than total amount")
		}

		switch {
		case vestingStart != 0 && vestingEnd != 0:
			genAccount = authvesting.NewContinuousVestingAccountRaw(baseVestingAccount, vestingStart)

		case vestingEnd != 0:
			genAccount = authvesting.NewDelayedVestingAccountRaw(baseVestingAccount)

		default:
			return errors.New("invalid vesting parameters; must supply start and end time or end time")
		}
	} else if moduleName != "" {
		genAccount = authtypes.NewEmptyModuleAccount(moduleName, authtypes.Burner, authtypes.Minter)
	} else {
		genAccount = baseAccount
	}

	if err := genAccount.Validate(); err != nil {
		return fmt.Errorf("failed to validate new genesis account: %w", err)
	}

	appState, appGenesis, err := genutiltypes.GenesisStateFromGenFile(genesisFileURL)
	if err != nil {
		return fmt.Errorf("failed to unmarshal genesis state: %w", err)
	}

	authGenState := authtypes.GetGenesisStateFromAppState(cdc, appState)

	accs, err := authtypes.UnpackAccounts(authGenState.Accounts)
	if err != nil {
		return fmt.Errorf("failed to get accounts from any: %w", err)
	}

	bankGenState := banktypes.GetGenesisStateFromAppState(cdc, appState)
	if accs.Contains(accAddr) {
		if !appendAcct {
			return fmt.Errorf(" Account %s already exists\nUse `append` flag to append account at existing address", accAddr)
		}

		genesisB := banktypes.GetGenesisStateFromAppState(cdc, appState)
		for idx, acc := range genesisB.Balances {
			if acc.Address != accAddr.String() {
				continue
			}

			updatedCoins := acc.Coins.Add(coins...)
			bankGenState.Balances[idx] = banktypes.Balance{Address: accAddr.String(), Coins: updatedCoins.Sort()}
			break
		}
	} else {
		// Add the new account to the set of genesis accounts and sanitize the accounts afterwards.
		accs = append(accs, genAccount)
		accs = authtypes.SanitizeGenesisAccounts(accs)

		genAccs, err := authtypes.PackAccounts(accs)
		if err != nil {
			return fmt.Errorf("failed to convert accounts into any's: %w", err)
		}
		authGenState.Accounts = genAccs

		authGenStateBz, err := cdc.MarshalJSON(&authGenState)
		if err != nil {
			return fmt.Errorf("failed to marshal auth genesis state: %w", err)
		}
		appState[authtypes.ModuleName] = authGenStateBz

		bankGenState.Balances = append(bankGenState.Balances, balances)
	}

	bankGenState.Balances = banktypes.SanitizeGenesisBalances(bankGenState.Balances)

	bankGenState.Supply = bankGenState.Supply.Add(balances.Coins...)

	bankGenStateBz, err := cdc.MarshalJSON(bankGenState)
	if err != nil {
		return fmt.Errorf("failed to marshal bank genesis state: %w", err)
	}
	appState[banktypes.ModuleName] = bankGenStateBz

	appStateJSON, err := json.Marshal(appState)
	if err != nil {
		return fmt.Errorf("failed to marshal application genesis state: %w", err)
	}

	appGenesis.AppState = appStateJSON
	return genutil.ExportGenesisFile(appGenesis, genesisFileURL)
}
