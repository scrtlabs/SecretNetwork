package keeper

import (
	"encoding/json"

	wasmTypes "github.com/enigmampc/SecretNetwork/go-cosmwasm/types"
	sdk "github.com/enigmampc/cosmos-sdk/types"
	sdkerrors "github.com/enigmampc/cosmos-sdk/types/errors"
	"github.com/enigmampc/cosmos-sdk/x/bank"
	"github.com/enigmampc/cosmos-sdk/x/staking"
)

type QueryHandler struct {
	Ctx     sdk.Context
	Plugins QueryPlugins
}

var _ wasmTypes.Querier = QueryHandler{}

func (q QueryHandler) Query(request wasmTypes.QueryRequest, gasLimit uint64) ([]byte, error) {
	// set a limit for a subctx
	sdkGas := gasLimit / GasMultiplier
	subctx := q.Ctx.WithGasMeter(sdk.NewGasMeter(sdkGas))

	// make sure we charge the higher level context even on panic
	defer func() {
		q.Ctx.GasMeter().ConsumeGas(subctx.GasMeter().GasConsumed(), "contract sub-query")
	}()

	// do the query
	if request.Bank != nil {
		return q.Plugins.Bank(subctx, request.Bank)
	}
	if request.Custom != nil {
		return q.Plugins.Custom(subctx, request.Custom)
	}
	if request.Staking != nil {
		return q.Plugins.Staking(subctx, request.Staking)
	}
	if request.Wasm != nil {
		return q.Plugins.Wasm(subctx, request.Wasm)
	}
	return nil, wasmTypes.Unknown{}
}

func (q QueryHandler) GasConsumed() uint64 {
	return q.Ctx.GasMeter().GasConsumed()
}

type CustomQuerier func(ctx sdk.Context, request json.RawMessage) ([]byte, error)

type QueryPlugins struct {
	Bank    func(ctx sdk.Context, request *wasmTypes.BankQuery) ([]byte, error)
	Custom  CustomQuerier
	Staking func(ctx sdk.Context, request *wasmTypes.StakingQuery) ([]byte, error)
	Wasm    func(ctx sdk.Context, request *wasmTypes.WasmQuery) ([]byte, error)
}

func DefaultQueryPlugins(bank bank.ViewKeeper, staking staking.Keeper, wasm *Keeper) QueryPlugins {
	return QueryPlugins{
		Bank:    BankQuerier(bank),
		Custom:  NoCustomQuerier,
		Staking: StakingQuerier(staking),
		Wasm:    WasmQuerier(wasm),
	}
}

func (e QueryPlugins) Merge(o *QueryPlugins) QueryPlugins {
	// only update if this is non-nil and then only set values
	if o == nil {
		return e
	}
	if o.Bank != nil {
		e.Bank = o.Bank
	}
	if o.Custom != nil {
		e.Custom = o.Custom
	}
	if o.Staking != nil {
		e.Staking = o.Staking
	}
	if o.Wasm != nil {
		e.Wasm = o.Wasm
	}
	return e
}

func BankQuerier(bank bank.ViewKeeper) func(ctx sdk.Context, request *wasmTypes.BankQuery) ([]byte, error) {
	return func(ctx sdk.Context, request *wasmTypes.BankQuery) ([]byte, error) {
		if request.AllBalances != nil {
			addr, err := sdk.AccAddressFromBech32(request.AllBalances.Address)
			if err != nil {
				return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, request.AllBalances.Address)
			}
			coins := bank.GetCoins(ctx, addr)
			res := wasmTypes.AllBalancesResponse{
				Amount: convertSdkCoinsToWasmCoins(coins),
			}
			return json.Marshal(res)
		}
		if request.Balance != nil {
			addr, err := sdk.AccAddressFromBech32(request.Balance.Address)
			if err != nil {
				return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, request.Balance.Address)
			}
			coins := bank.GetCoins(ctx, addr)
			amount := coins.AmountOf(request.Balance.Denom)
			res := wasmTypes.BalanceResponse{
				Amount: wasmTypes.Coin{
					Denom:  request.Balance.Denom,
					Amount: amount.String(),
				},
			}
			return json.Marshal(res)
		}
		return nil, wasmTypes.UnsupportedRequest{Kind: "unknown BankQuery variant"}
	}
}

func NoCustomQuerier(sdk.Context, json.RawMessage) ([]byte, error) {
	return nil, wasmTypes.UnsupportedRequest{Kind: "custom"}
}

func StakingQuerier(keeper staking.Keeper) func(ctx sdk.Context, request *wasmTypes.StakingQuery) ([]byte, error) {
	return func(ctx sdk.Context, request *wasmTypes.StakingQuery) ([]byte, error) {
		if request.BondedDenom != nil {
			denom := keeper.BondDenom(ctx)
			res := wasmTypes.BondedDenomResponse{
				Denom: denom,
			}
			return json.Marshal(res)
		}
		if request.Validators != nil {
			validators := keeper.GetBondedValidatorsByPower(ctx)
			//validators := keeper.GetAllValidators(ctx)
			wasmVals := make([]wasmTypes.Validator, len(validators))
			for i, v := range validators {
				wasmVals[i] = wasmTypes.Validator{
					Address:       v.OperatorAddress.String(),
					Commission:    v.Commission.Rate.String(),
					MaxCommission: v.Commission.MaxRate.String(),
					MaxChangeRate: v.Commission.MaxChangeRate.String(),
				}
			}
			res := wasmTypes.ValidatorsResponse{
				Validators: wasmVals,
			}
			return json.Marshal(res)
		}
		if request.AllDelegations != nil {
			delegator, err := sdk.AccAddressFromBech32(request.AllDelegations.Delegator)
			if err != nil {
				return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, request.AllDelegations.Delegator)
			}
			sdkDels := keeper.GetAllDelegatorDelegations(ctx, delegator)
			delegations, err := sdkToDelegations(ctx, keeper, sdkDels)
			if err != nil {
				return nil, err
			}
			res := wasmTypes.AllDelegationsResponse{
				Delegations: delegations,
			}
			return json.Marshal(res)
		}
		if request.Delegation != nil {
			delegator, err := sdk.AccAddressFromBech32(request.Delegation.Delegator)
			if err != nil {
				return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, request.Delegation.Delegator)
			}
			validator, err := sdk.ValAddressFromBech32(request.Delegation.Validator)
			if err != nil {
				return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, request.Delegation.Validator)
			}

			var res wasmTypes.DelegationResponse
			d, found := keeper.GetDelegation(ctx, delegator, validator)
			if found {
				res.Delegation, err = sdkToFullDelegation(ctx, keeper, d)
				if err != nil {
					return nil, err
				}
			}
			return json.Marshal(res)
		}
		return nil, wasmTypes.UnsupportedRequest{Kind: "unknown Staking variant"}
	}
}

func sdkToDelegations(ctx sdk.Context, keeper staking.Keeper, delegations []staking.Delegation) (wasmTypes.Delegations, error) {
	result := make([]wasmTypes.Delegation, len(delegations))
	bondDenom := keeper.BondDenom(ctx)

	for i, d := range delegations {
		// shares to amount logic comes from here:
		// https://github.com/enigmampc/cosmos-sdk/blob/v0.38.3/x/staking/keeper/querier.go#L404
		val, found := keeper.GetValidator(ctx, d.ValidatorAddress)
		if !found {
			return nil, sdkerrors.Wrap(staking.ErrNoValidatorFound, "can't load validator for delegation")
		}
		amount := sdk.NewCoin(bondDenom, val.TokensFromShares(d.Shares).TruncateInt())

		// Accumulated Rewards???

		// can relegate? other query for redelegations?
		// keeper.GetRedelegation

		result[i] = wasmTypes.Delegation{
			Delegator: d.DelegatorAddress.String(),
			Validator: d.ValidatorAddress.String(),
			Amount:    convertSdkCoinToWasmCoin(amount),
		}
	}
	return result, nil
}

func sdkToFullDelegation(ctx sdk.Context, keeper staking.Keeper, delegation staking.Delegation) (*wasmTypes.FullDelegation, error) {
	val, found := keeper.GetValidator(ctx, delegation.ValidatorAddress)
	if !found {
		return nil, sdkerrors.Wrap(staking.ErrNoValidatorFound, "can't load validator for delegation")
	}
	bondDenom := keeper.BondDenom(ctx)
	amount := sdk.NewCoin(bondDenom, val.TokensFromShares(delegation.Shares).TruncateInt())

	// can relegate? other query for redelegations?
	// keeper.GetRedelegation

	return &wasmTypes.FullDelegation{
		Delegator: delegation.DelegatorAddress.String(),
		Validator: delegation.ValidatorAddress.String(),
		Amount:    convertSdkCoinToWasmCoin(amount),
		// TODO: AccumulatedRewards
		AccumulatedRewards: wasmTypes.NewCoin(0, bondDenom),
		// TODO: Determine redelegate
		CanRedelegate: wasmTypes.NewCoin(0, bondDenom),
	}, nil
}

func WasmQuerier(wasm *Keeper) func(ctx sdk.Context, request *wasmTypes.WasmQuery) ([]byte, error) {
	return func(ctx sdk.Context, request *wasmTypes.WasmQuery) ([]byte, error) {
		if request.Smart != nil {
			addr, err := sdk.AccAddressFromBech32(request.Smart.ContractAddr)
			if err != nil {
				return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, request.Smart.ContractAddr)
			}
			return wasm.QuerySmart(ctx, addr, request.Smart.Msg, true)
		}
		if request.Raw != nil {
			addr, err := sdk.AccAddressFromBech32(request.Raw.ContractAddr)
			if err != nil {
				return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, request.Raw.ContractAddr)
			}
			models := wasm.QueryRaw(ctx, addr, request.Raw.Key)
			// TODO: do we want to change the return value?
			return json.Marshal(models)
		}
		return nil, wasmTypes.UnsupportedRequest{Kind: "unknown WasmQuery variant"}
	}
}

func convertSdkCoinsToWasmCoins(coins []sdk.Coin) wasmTypes.Coins {
	converted := make(wasmTypes.Coins, len(coins))
	for i, c := range coins {
		converted[i] = convertSdkCoinToWasmCoin(c)
	}
	return converted
}

func convertSdkCoinToWasmCoin(coin sdk.Coin) wasmTypes.Coin {
	return wasmTypes.Coin{
		Denom:  coin.Denom,
		Amount: coin.Amount.String(),
	}
}
