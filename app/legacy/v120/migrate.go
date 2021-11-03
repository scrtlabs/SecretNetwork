package legacy

import (
	"fmt"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/crypto/keys/multisig"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	v038auth "github.com/cosmos/cosmos-sdk/x/auth/legacy/v038"
	v039auth "github.com/cosmos/cosmos-sdk/x/auth/legacy/v039"
	v040auth "github.com/cosmos/cosmos-sdk/x/auth/legacy/v040"
	v036supply "github.com/cosmos/cosmos-sdk/x/bank/legacy/v036"
	v038bank "github.com/cosmos/cosmos-sdk/x/bank/legacy/v038"
	v040bank "github.com/cosmos/cosmos-sdk/x/bank/legacy/v040"
	v039crisis "github.com/cosmos/cosmos-sdk/x/crisis/legacy/v039"
	v040crisis "github.com/cosmos/cosmos-sdk/x/crisis/legacy/v040"
	v036distribution "github.com/cosmos/cosmos-sdk/x/distribution/legacy/v036"
	v038distr "github.com/cosmos/cosmos-sdk/x/distribution/legacy/v038"
	v040distr "github.com/cosmos/cosmos-sdk/x/distribution/legacy/v040"
	v038evidence "github.com/cosmos/cosmos-sdk/x/evidence/legacy/v038"
	v040evidence "github.com/cosmos/cosmos-sdk/x/evidence/legacy/v040"
	v039genutil "github.com/cosmos/cosmos-sdk/x/genutil/legacy/v039"
	v040genutil "github.com/cosmos/cosmos-sdk/x/genutil/legacy/v040"
	"github.com/cosmos/cosmos-sdk/x/genutil/types"
	v036gov "github.com/cosmos/cosmos-sdk/x/gov/legacy/v036"
	v040gov "github.com/cosmos/cosmos-sdk/x/gov/legacy/v040"
	v043gov "github.com/cosmos/cosmos-sdk/x/gov/legacy/v043"
	v039mint "github.com/cosmos/cosmos-sdk/x/mint/legacy/v039"
	v040mint "github.com/cosmos/cosmos-sdk/x/mint/legacy/v040"
	v036params "github.com/cosmos/cosmos-sdk/x/params/legacy/v036"
	v039slashing "github.com/cosmos/cosmos-sdk/x/slashing/legacy/v039"
	v040slashing "github.com/cosmos/cosmos-sdk/x/slashing/legacy/v040"
	v038staking "github.com/cosmos/cosmos-sdk/x/staking/legacy/v038"
	v040staking "github.com/cosmos/cosmos-sdk/x/staking/legacy/v040"
	v038upgrade "github.com/cosmos/cosmos-sdk/x/upgrade/legacy/v038"
	v106registration "github.com/enigmampc/SecretNetwork/x/registration/legacy/v106"
	v120registration "github.com/enigmampc/SecretNetwork/x/registration/legacy/v120"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	v106compute "github.com/enigmampc/SecretNetwork/x/compute/legacy/v106"
	v120compute "github.com/enigmampc/SecretNetwork/x/compute/legacy/v120"
	v106tokenswap "github.com/enigmampc/SecretNetwork/x/tokenswap/legacy/v106"
)

func migrateGenutil(oldGenState v039genutil.GenesisState) *types.GenesisState {
	return &types.GenesisState{
		GenTxs: oldGenState.GenTxs,
	}
}

// v039authMigrate accepts exported genesis state from v0.38 and migrates it to v0.39
// genesis state.
func v039authMigrate(oldAuthGenState v038auth.GenesisState, legacyCdc *codec.LegacyAmino) v039auth.GenesisState {
	accounts := make(v038auth.GenesisAccounts, len(oldAuthGenState.Accounts))

	for i, acc := range oldAuthGenState.Accounts {
		switch t := acc.(type) {
		case *v038auth.BaseAccount:
			pubKey := t.PubKey
			newAccount := v039auth.NewBaseAccount(t.Address, t.Coins, t.PubKey, t.AccountNumber, t.Sequence)

			if mpk, ok := newAccount.PubKey.(*multisig.LegacyAminoPubKey); ok {
				publicKeys := make([]cryptotypes.PubKey, len(mpk.PubKeys))

				// Manually Unmarshal multisig's inner public keys to secp256k1 type
				for i, rawPk := range mpk.PubKeys {
					var pk secp256k1.PubKey
					pkBytes, err := rawPk.MarshalAmino()
					if err != nil {
						panic("could not get multisig inner public key bytes")
					}
					legacyCdc.MustUnmarshal(pkBytes, &pk)

					publicKeys[i] = &pk
				}

				pubKey = multisig.NewLegacyAminoPubKey(int(mpk.Threshold), publicKeys)
			}

			accounts[i] = v039auth.NewBaseAccount(t.Address, t.Coins, pubKey, t.AccountNumber, t.Sequence)

		case *v038auth.BaseVestingAccount:
			accounts[i] = v039auth.NewBaseVestingAccount(
				v039auth.NewBaseAccount(t.Address, t.Coins, t.PubKey, t.AccountNumber, t.Sequence),
				t.OriginalVesting, t.DelegatedFree, t.DelegatedVesting, t.EndTime,
			)

		case *v038auth.ContinuousVestingAccount:
			accounts[i] = v039auth.NewContinuousVestingAccountRaw(
				v039auth.NewBaseVestingAccount(
					v039auth.NewBaseAccount(t.Address, t.Coins, t.PubKey, t.AccountNumber, t.Sequence),
					t.OriginalVesting, t.DelegatedFree, t.DelegatedVesting, t.EndTime,
				),
				t.StartTime,
			)

		case *v038auth.DelayedVestingAccount:
			accounts[i] = v039auth.NewDelayedVestingAccountRaw(
				v039auth.NewBaseVestingAccount(
					v039auth.NewBaseAccount(t.Address, t.Coins, t.PubKey, t.AccountNumber, t.Sequence),
					t.OriginalVesting, t.DelegatedFree, t.DelegatedVesting, t.EndTime,
				),
			)

		case *v038auth.ModuleAccount:
			accounts[i] = v039auth.NewModuleAccount(
				v039auth.NewBaseAccount(t.Address, t.Coins, t.PubKey, t.AccountNumber, t.Sequence),
				t.Name, t.Permissions...,
			)

		default:
			panic(fmt.Sprintf("unexpected account type: %T", acc))
		}
	}

	accounts = v038auth.SanitizeGenesisAccounts(accounts)

	if err := v038auth.ValidateGenAccounts(accounts); err != nil {
		panic(err)
	}

	return v039auth.NewGenesisState(oldAuthGenState.Params, accounts)
}

// Migrate migrates exported state from v0.39 to a v0.40 genesis state.
func Migrate(appState types.AppMap, clientCtx client.Context) types.AppMap {
	v106Codec := codec.NewLegacyAmino()
	v038auth.RegisterLegacyAminoCodec(v106Codec)
	v036gov.RegisterLegacyAminoCodec(v106Codec)
	v036params.RegisterLegacyAminoCodec(v106Codec)
	v036distribution.RegisterLegacyAminoCodec(v106Codec)
	v038upgrade.RegisterLegacyAminoCodec(v106Codec)

	v120Codec := clientCtx.Codec

	if appState[v038bank.ModuleName] != nil {
		// unmarshal relative source genesis application state
		var bankGenState v038bank.GenesisState
		v106Codec.MustUnmarshalJSON(appState[v038bank.ModuleName], &bankGenState)

		// unmarshal x/auth genesis state to retrieve all account balances
		var authGenState v039auth.GenesisState
		v106Codec.MustUnmarshalJSON(appState[v038auth.ModuleName], &authGenState)

		// unmarshal x/supply genesis state to retrieve total supply
		var supplyGenState v036supply.GenesisState
		v106Codec.MustUnmarshalJSON(appState[v036supply.ModuleName], &supplyGenState)

		// delete deprecated x/bank genesis state
		delete(appState, v038bank.ModuleName)

		// delete deprecated x/supply genesis state
		delete(appState, v036supply.ModuleName)

		// Migrate relative source genesis application state and marshal it into
		// the respective key.
		appState[v040bank.ModuleName] = v120Codec.MustMarshalJSON(v040bank.Migrate(bankGenState, authGenState, supplyGenState))
	}

	// remove balances from existing accounts
	if appState[v038auth.ModuleName] != nil {
		var v038authGenState v038auth.GenesisState
		v106Codec.MustUnmarshalJSON(appState[v038auth.ModuleName], &v038authGenState)

		v039authGenState := v039authMigrate(v038authGenState, v106Codec)

		// delete deprecated x/auth genesis state
		delete(appState, v038auth.ModuleName)

		// Migrate relative source genesis application state and marshal it into
		// the respective key.
		appState[v040auth.ModuleName] = v120Codec.MustMarshalJSON(v040auth.Migrate(v039authGenState))
	}

	// Migrate x/crisis.
	if appState[v039crisis.ModuleName] != nil {
		// unmarshal relative source genesis application state
		var crisisGenState v039crisis.GenesisState
		v106Codec.MustUnmarshalJSON(appState[v039crisis.ModuleName], &crisisGenState)

		// delete deprecated x/crisis genesis state
		delete(appState, v039crisis.ModuleName)

		// Migrate relative source genesis application state and marshal it into
		// the respective key.
		appState[v040crisis.ModuleName] = v120Codec.MustMarshalJSON(v040crisis.Migrate(crisisGenState))
	}

	// Migrate x/distribution.
	if appState[v038distr.ModuleName] != nil {
		// unmarshal relative source genesis application state
		var distributionGenState v038distr.GenesisState
		v106Codec.MustUnmarshalJSON(appState[v038distr.ModuleName], &distributionGenState)

		// delete deprecated x/distribution genesis state
		delete(appState, v038distr.ModuleName)

		// Migrate relative source genesis application state and marshal it into
		// the respective key.
		appState[v040distr.ModuleName] = v120Codec.MustMarshalJSON(v040distr.Migrate(distributionGenState))
	}

	// Migrate x/evidence.
	if appState[v038evidence.ModuleName] != nil {
		// unmarshal relative source genesis application state
		var evidenceGenState v038evidence.GenesisState
		v106Codec.MustUnmarshalJSON(appState[v038evidence.ModuleName], &evidenceGenState)

		// delete deprecated x/evidence genesis state
		delete(appState, v038evidence.ModuleName)

		// Migrate relative source genesis application state and marshal it into
		// the respective key.
		appState[v040evidence.ModuleName] = v120Codec.MustMarshalJSON(v040evidence.Migrate(evidenceGenState))
	}

	// Migrate x/gov.
	// NOTE: custom gov migration contains v043 migration step, but call it as v040
	if appState[v036gov.ModuleName] != nil {
		// unmarshal relative source genesis application state
		var govGenState036 v036gov.GenesisState
		v106Codec.MustUnmarshalJSON(appState[v036gov.ModuleName], &govGenState036)

		// delete deprecated x/gov genesis state
		delete(appState, v036gov.ModuleName)

		// First migrate to v0.40
		govGenState040 := v040gov.Migrate(govGenState036)

		// Migrate relative source genesis application state and marshal it into
		// the respective key.
		appState[v043gov.ModuleName] = v120Codec.MustMarshalJSON(v043gov.MigrateJSON(govGenState040))
	}

	// Migrate x/mint.
	if appState[v039mint.ModuleName] != nil {
		// unmarshal relative source genesis application state
		var mintGenState v039mint.GenesisState
		v106Codec.MustUnmarshalJSON(appState[v039mint.ModuleName], &mintGenState)

		// delete deprecated x/mint genesis state
		delete(appState, v039mint.ModuleName)

		// Migrate relative source genesis application state and marshal it into
		// the respective key.
		appState[v040mint.ModuleName] = v120Codec.MustMarshalJSON(v040mint.Migrate(mintGenState))
	}

	// Migrate x/slashing.
	if appState[v039slashing.ModuleName] != nil {
		// unmarshal relative source genesis application state
		var slashingGenState v039slashing.GenesisState
		v106Codec.MustUnmarshalJSON(appState[v039slashing.ModuleName], &slashingGenState)

		// delete deprecated x/slashing genesis state
		delete(appState, v039slashing.ModuleName)

		// fill empty cons address
		for address, info := range slashingGenState.SigningInfos {
			if info.Address.Empty() {
				if addr, err := sdk.ConsAddressFromBech32(address); err != nil {
					panic(err)
				} else {
					info.Address = addr
				}

				slashingGenState.SigningInfos[address] = info
			}
		}

		// Migrate relative source genesis application state and marshal it into
		// the respective key.
		appState[v040slashing.ModuleName] = v120Codec.MustMarshalJSON(v040slashing.Migrate(slashingGenState))
	}

	// Migrate x/staking.
	if appState[v038staking.ModuleName] != nil {
		// unmarshal relative source genesis application state
		var stakingGenState v038staking.GenesisState
		v106Codec.MustUnmarshalJSON(appState[v038staking.ModuleName], &stakingGenState)

		// delete deprecated x/staking genesis state
		delete(appState, v038staking.ModuleName)

		// Migrate relative source genesis application state and marshal it into
		// the respective key.
		appState[v040staking.ModuleName] = v120Codec.MustMarshalJSON(v040staking.Migrate(stakingGenState))
	}

	// Migrate x/genutil
	if appState[v039genutil.ModuleName] != nil {
		// unmarshal relative source genesis application state
		var genutilGenState v039genutil.GenesisState
		v106Codec.MustUnmarshalJSON(appState[v039genutil.ModuleName], &genutilGenState)

		// delete deprecated x/staking genesis state
		delete(appState, v039genutil.ModuleName)

		// Migrate relative source genesis application state and marshal it into
		// the respective key.
		appState[v040genutil.ModuleName] = v120Codec.MustMarshalJSON(migrateGenutil(genutilGenState))
	}

	if appState[v106compute.ModuleName] != nil {
		// unmarshal relative source genesis application state
		var computeGenState v106compute.GenesisState
		v106Codec.MustUnmarshalJSON(appState[v106compute.ModuleName], &computeGenState)

		// delete deprecated x/wasm genesis state
		delete(appState, v106compute.ModuleName)

		// Migrate relative source genesis application state and marshal it into
		// the respective key.
		appState[v120compute.ModuleName] = v120Codec.MustMarshalJSON(v120compute.Migrate(computeGenState))
	}

	if appState[v106registration.ModuleName] != nil {
		// unmarshal relative source genesis application state
		var registerGenState v106registration.GenesisState
		v106Codec.MustUnmarshalJSON(appState[v106registration.ModuleName], &registerGenState)

		// delete deprecated x/staking genesis state
		delete(appState, v106registration.ModuleName)

		// Migrate relative source genesis application state and marshal it into
		// the respective key.
		appState[v120registration.ModuleName] = v120Codec.MustMarshalJSON(v120registration.Migrate(registerGenState))
	}

	if appState[v106tokenswap.ModuleName] != nil {
		// Token Swap module is discontinued
		delete(appState, v106tokenswap.ModuleName)
	}

	return appState
}
