package keeper

import (
	"encoding/json"
	"fmt"

	channeltypes "github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"
	"github.com/scrtlabs/SecretNetwork/x/compute/internal/types"

	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	mintkeeper "github.com/cosmos/cosmos-sdk/x/mint/keeper"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	errorsmod "cosmossdk.io/errors"
	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	wasmTypes "github.com/scrtlabs/SecretNetwork/go-cosmwasm/types"
)

type GRPCQueryRouter interface {
	Route(path string) baseapp.GRPCQueryHandler
}

type QueryHandler struct {
	Ctx     sdk.Context
	Plugins QueryPlugins
	Caller  sdk.AccAddress
}

var _ wasmTypes.Querier = QueryHandler{}

func (q QueryHandler) Query(request wasmTypes.QueryRequest, queryDepth uint32, gasLimit uint64) ([]byte, error) {
	// set a limit for a subctx
	sdkGas := gasLimit / types.GasMultiplier
	subctx := q.Ctx.WithGasMeter(storetypes.NewGasMeter(sdkGas))

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
		return q.Plugins.Wasm(subctx, request.Wasm, queryDepth)
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
	if request.IBC != nil {
		return q.Plugins.IBC(q.Ctx, q.Caller, request.IBC)
	}
	if request.Stargate != nil {
		return q.Plugins.Stargate(q.Ctx, request.Stargate)
	}
	return nil, wasmTypes.Unknown{}
}

func (q QueryHandler) GasConsumed() uint64 {
	return q.Ctx.GasMeter().GasConsumed()
}

type CustomQuerier func(ctx sdk.Context, request json.RawMessage) ([]byte, error)

type QueryPlugins struct {
	Bank     func(ctx sdk.Context, request *wasmTypes.BankQuery) ([]byte, error)
	Custom   CustomQuerier
	Staking  func(ctx sdk.Context, request *wasmTypes.StakingQuery) ([]byte, error)
	Wasm     func(ctx sdk.Context, request *wasmTypes.WasmQuery, queryDepth uint32) ([]byte, error)
	Dist     func(ctx sdk.Context, request *wasmTypes.DistQuery) ([]byte, error)
	Mint     func(ctx sdk.Context, request *wasmTypes.MintQuery) ([]byte, error)
	Gov      func(ctx sdk.Context, request *wasmTypes.GovQuery) ([]byte, error)
	IBC      func(ctx sdk.Context, caller sdk.AccAddress, request *wasmTypes.IBCQuery) ([]byte, error)
	Stargate func(ctx sdk.Context, request *wasmTypes.StargateQuery) ([]byte, error)
}

func DefaultQueryPlugins(gov govkeeper.Keeper, dist distrkeeper.Keeper, mint mintkeeper.Keeper, bank bankkeeper.Keeper, staking stakingkeeper.Keeper, stargateQueryRouter GRPCQueryRouter, wasm *Keeper, channelKeeper types.ChannelKeeper) QueryPlugins {
	return QueryPlugins{
		Bank:     BankQuerier(bank),
		Custom:   NoCustomQuerier,
		Staking:  StakingQuerier(staking, dist),
		Wasm:     WasmQuerier(wasm),
		Dist:     DistQuerier(dist),
		Mint:     MintQuerier(mint),
		Gov:      GovQuerier(gov),
		Stargate: StargateQuerier(stargateQueryRouter),
		IBC:      IBCQuerier(wasm, channelKeeper),
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
	if o.IBC != nil {
		e.IBC = o.IBC
	}
	if o.Stargate != nil {
		e.Stargate = o.Stargate
	}
	return e
}

// Assaf: stargateQueryAllowlist is a list of all safe and efficient queries
//
// excluded from this list (should be safe, but needs a clear use case):
//   - /secret.registration.*
//   - /ibc.core.*
//   - /secret.intertx.*
//   - /cosmos.evidence.*
//   - /cosmos.upgrade.*
//   - All "get all" queries - only O(1) queries should be served
//
// we reserve the right to add/remove queries in future chain upgrades
//
// used this to find all query paths:
// find -name query.proto | sort | xargs grep -Poin 'package [a-z0-9.]+;|rpc [a-zA-Z]+\('
var stargateQueryAllowlist = map[string]bool{
	"/cosmos.auth.v1beta1.Query/Account": true,
	"/cosmos.auth.v1beta1.Query/Params":  true,

	"/cosmos.bank.v1beta1.Query/Balance":       true,
	"/cosmos.bank.v1beta1.Query/DenomMetadata": true,
	"/cosmos.bank.v1beta1.Query/SupplyOf":      true,
	"/cosmos.bank.v1beta1.Query/Params":        true,

	"/cosmos.distribution.v1beta1.Query/Params":                   true,
	"/cosmos.distribution.v1beta1.Query/DelegatorWithdrawAddress": true,
	"/cosmos.distribution.v1beta1.Query/FoundationTax":            true,
	"/cosmos.distribution.v1beta1.Query/ValidatorCommission":      true,

	"/cosmos.feegrant.v1beta1.Query/Allowance": true,

	"/cosmos.gov.v1beta1.Query/Deposit":  true,
	"/cosmos.gov.v1beta1.Query/Params":   true,
	"/cosmos.gov.v1beta1.Query/Proposal": true,
	"/cosmos.gov.v1beta1.Query/Vote":     true,

	"/cosmos.mint.v1beta1.Query/Params":           true,
	"/cosmos.mint.v1beta1.Query/Inflation":        true,
	"/cosmos.mint.v1beta1.Query/AnnualProvisions": true,

	"/cosmos.params.v1beta1.Query/Params": true,

	"/cosmos.slashing.v1beta1.Query/Params":      true,
	"/cosmos.slashing.v1beta1.Query/SigningInfo": true,

	"/cosmos.staking.v1beta1.Query/Validator":           true,
	"/cosmos.staking.v1beta1.Query/Delegation":          true,
	"/cosmos.staking.v1beta1.Query/UnbondingDelegation": true,
	"/cosmos.staking.v1beta1.Query/Params":              true,

	"/ibc.applications.transfer.v1.Query/DenomHash":  true,
	"/ibc.applications.transfer.v1.Query/DenomTrace": true,
	"/ibc.applications.transfer.v1.Query/Params":     true,

	"/secret.compute.v1beta1.Query/ContractInfo":              true,
	"/secret.compute.v1beta1.Query/CodeHashByContractAddress": true,
	"/secret.compute.v1beta1.Query/CodeHashByCodeId":          true,
	"/secret.compute.v1beta1.Query/LabelByAddress":            true,
	"/secret.compute.v1beta1.Query/AddressByLabel":            true,
}

func StargateQuerier(queryRouter GRPCQueryRouter) func(ctx sdk.Context, request *wasmTypes.StargateQuery) ([]byte, error) {
	return func(ctx sdk.Context, msg *wasmTypes.StargateQuery) ([]byte, error) {
		if !stargateQueryAllowlist[msg.Path] {
			return nil, wasmTypes.UnsupportedRequest{Kind: fmt.Sprintf("query path '%s' is not allowed from the contract", msg.Path)}
		}

		route := queryRouter.Route(msg.Path)
		if route == nil {
			return nil, wasmTypes.UnsupportedRequest{Kind: fmt.Sprintf("No route to query path '%s'", msg.Path)}
		}
		req := abci.RequestQuery{
			Data: msg.Data,
			Path: msg.Path,
		}
		res, err := route(ctx, &req)
		if err != nil {
			return nil, err
		}
		return res.Value, nil
	}
}

func GovQuerier(keeper govkeeper.Keeper) func(ctx sdk.Context, request *wasmTypes.GovQuery) ([]byte, error) {
	return func(ctx sdk.Context, request *wasmTypes.GovQuery) ([]byte, error) {
		if request.Proposals != nil {
			var proposals govtypes.Proposals
			keeper.Proposals.Walk(ctx, nil, func(_ uint64, value govtypes.Proposal) (stop bool, err error) {
				proposals = append(proposals, &value)
				return false, nil
			})

			if len(proposals) == 0 {
				return json.Marshal(wasmTypes.ProposalsResponse{
					Proposals: []wasmTypes.Proposal{},
				})
			}

			var activeProps []wasmTypes.Proposal
			for _, val := range proposals {
				if val.Status == govtypes.StatusVotingPeriod {
					activeProps = append(activeProps, wasmTypes.Proposal{
						ProposalID:      val.Id,
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

func IBCQuerier(wasm *Keeper, channelKeeper types.ChannelKeeper) func(ctx sdk.Context, caller sdk.AccAddress, request *wasmTypes.IBCQuery) ([]byte, error) {
	return func(ctx sdk.Context, caller sdk.AccAddress, request *wasmTypes.IBCQuery) ([]byte, error) {
		if request.PortID != nil {
			contractInfo := wasm.GetContractInfo(ctx, caller)
			res := wasmTypes.PortIDResponse{
				PortID: contractInfo.IBCPortID,
			}
			return json.Marshal(res)
		}
		if request.ListChannels != nil {
			portID := request.ListChannels.PortID
			channels := make(wasmTypes.IBCChannels, 0)
			channelKeeper.IterateChannels(ctx, func(ch channeltypes.IdentifiedChannel) bool {
				// it must match the port and be in open state
				if (portID == "" || portID == ch.PortId) && ch.State == channeltypes.OPEN {
					newChan := wasmTypes.IBCChannel{
						Endpoint: wasmTypes.IBCEndpoint{
							PortID:    ch.PortId,
							ChannelID: ch.ChannelId,
						},
						CounterpartyEndpoint: wasmTypes.IBCEndpoint{
							PortID:    ch.Counterparty.PortId,
							ChannelID: ch.Counterparty.ChannelId,
						},
						Order:        ch.Ordering.String(),
						Version:      ch.Version,
						ConnectionID: ch.ConnectionHops[0],
					}
					channels = append(channels, newChan)
				}
				return false
			})
			res := wasmTypes.ListChannelsResponse{
				Channels: channels,
			}
			return json.Marshal(res)
		}
		if request.Channel != nil {
			channelID := request.Channel.ChannelID
			portID := request.Channel.PortID
			if portID == "" {
				contractInfo := wasm.GetContractInfo(ctx, caller)
				portID = contractInfo.IBCPortID
			}
			got, found := channelKeeper.GetChannel(ctx, portID, channelID)
			var channel *wasmTypes.IBCChannel
			// it must be in open state
			if found && got.State == channeltypes.OPEN {
				channel = &wasmTypes.IBCChannel{
					Endpoint: wasmTypes.IBCEndpoint{
						PortID:    portID,
						ChannelID: channelID,
					},
					CounterpartyEndpoint: wasmTypes.IBCEndpoint{
						PortID:    got.Counterparty.PortId,
						ChannelID: got.Counterparty.ChannelId,
					},
					Order:        got.Ordering.String(),
					Version:      got.Version,
					ConnectionID: got.ConnectionHops[0],
				}
			}
			res := wasmTypes.ChannelResponse{
				Channel: channel,
			}
			return json.Marshal(res)
		}
		return nil, wasmTypes.UnsupportedRequest{Kind: "unknown IBCQuery variant"}
	}
}

func MintQuerier(keeper mintkeeper.Keeper) func(ctx sdk.Context, request *wasmTypes.MintQuery) ([]byte, error) {
	return func(ctx sdk.Context, request *wasmTypes.MintQuery) ([]byte, error) {
		if request.BondedRatio != nil {
			total, _ := keeper.BondedRatio(ctx)

			resp := wasmTypes.MintingBondedRatioResponse{
				BondedRatio: total.String(),
			}

			return json.Marshal(resp)
		}
		if request.Inflation != nil {
			minter, _ := keeper.Minter.Get(ctx)
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
		// TODO: rewrite the function
		/*
			if request.Rewards != nil {
				addr, err := sdk.AccAddressFromBech32(request.Rewards.Delegator)
				if err != nil {
					return nil, sdkerrors.ErrInvalidAddress.Wrap(request.Rewards.Delegator)
				}

				params := distrtypes.NewQueryDelegatorParams(addr)

				jsonParams, _ := json.Marshal(params)

				req := abci.RequestQuery{
					Data: jsonParams,
				}
				// keeper.DelegationTotalRewards(ctx, distrtypes.QueryDelegationTotalRewardsRequest{
				//	DelegatorAddress: request.Rewards.Delegator,
				// })
				route := []string{distrtypes.QueryDelegatorTotalRewards}

				query, err := distrkeeper.NewQuerier(keeper, codec.NewLegacyAmino())(ctx, route, req)
				if err != nil {
					return nil, sdkerrors.ErrUnknownRequest.Wrap(err.Error())
				}

				var res wasmTypes.RewardsResponse

				err = json.Unmarshal(query, &res)
				if err != nil {
					return nil, sdkerrors.ErrJSONMarshal.Wrap(err.Error())
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
					return nil, sdkerrors.ErrJSONMarshal.Wrap(err.Error())
				}

				return ret, nil
			}*/
		return nil, wasmTypes.UnsupportedRequest{Kind: "unknown DistQuery variant"}
	}
}

func BankQuerier(bankKeeper bankkeeper.ViewKeeper) func(ctx sdk.Context, request *wasmTypes.BankQuery) ([]byte, error) {
	return func(ctx sdk.Context, request *wasmTypes.BankQuery) ([]byte, error) {
		if request.AllBalances != nil {
			addr, err := sdk.AccAddressFromBech32(request.AllBalances.Address)
			if err != nil {
				return nil, sdkerrors.ErrInvalidAddress.Wrap(request.AllBalances.Address)
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
				return nil, sdkerrors.ErrInvalidAddress.Wrap(request.Balance.Address)
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
			denom, _ := keeper.BondDenom(ctx)
			res := wasmTypes.BondedDenomResponse{
				Denom: denom,
			}
			return json.Marshal(res)
		}
		if request.Validators != nil {
			validators, _ := keeper.GetBondedValidatorsByPower(ctx)
			// validators := keeper.GetAllValidators(ctx)
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
				return nil, sdkerrors.ErrInvalidAddress.Wrap(request.AllDelegations.Delegator)
			}
			sdkDels, _ := keeper.GetAllDelegatorDelegations(ctx, delegator)
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
				return nil, sdkerrors.ErrInvalidAddress.Wrap(request.Delegation.Delegator)
			}
			validator, err := sdk.ValAddressFromBech32(request.Delegation.Validator)
			if err != nil {
				return nil, sdkerrors.ErrInvalidAddress.Wrap(request.Delegation.Validator)
			}

			var res wasmTypes.DelegationResponse
			d, err := keeper.GetDelegation(ctx, delegator, validator)
			if err == nil {
				res.Delegation, err = sdkToFullDelegation(ctx, keeper, distKeeper, d)
				if err != nil {
					return nil, err
				}
			}
			return json.Marshal(res)
		}
		if request.UnBondingDelegations != nil {
			bondDenom, _ := keeper.BondDenom(ctx)

			delegator, err := sdk.AccAddressFromBech32(request.UnBondingDelegations.Delegator)
			if err != nil {
				return nil, sdkerrors.ErrInvalidAddress.Wrap(request.Delegation.Delegator)
			}

			unbondingDelegations, _ := keeper.GetAllUnbondingDelegations(ctx, delegator)
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
		if request.Validator != nil {
			valAddr, err := sdk.ValAddressFromBech32(request.Validator.Address)
			if err != nil {
				return nil, err
			}
			v, err := keeper.GetValidator(ctx, valAddr)
			res := wasmTypes.ValidatorResponse{}
			if err == nil {
				res.Validator = &wasmTypes.Validator{
					Address:       v.OperatorAddress,
					Commission:    v.Commission.Rate.String(),
					MaxCommission: v.Commission.MaxRate.String(),
					MaxChangeRate: v.Commission.MaxChangeRate.String(),
				}
			}
			return json.Marshal(res)
		}
		if request.AllValidators != nil {
			validators, _ := keeper.GetBondedValidatorsByPower(ctx)
			// validators := keeper.GetAllValidators(ctx)
			wasmVals := make([]wasmTypes.Validator, len(validators))
			for i, v := range validators {
				wasmVals[i] = wasmTypes.Validator{
					Address:       v.OperatorAddress,
					Commission:    v.Commission.Rate.String(),
					MaxCommission: v.Commission.MaxRate.String(),
					MaxChangeRate: v.Commission.MaxChangeRate.String(),
				}
			}
			res := wasmTypes.AllValidatorsResponse{
				Validators: wasmVals,
			}
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
	bondDenom, _ := keeper.BondDenom(ctx)

	for i, d := range delegations {
		delAddr, err := sdk.AccAddressFromBech32(d.DelegatorAddress)
		if err != nil {
			return nil, errorsmod.Wrap(err, "delegator address")
		}
		valAddr, err := sdk.ValAddressFromBech32(d.ValidatorAddress)
		if err != nil {
			return nil, errorsmod.Wrap(err, "validator address")
		}

		// shares to amount logic comes from here:
		// https://github.com/cosmos/cosmos-sdk/blob/v0.38.3/x/staking/keeper/querier.go#L404
		val, err := keeper.GetValidator(ctx, valAddr)
		if err != nil {
			return nil, errorsmod.Wrap(stakingtypes.ErrNoValidatorFound, "can't load validator for delegation")
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
		return nil, errorsmod.Wrap(err, "delegator address")
	}
	valAddr, err := sdk.ValAddressFromBech32(delegation.ValidatorAddress)
	if err != nil {
		return nil, errorsmod.Wrap(err, "validator address")
	}
	val, err := keeper.GetValidator(ctx, valAddr)
	if err != nil {
		return nil, errorsmod.Wrap(stakingtypes.ErrNoValidatorFound, "can't load validator for delegation")
	}
	bondDenom, _ := keeper.BondDenom(ctx)
	amount := sdk.NewCoin(bondDenom, val.TokensFromShares(delegation.Shares).TruncateInt())

	delegationCoins := convertSdkCoinToWasmCoin(amount)

	// FIXME: this is very rough but better than nothing...
	// https://github.com/CosmWasm/wasmd/issues/282
	// if this (val, delegate) pair is receiving a redelegation, it cannot redelegate more
	// otherwise, it can redelegate the full amount
	// (there are cases of partial funds redelegated, but this is a start)
	redelegateCoins := wasmTypes.NewCoin(0, bondDenom)
	if has, _ := keeper.HasReceivingRedelegation(ctx, delAddr, valAddr); !has {
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
	/*
		// Try to get *delegator* reward info!

		params := distrtypes.QueryDelegationRewardsRequest{
			DelegatorAddress: delegation.DelegatorAddress,
			ValidatorAddress: delegation.ValidatorAddress,
		}
		cache, _ := ctx.CacheContext()
		// TODO: rewrite the function
		qres, err := distKeeper.Querier.DelegationRewards(sdk.WrapSDKContext(cache), &params)
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
		return rewards, nil*/
	return nil, nil
}

func WasmQuerier(wasm *Keeper) func(ctx sdk.Context, request *wasmTypes.WasmQuery, queryDepth uint32) ([]byte, error) {
	return func(ctx sdk.Context, request *wasmTypes.WasmQuery, queryDepth uint32) ([]byte, error) {
		if request.Smart != nil {
			addr, err := sdk.AccAddressFromBech32(request.Smart.ContractAddr)
			if err != nil {
				return nil, sdkerrors.ErrInvalidAddress.Wrap(request.Smart.ContractAddr)
			}
			return wasm.querySmartRecursive(ctx, addr, request.Smart.Msg, queryDepth, false)
		}
		if request.Raw != nil {
			addr, err := sdk.AccAddressFromBech32(request.Raw.ContractAddr)
			if err != nil {
				return nil, sdkerrors.ErrInvalidAddress.Wrap(request.Raw.ContractAddr)
			}
			models := wasm.QueryRaw(ctx, addr, request.Raw.Key)
			// TODO: do we want to change the return value?
			return json.Marshal(models)
		}
		if request.ContractInfo != nil {
			addr, err := sdk.AccAddressFromBech32(request.ContractInfo.ContractAddr)
			if err != nil {
				return nil, sdkerrors.ErrInvalidAddress.Wrap(request.ContractInfo.ContractAddr)
			}
			info := wasm.GetContractInfo(ctx, addr)
			if info == nil {
				return nil, sdkerrors.ErrInvalidAddress.Wrap(request.ContractInfo.ContractAddr)
			}

			res := wasmTypes.ContractInfoResponse{
				CodeID:  info.CodeID,
				Creator: info.Creator.String(),
				Admin:   "", // In secret we don't have an admin
				Pinned:  false,
				IBCPort: info.IBCPortID,
			}
			return json.Marshal(res)
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
