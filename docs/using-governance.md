# How to participate in on-chain governance

Governance is the process from which users in the Enigma Blockchain can come to consensus on software upgrades, parameters of the mainnet or signaling mechanisms through text proposals. This is done through voting on proposals, which will be submitted by `SCRT` holders on the mainnet.

Some considerations about the voting process:

- Voting is done by bonded `SCRT` holders on a 1 bonded `SCRT` 1 vote basis.
- Delegators inherit the vote of their validator if they don't vote.
- Votes are tallied at the end of the voting period (1 week on mainnet) where each address can vote multiple times to update its `Option` value (paying the transaction fee each time), only the most recently cast vote will count as valid.
- Voters can choose between options `Yes`, `No`, `NoWithVeto` and `Abstain`.
- At the end of the voting period, a proposal is accepted IFF:
  - `(YesVotes / (YesVotes+NoVotes+NoWithVetoVotes)) > 1/2` ([threshold](https://github.com/enigmampc/EnigmaBlockchain/blob/b0792cc7f63a9264afe5de252a5821788c21834d/enigma-1-genesis.json#L1864))
  - `(NoWithVetoVotes / (YesVotes+NoVotes+NoWithVetoVotes)) < 1/3` ([veto](https://github.com/enigmampc/EnigmaBlockchain/blob/b0792cc7f63a9264afe5de252a5821788c21834d/enigma-1-genesis.json#L1865))
  - `((YesVotes+NoVotes+NoWithVetoVotes) / totalBondedStake) >= 1/3` ([quorum](https://github.com/enigmampc/EnigmaBlockchain/blob/b0792cc7f63a9264afe5de252a5821788c21834d/enigma-1-genesis.json#L1863))

For more information about the governance process and how it works, please check out the Governance module [specification](https://github.com/cosmos/cosmos-sdk/tree/master/x/gov/spec).

## Setup

- [How to use a light client (Windows, Mac & Linux)](/docs/ligth-client-mainnet.md)
- [Ledger Nano S support](/docs/ledger-nano-s.md)

## Create a Governance Proposal

In order to create a governance proposal, you must submit an initial deposit along with a title and description. Currently, in order to enter the voting period, a proposal must accumulate within a week deposits of at least [1000 SCRT](https://github.com/enigmampc/EnigmaBlockchain/blob/b0792cc7f63a9264afe5de252a5821788c21834d/enigma-1-genesis.json#L1851-L1856).

Various modules outside of governance may implement their own proposal types and handlers (eg. parameter changes), where the governance module itself supports `Text` proposals. Any module outside of governance has it's command mounted on top of `submit-proposal`.

To submit a `Text` proposal:

```bash
enigmacli tx gov submit-proposal \
  --title <title> \
  --description <description> \
  --type Text \
  --deposit 1000000uscrt \
  --from <key_alias>
```

You may also provide the proposal directly through the `--proposal` flag which points to a JSON file containing the proposal:

```bash
enigmacli tx gov submit-proposal \
  --type Text \
  --proposal <path/to/proposal.json> \
  --from <key_alias>
```

Where `proposal.json` is:

```json
{
  "title": "My Cool Proposal",
  "description": "A description with line breaks \n and `code formatting`",
  "deposit": [
    {
      "denom": "uscrt",
      "amount": "1000000"
    }
  ]
}
```

To submit a parameter change proposal, you must provide a proposal file as its contents are less friendly to CLI input:

```bash
enigmacli tx gov submit-proposal param-change <path/to/proposal.json> --from <key_alias>
```

Where `proposal.json` is:

```json
{
  "title": "Param Change",
  "description": "Update max validators",
  "changes": [
    {
      "subspace": "staking",
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

You can see another `param-change` example here: [enigma-1-proposal-3.json](/enigma-1-proposal-3.json)

:warning: Currently parameter changes are _evaluated_ but not _validated_, so it is very important that any `value` change is valid (ie. correct type and within bounds) for its respective parameter, eg. `MaxValidators` should be an integer and not a decimal.

Proper vetting of a parameter change proposal should prevent this from happening (no deposits should occur during the governance process), but it should be noted regardless.

The `SoftwareUpgrade` is currently not supported as it's not implemented and currently does not differ from the semantics of a `Text` proposal.

## Query Proposals

Once created, you can now query information of the proposal:

```bash
enigmacli query gov proposal <proposal_id>
```

Or query all available proposals:

```bash
enigmacli query gov proposals
```

You can also query proposals filtered by `voter` or `depositor` by using the corresponding flags.

To query for the proposer of a given governance proposal:

```bash
enigmacli query gov proposer <proposal_id>
```

## Increase Deposit

If the proposal you previously created didn't meet the `MinDeposit` requirement, you can still increase the total amount deposited to activate it. Once the minimum deposit is reached, the proposal enters voting period:

```bash
enigmacli tx gov deposit <proposal_id> "10000000uscrt" --from <key_alias>
```

_NOTE_: Proposals that don't meet this requirement will be deleted after `MaxDepositPeriod` is reached.

The only ways deposits won't be returned to their owners is:

1. If in the voting period the proposal gets 1/3 `NoWithVeto` out of all votes, excluding Abstain votes (So `NoWithVeto` needs to be 1/3 out of all `Yes`, `No` & `NoWithVeto` ).
2. If in the voting period less than 1/3 of voting power votes (== The proposal won't reach a quorum).

Anyone can deposit for a proposal, even if you have 0 `uSCRT` tokens staked/delegated/bonded.

## Query Deposits

Once a new proposal is created, you can query all the deposits submitted to it:

```bash
enigmacli query gov deposits <proposal_id>
```

You can also query a deposit submitted by a specific address:

```bash
enigmacli query gov deposit <proposal_id> <depositor_address>
```

## Vote on a Proposal

After a proposal's deposit reaches the `MinDeposit` value, the voting period opens. Bonded `SCRT` holders can then cast vote on it:

```bash
enigmacli tx gov vote <proposal_id> <Yes/No/NoWithVeto/Abstain> --from <key_alias>
```

## Query Votes

Check the vote with the option you just submitted:

```bash
enigmacli query gov vote <proposal_id> <voter_address>
```

You can also get all the previous votes submitted to the proposal with:

```bash
enigmacli query gov votes <proposal_id>
```

## Query proposal tally results

To check the current tally of a given proposal you can use the `tally` command:

```bash
enigmacli query gov tally <proposal_id>
```

## Query Governance Parameters

To check the current governance parameters run:

```bash
enigmacli query gov params
```

To query subsets of the governance parameters run:

```bash
enigmacli query gov param voting
enigmacli query gov param tallying
enigmacli query gov param deposit
```
