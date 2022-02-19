package keeper

import (
	"encoding/json"
	"github.com/enigmampc/SecretNetwork/x/compute/internal/types"
	"strings"

	"github.com/cosmos/cosmos-sdk/codec"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	mintkeeper "github.com/cosmos/cosmos-sdk/x/mint/keeper"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	wasmTypes "github.com/enigmampc/SecretNetwork/go-cosmwasm/types"
	abci "github.com/tendermint/tendermint/abci/types"
)

type QueryHandler struct {
	Ctx     sdk.Context
	Plugins QueryPlugins
}

var _ wasmTypes.Querier = QueryHandler{}

func (q QueryHandler) Query(request wasmTypes.QueryRequest, gasLimit uint64) ([]byte, error) {
	// set a limit for a subctx
	sdkGas := gasLimit / types.GasMultiplier
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
	if request.Dist != nil {
		return q.Plugins.Dist(q.Ctx, request.Dist)
	}
	if request.Mint != nil {
		return q.Plugins.Mint(q.Ctx, request.Mint)
	}
	if request.Gov != nil {
		return q.Plugins.Gov(q.Ctx, request.Gov)
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
	Dist    func(ctx sdk.Context, request *wasmTypes.DistQuery) ([]byte, error)
	Mint    func(ctx sdk.Context, request *wasmTypes.MintQuery) ([]byte, error)
	Gov     func(ctx sdk.Context, request *wasmTypes.GovQuery) ([]byte, error)
}

func DefaultQueryPlugins(gov govkeeper.Keeper, dist distrkeeper.Keeper, mint mintkeeper.Keeper, bank bankkeeper.Keeper, staking stakingkeeper.Keeper, wasm *Keeper) QueryPlugins {
	return QueryPlugins{
		Bank:    BankQuerier(bank),
		Custom:  NoCustomQuerier,
		Staking: StakingQuerier(staking, dist),
		Wasm:    WasmQuerier(wasm),
		Dist:    DistQuerier(dist),
		Mint:    MintQuerier(mint),
		Gov:     GovQuerier(gov),
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
	if o.Dist != nil {
		e.Dist = o.Dist
	}
	if o.Mint != nil {
		e.Mint = o.Mint
	}
	if o.Gov != nil {
		e.Gov = o.Gov
	}
	return e
}

func GovQuerier(keeper govkeeper.Keeper) func(ctx sdk.Context, request *wasmTypes.GovQuery) ([]byte, error) {
	return func(ctx sdk.Context, request *wasmTypes.GovQuery) ([]byte, error) {
		if request.Proposals != nil {
			proposals := keeper.GetProposals(ctx)

			if len(proposals) == 0 {
				return json.Marshal(wasmTypes.ProposalsResponse{
					Proposals: []wasmTypes.Proposal{},
				})
			}

			var activeProps []wasmTypes.Proposal
			for _, val := range proposals {
				if val.Status == govtypes.StatusVotingPeriod {
					activeProps = append(activeProps, wasmTypes.Proposal{
						ProposalID:      val.ProposalId,
						VotingStartTime: uint64(val.VotingStartTime.Unix()),
						VotingEndTime:   uint64(val.VotingEndTime.Unix()),
					})
				}
			}

			return json.Marshal(wasmTypes.ProposalsResponse{Proposals: activeProps})
		}
		return nil, wasmTypes.UnsupportedRequest{Kind: "unknown GovQuery variant"}
	}
}

func MintQuerier(keeper mintkeeper.Keeper) func(ctx sdk.Context, request *wasmTypes.MintQuery) ([]byte, error) {
	return func(ctx sdk.Context, request *wasmTypes.MintQuery) ([]byte, error) {
		if request.BondedRatio != nil {
			total := keeper.BondedRatio(ctx)

			resp := wasmTypes.MintingBondedRatioResponse{
				BondedRatio: total.String(),
			}

			return json.Marshal(resp)
		}
		if request.Inflation != nil {
			minter := keeper.GetMinter(ctx)
			inflation := minter.Inflation

			resp := wasmTypes.MintingInflationResponse{
				InflationRate: inflation.String(),
			}

			return json.Marshal(resp)
		}
		return nil, wasmTypes.UnsupportedRequest{Kind: "unknown MintQuery variant"}
	}

}

func DistQuerier(keeper distrkeeper.Keeper) func(ctx sdk.Context, request *wasmTypes.DistQuery) ([]byte, error) {
	return func(ctx sdk.Context, request *wasmTypes.DistQuery) ([]byte, error) {
		if request.Rewards != nil {
			addr, err := sdk.AccAddressFromBech32(request.Rewards.Delegator)
			if err != nil {
				return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, request.Rewards.Delegator)
			}

			params := distrtypes.NewQueryDelegatorParams(addr)

			jsonParams, _ := json.Marshal(params)

			req := abci.RequestQuery{
				Data: jsonParams,
			}
			//keeper.DelegationTotalRewards(ctx, distrtypes.QueryDelegationTotalRewardsRequest{
			//	DelegatorAddress: request.Rewards.Delegator,
			//})
			route := []string{distrtypes.QueryDelegatorTotalRewards}

			query, err := distrkeeper.NewQuerier(keeper, codec.NewLegacyAmino() /* TODO: Is there a way to get an existing Amino codec? */)(ctx, route, req)
			if err != nil {
				return nil, sdkerrors.Wrap(sdkerrors.ErrUnknownRequest, err.Error())
			}

			var res wasmTypes.RewardsResponse

			err = json.Unmarshal(query, &res)
			if err != nil {
				return nil, sdkerrors.Wrap(sdkerrors.ErrJSONMarshal, err.Error())
			}

			for i, valRewards := range res.Rewards {
				res.Rewards[i].Validator = valRewards.Validator
				for j, valReward := range valRewards.Reward {
					// this is here so we can remove fractions of uscrt from the result
					res.Rewards[i].Reward[j].Amount = strings.Split(valReward.Amount, ".")[0]
					res.Rewards[i].Reward[j].Denom = valReward.Denom
				}
			}

			for i, val := range res.Total {
				res.Total[i].Amount = strings.Split(val.Amount, ".")[0]
				res.Total[i].Denom = val.Denom
			}

			ret, err := json.Marshal(res)
			if err != nil {
				return nil, sdkerrors.Wrap(sdkerrors.ErrJSONMarshal, err.Error())
			}

			return ret, nil
		}
		return nil, wasmTypes.UnsupportedRequest{Kind: "unknown DistQuery variant"}
	}
}

func BankQuerier(bankKeeper bankkeeper.ViewKeeper) func(ctx sdk.Context, request *wasmTypes.BankQuery) ([]byte, error) {
	return func(ctx sdk.Context, request *wasmTypes.BankQuery) ([]byte, error) {
		if request.AllBalances != nil {
			addr, err := sdk.AccAddressFromBech32(request.AllBalances.Address)
			if err != nil {
				return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, request.AllBalances.Address)
			}
			coins := bankKeeper.GetAllBalances(ctx, addr)
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
			coins := bankKeeper.GetAllBalances(ctx, addr)
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

func StakingQuerier(keeper stakingkeeper.Keeper, distKeeper distrkeeper.Keeper) func(ctx sdk.Context, request *wasmTypes.StakingQuery) ([]byte, error) {
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
					Address:       v.OperatorAddress,
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
				res.Delegation, err = sdkToFullDelegation(ctx, keeper, distKeeper, d)
				if err != nil {
					return nil, err
				}
			}
			return json.Marshal(res)
		}
		if request.UnBondingDelegations != nil {
			bondDenom := keeper.BondDenom(ctx)

			delegator, err := sdk.AccAddressFromBech32(request.UnBondingDelegations.Delegator)
			if err != nil {
				return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, request.Delegation.Delegator)
			}

			unbondingDelegations := keeper.GetAllUnbondingDelegations(ctx, delegator)
			if unbondingDelegations == nil {
				unbondingDelegations = stakingtypes.UnbondingDelegations{}
			}

			delegations, err := sdkToUnbondingDelegations(bondDenom, unbondingDelegations)
			if err != nil {
				return nil, err
			}

			var res wasmTypes.UnbondingDelegationsResponse
			res.Delegations = delegations

			return json.Marshal(res)

		}
		return nil, wasmTypes.UnsupportedRequest{Kind: "unknown Staking variant"}
	}
}

func sdkToUnbondingDelegations(bondDenom string, delegations stakingtypes.UnbondingDelegations) ([]wasmTypes.Delegation, error) {
	result := make([]wasmTypes.Delegation, len(delegations))

	for i, d := range delegations {

		for _, e := range d.Entries {

			wasmCoin := wasmTypes.Coin{
				Denom:  bondDenom,
				Amount: e.Balance.String(),
			}

			result[i] = wasmTypes.Delegation{
				Delegator: d.DelegatorAddress,
				Validator: d.ValidatorAddress,
				Amount:    wasmCoin,
			}

		}

	}
	return result, nil
}

func sdkToDelegations(ctx sdk.Context, keeper stakingkeeper.Keeper, delegations []stakingtypes.Delegation) (wasmTypes.Delegations, error) {
	result := make([]wasmTypes.Delegation, len(delegations))
	bondDenom := keeper.BondDenom(ctx)

	for i, d := range delegations {
		delAddr, err := sdk.AccAddressFromBech32(d.DelegatorAddress)
		if err != nil {
			return nil, sdkerrors.Wrap(err, "delegator address")
		}
		valAddr, err := sdk.ValAddressFromBech32(d.ValidatorAddress)
		if err != nil {
			return nil, sdkerrors.Wrap(err, "validator address")
		}

		// shares to amount logic comes from here:
		// https://github.com/cosmos/cosmos-sdk/blob/v0.38.3/x/staking/keeper/querier.go#L404
		val, found := keeper.GetValidator(ctx, valAddr)
		if !found {
			return nil, sdkerrors.Wrap(stakingtypes.ErrNoValidatorFound, "can't load validator for delegation")
		}
		amount := sdk.NewCoin(bondDenom, val.TokensFromShares(d.Shares).TruncateInt())

		// Accumulated Rewards???

		// can relegate? other query for redelegations?
		// keeper.GetRedelegation

		result[i] = wasmTypes.Delegation{
			Delegator: delAddr.String(),
			Validator: valAddr.String(),
			Amount:    convertSdkCoinToWasmCoin(amount),
		}
	}
	return result, nil
}

func sdkToFullDelegation(ctx sdk.Context, keeper stakingkeeper.Keeper, distKeeper distrkeeper.Keeper, delegation stakingtypes.Delegation) (*wasmTypes.FullDelegation, error) {
	delAddr, err := sdk.AccAddressFromBech32(delegation.DelegatorAddress)
	if err != nil {
		return nil, sdkerrors.Wrap(err, "delegator address")
	}
	valAddr, err := sdk.ValAddressFromBech32(delegation.ValidatorAddress)
	if err != nil {
		return nil, sdkerrors.Wrap(err, "validator address")
	}
	val, found := keeper.GetValidator(ctx, valAddr)
	if !found {
		return nil, sdkerrors.Wrap(stakingtypes.ErrNoValidatorFound, "can't load validator for delegation")
	}
	bondDenom := keeper.BondDenom(ctx)
	amount := sdk.NewCoin(bondDenom, val.TokensFromShares(delegation.Shares).TruncateInt())

	delegationCoins := convertSdkCoinToWasmCoin(amount)

	// FIXME: this is very rough but better than nothing...
	// https://github.com/CosmWasm/wasmd/issues/282
	// if this (val, delegate) pair is receiving a redelegation, it cannot redelegate more
	// otherwise, it can redelegate the full amount
	// (there are cases of partial funds redelegated, but this is a start)
	redelegateCoins := wasmTypes.NewCoin(0, bondDenom)
	if !keeper.HasReceivingRedelegation(ctx, delAddr, valAddr) {
		redelegateCoins = delegationCoins
	}

	// FIXME: make a cleaner way to do this (modify the sdk)
	// we need the info from `distKeeper.calculateDelegationRewards()`, but it is not public
	// neither is `queryDelegationRewards(ctx sdk.Context, _ []string, req abci.RequestQuery, k Keeper)`
	// so we go through the front door of the querier....
	accRewards, err := getAccumulatedRewards(ctx, distKeeper, delegation)
	if err != nil {
		return nil, err
	}

	return &wasmTypes.FullDelegation{
		Delegator:          delAddr.String(),
		Validator:          valAddr.String(),
		Amount:             delegationCoins,
		AccumulatedRewards: accRewards,
		CanRedelegate:      redelegateCoins,
	}, nil
}

// FIXME: simplify this enormously when
// https://github.com/cosmos/cosmos-sdk/issues/7466 is merged
func getAccumulatedRewards(ctx sdk.Context, distKeeper distrkeeper.Keeper, delegation stakingtypes.Delegation) ([]wasmTypes.Coin, error) {
	// Try to get *delegator* reward info!
	params := distrtypes.QueryDelegationRewardsRequest{
		DelegatorAddress: delegation.DelegatorAddress,
		ValidatorAddress: delegation.ValidatorAddress,
	}
	cache, _ := ctx.CacheContext()
	qres, err := distKeeper.DelegationRewards(sdk.WrapSDKContext(cache), &params)
	if err != nil {
		return nil, err
	}

	// now we have it, convert it into wasmTypes
	rewards := make([]wasmTypes.Coin, len(qres.Rewards))
	for i, r := range qres.Rewards {
		rewards[i] = wasmTypes.Coin{
			Denom:  r.Denom,
			Amount: r.Amount.TruncateInt().String(),
		}
	}
	return rewards, nil
}

func WasmQuerier(wasm *Keeper) func(ctx sdk.Context, request *wasmTypes.WasmQuery) ([]byte, error) {
	return func(ctx sdk.Context, request *wasmTypes.WasmQuery) ([]byte, error) {
		if request.Smart != nil {
			addr, err := sdk.AccAddressFromBech32(request.Smart.ContractAddr)
			if err != nil {
				return nil, sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, request.Smart.ContractAddr)
			}
			return wasm.querySmartRecursive(ctx, addr, request.Smart.Msg, true)
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
