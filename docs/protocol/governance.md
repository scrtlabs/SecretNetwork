# Governance

Governance is the process from which users in the Secret Blockchain can come to consensus on software upgrades, parameters of the mainnet or signaling mechanisms through text proposals. This is done through voting on proposals, which will be submitted by `SCRT` holders on the mainnet.

Some considerations about the voting process:

- Voting is done by bonded `SCRT` holders on a 1 bonded `SCRT` 1 vote basis.
- Delegators inherit the vote of their validator if they don't vote.
- Votes are tallied at the end of the voting period (1 week on mainnet) where each address can vote multiple times to update its `Option` value (paying the transaction fee each time), only the most recently cast vote will count as valid.
- Voters can choose between options `Yes`, `No`, `NoWithVeto` and `Abstain`.
- At the end of the voting period, a proposal is accepted IFF:
  - `(YesVotes / (YesVotes+NoVotes+NoWithVetoVotes)) > 1/2` ([threshold](https://github.com/enigmampc/SecretNetwork/blob/b0792cc7f63a9264afe5de252a5821788c21834d/enigma-1-genesis.json#L1864))
  - `(NoWithVetoVotes / (YesVotes+NoVotes+NoWithVetoVotes)) < 1/3` ([veto](https://github.com/enigmampc/SecretNetwork/blob/b0792cc7f63a9264afe5de252a5821788c21834d/enigma-1-genesis.json#L1865))
  - `((YesVotes+NoVotes+NoWithVetoVotes) / totalBondedStake) >= 1/3` ([quorum](https://github.com/enigmampc/SecretNetwork/blob/b0792cc7f63a9264afe5de252a5821788c21834d/enigma-1-genesis.json#L1863))

For more information about the governance process and how it works, please check out the Governance module [specification](https://github.com/cosmos/cosmos-sdk/tree/master/x/gov/spec).

## Setup

- [How to use a light client (Windows, Mac & Linux)](../light-client-mainnet.md)
- [Ledger Nano S support](../ledger-nano-s.md)

## Create a Governance Proposal

In order to create a governance proposal, you must submit an initial deposit along with a title and description. Currently, in order to enter the voting period, a proposal must accumulate within a week deposits of at least [1000 `SCRT`](https://github.com/enigmampc/SecretNetwork/blob/b0792cc7f63a9264afe5de252a5821788c21834d/enigma-1-genesis.json#L1851-L1856).

Various modules outside of governance may implement their own proposal types and handlers (eg. parameter changes), where the governance module itself supports `Text` proposals. Any module outside of governance has it's command mounted on top of `submit-proposal`.

### Text

To submit a `Text` proposal:

```bash
secretcli tx gov submit-proposal \
  --title <title> \
  --description <description> \
  --type Text \
  --deposit 1000000uscrt \
  --from <key_alias>
```

You may also provide the proposal directly through the `--proposal` flag which points to a JSON file containing the proposal:

```bash
secretcli tx gov submit-proposal --proposal <path/to/proposal.json> --from <key_alias>
```

Where `proposal.json` is:

```json
{
  "type": "Text",
  "title": "My Cool Proposal",
  "description": "A description with line breaks \n and `code formatting`",
  "deposit": "1000000uscrt"
}
```

### Param Change

To submit a parameter change proposal, you must provide a proposal file as its contents are less friendly to CLI input:

```bash
secretcli tx gov submit-proposal param-change <path/to/proposal.json> --from <key_alias>
```

Where `proposal.json` is:

```json
{
  "title": "Param Change",
  "description": "Update max validators with line breaks \n and `code formatting`",
  "changes": [
    {
      "subspace": "Staking",
      "key": "MaxValidators",
      "value": 105
    }
  ],
  "deposit": [
    {
      "denom": "uscrt",
      "amount": "10000000"
    }
  ]
}
```

You can see another `param-change` example here: [enigma-1-proposal-3.json](https://github.com/enigmampc/SecretNetwork/blob/4561c0904c7b7659f019b96147cde13ac8db0933/enigma-1-proposal-3.json)

#### Subspaces, Keys and Values

| Subspace       | Key                       | Type             | Example                                                                                                   |
| -------------- | ------------------------- | ---------------- | --------------------------------------------------------------------------------------------------------- |
| `auth`         | `MaxMemoCharacters`       | string (uint64)  | `"256"`                                                                                                   |
| `auth`         | `TxSigLimit`              | string (uint64)  | `"7"`                                                                                                     |
| `auth`         | `TxSizeCostPerByte`       | string (uint64)  | `"10"`                                                                                                    |
| `auth`         | `SigVerifyCostED25519`    | string (uint64)  | `"590"`                                                                                                   |
| `auth`         | `SigVerifyCostSecp256k1`  | string (uint64)  | `"1000"`                                                                                                  |
| `bank`         | `sendenabled`             | bool             | `true`                                                                                                    |
| `crisis`       | `ConstantFee`             | object (coin)    | `{"denom": "uscrt", "amount": "1000"}`                                                                    |
| `distribution` | `communitytax`            | string (dec)     | `"0.020000000000000000"`                                                                                  |
| `distribution` | `secretfoundationtax`     | string (dec)     | `"0.030000000000000000"`                                                                                  |
| `distribution` | `secretfoundationaddress` | string           | `"secret164z7wwzv84h4hwn6rvjjkns6j4ht43jv8u9k0c"`                                                         |
| `distribution` | `baseproposerreward`      | string (dec)     | `"0.010000000000000000"`                                                                                  |
| `distribution` | `bonusproposerreward`     | string (dec)     | `"0.040000000000000000"`                                                                                  |
| `distribution` | `withdrawaddrenabled`     | bool             | `true`                                                                                                    |
| `evidence`     | `MaxEvidenceAge`          | string (time ns) | `"120000000000"`                                                                                          |
| `gov`          | `depositparams`           | object           | `{"min_deposit": [{"denom": "uscrt", "amount": "10000000"}], "max_deposit_period": "172800000000000"}`    |
| `gov`          | `votingparams`            | object           | `{"voting_period": "172800000000000"}`                                                                    |
| `gov`          | `tallyparams`             | object           | `{"quorum": "0.334000000000000000", "threshold": "0.500000000000000000", "veto": "0.334000000000000000"}` |
| `mint`         | `MintDenom`               | string           | `"uscrt"`                                                                                                 |
| `mint`         | `InflationRateChange`     | string (dec)     | `"0.080000000000000000"`                                                                                  |
| `mint`         | `InflationMax`            | string (dec)     | `"0.150000000000000000"`                                                                                  |
| `mint`         | `InflationMin`            | string (dec)     | `"0.070000000000000000"`                                                                                  |
| `mint`         | `GoalBonded`              | string (dec)     | `"0.670000000000000000"`                                                                                  |
| `mint`         | `BlocksPerYear`           | string (uint64)  | `"6311520"`                                                                                               |
| `slashing`     | `SignedBlocksWindow`      | string (int64)   | `"5000"`                                                                                                  |
| `slashing`     | `MinSignedPerWindow`      | string (dec)     | `"0.500000000000000000"`                                                                                  |
| `slashing`     | `DowntimeJailDuration`    | string (time ns) | `"600000000000"`                                                                                          |
| `slashing`     | `SlashFractionDoubleSign` | string (dec)     | `"0.050000000000000000"`                                                                                  |
| `slashing`     | `SlashFractionDowntime`   | string (dec)     | `"0.010000000000000000"`                                                                                  |
| `staking`      | `UnbondingTime`           | string (time ns) | `"259200000000000"`                                                                                       |
| `staking`      | `MaxValidators`           | uint16           | `100`                                                                                                     |
| `staking`      | `KeyMaxEntries`           | uint16           | `7`                                                                                                       |
| `staking`      | `HistoricalEntries`       | uint16           | `3`                                                                                                       |
| `staking`      | `BondDenom`               | string           | `"uscrt"`                                                                                                 |

Please note:

- The `subspace` is always the `ModuleName`: E.g. https://github.com/cosmos/cosmos-sdk/blob/v0.38.1/x/distribution/types/keys.go#L11
- The `key` is usually defined in `x/$MODULE_NAME/types/params.go`: E.g. https://github.com/cosmos/cosmos-sdk/blob/v0.38.1/x/distribution/types/params.go#L19-L22
- The `value`'s type is usually near the `key` definition: E.g. https://github.com/cosmos/cosmos-sdk/blob/v0.38.1/x/distribution/types/params.go#L26-L31
- :warning: `subspace` and `key` are case sensitive and `value` must be of the correct type and within the allowed bounds. Proposals with errors on these inputs should not enter voting period (should not get deposits) or be voted on with `NoWithVeto`.
- :warning: Currently parameter changes are _evaluated_ but not _validated_, so it is very important that any `value` change is valid (i.e. correct type and within bounds) for its respective parameter, eg. `MaxValidators` should be an integer and not a decimal.
- :warning: Proper vetting of a parameter change proposal should prevent this from happening (no deposits should occur during the governance process), but it should be noted regardless.

##### Known Constraints

- `distribution.baseproposerreward + distribution.bonusproposerreward < 1`. See [this](https://github.com/enigmampc/SecretNetwork/issues/95) and [this](https://github.com/cosmos/cosmos-sdk/issues/5808) for more info.

To read more go to https://github.com/gavinly/CosmosParametersWiki.

### Community Pool Spend

To submit a community pool spend proposal, you also must provide a proposal file as its contents are less friendly to CLI input:

```bash
secretcli tx gov submit-proposal community-pool-spend <path/to/proposal.json> --from <key_alias>
```

Where `proposal.json` is:

```json
{
  "title": "Community Pool Spend",
  "description": "Spend 10 SCRT with line breaks \n and `code formatting`",
  "recipient": "secret1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
  "amount": [
    {
      "denom": "uscrt",
      "amount": "10000000"
    }
  ],
  "deposit": [
    {
      "denom": "uscrt",
      "amount": "10000000"
    }
  ]
}
```

### Software Upgrade

The `SoftwareUpgrade` is currently not supported as it's not implemented and currently does not differ from the semantics of a `Text` proposal.

## Query Proposals

Once created, you can now query information of the proposal:

```bash
secretcli query gov proposal <proposal_id>
```

Or query all available proposals:

```bash
secretcli query gov proposals
```

You can also query proposals filtered by `voter` or `depositor` by using the corresponding flags.

To query for the proposer of a given governance proposal:

```bash
secretcli query gov proposer <proposal_id>
```

## Increase Deposit

If the proposal you previously created didn't meet the `MinDeposit` requirement, you can still increase the total amount deposited to activate it. Once the minimum deposit is reached, the proposal enters voting period:

```bash
secretcli tx gov deposit <proposal_id> "10000000uscrt" --from <key_alias>
```

_NOTE_: Proposals that don't meet this requirement will be deleted after `MaxDepositPeriod` is reached.

The only ways deposits won't be returned to their owners is:

1. If in the voting period the proposal gets 1/3 `NoWithVeto` out of all votes, excluding Abstain votes (So `NoWithVeto` needs to be 1/3 out of all `Yes`, `No` & `NoWithVeto` ).
2. If in the voting period less than 1/3 of voting power votes (== The proposal won't reach a quorum).

Anyone can deposit for a proposal, even if you have 0 `SCRT` tokens staked/delegated/bonded.

## Query Deposits

Once a new proposal is created, you can query all the deposits submitted to it:

```bash
secretcli query gov deposits <proposal_id>
```

You can also query a deposit submitted by a specific address:

```bash
secretcli query gov deposit <proposal_id> <depositor_address>
```

## Vote on a Proposal

After a proposal's deposit reaches the `MinDeposit` value, the voting period opens. Bonded `SCRT` holders can then cast vote on it:

```bash
secretcli tx gov vote <proposal_id> <Yes/No/NoWithVeto/Abstain> --from <key_alias>
```

## Query Votes

Check the vote with the option you just submitted:

```bash
secretcli query gov vote <proposal_id> <voter_address>
```

You can also get all the previous votes submitted to the proposal with:

```bash
secretcli query gov votes <proposal_id>
```

## Query proposal tally results

To check the current tally of a given proposal you can use the `tally` command:

```bash
secretcli query gov tally <proposal_id>
```

## Query Governance Parameters

To check the current governance parameters run:

```bash
secretcli query gov params
```

To query subsets of the governance parameters run:

```bash
secretcli query gov param voting
secretcli query gov param tallying
secretcli query gov param deposit
```
