# Client

Section describes interaction with the module by a client.

## CLI

A client can query and interact with the module using the CLI.

### Query

The `query` commands allows clients to query the module state.

```bash
app q usc -h
```

#### params

Get the current module parameters.

Usage:

```bash
app q usc params [flags]
```

Example output:

```bash
params:
  collateral_metas:
  - decimals: 6
    denom: uusdt
    description: USDT native token (micro USDT)
  - decimals: 6
    denom: ibc/312F13C9A9ECCE611FE8112B5ABCF0A14DE2C3937E38DEBF6B73F2534A83464E
    description: 'Osmosis: Terra USD token (micro USD)'
  max_redeem_entries: 7
  redeem_dur: 60s
  usc_meta:
    decimals: 6
    denom: uusc
    description: USC native token (micro USC)
```

#### pool

Get the current `Active` and `Redeeming` pool supplies.

Usage:

```bash
app q usc pool [flags]
```

Example output:

```bash
active_pool:
- amount: "7000000"
  denom: ibc/312F13C9A9ECCE611FE8112B5ABCF0A14DE2C3937E38DEBF6B73F2534A83464E
- amount: "4000000"
  denom: uusdt
redeeming_pool: []
```

#### redeem-entry

Get the current client's redeeming state.

Usage:

```bash
app q usc redeem-entry [address] [flags]
```

Example:

```bash
app q usc redeem-entry cosmos1dtld468wkdxs3u4zpcwze8yczg43xutjwc7q3a
```

Example output:

```bash
entry:
  address: cosmos1dtld468wkdxs3u4zpcwze8yczg43xutjwc7q3a
  operations:
  - collateral_amount:
    - amount: "1000"
      denom: ibc/312F13C9A9ECCE611FE8112B5ABCF0A14DE2C3937E38DEBF6B73F2534A83464E
    completion_time: "2022-06-17T12:10:10.659041535Z"
    creation_height: "35613"
```

### Transactions

The `tx` commands allows clients to interact with the module.

```bash
app tx usc -h
```

#### mint

Mint the **USC** token in exchange for collateral tokens.

Usage:

```bash
app tx usc mint [collaterals_amount] --from [account_name] [flags]
```

Example:

```bash
app tx usc mint 1000ibc/312F13C9A9ECCE611FE8112B5ABCF0A14DE2C3937E38DEBF6B73F2534A83464E --from tiky
```

#### redeem

Exchange collateral tokens for the **USC** token.

Example:

```bash
app tx usc redeem 1000uusc --from tiky
```

### Governance

#### Parameter change

CLI example on how to add a new collateral to the list of supported tokens.

Proposal file (`proposal_usc_param_1.json`):

```json
{
  "title": "x/usc module param change: adding a token to the supported collateral tokens array",
  "description": "Adding the ibc/312F13C9A9ECCE611FE8112B5ABCF0A14DE2C3937E38DEBF6B73F2534A83464E token (ibc/BE1BB42D4BE3C30D50B68D7C41DB4DFCE9678E8EF8C539F6E6A9345048894FCC received from the Osmosis via IBC)",
  "changes": [
    {
      "subspace": "usc",
      "key": "CollateralMetas",
      "value": [
        {
          "denom": "uusdt",
          "decimals": 6,
          "description": "USDT native token (micro USDT)"
        },
        {
          "denom": "ibc/312F13C9A9ECCE611FE8112B5ABCF0A14DE2C3937E38DEBF6B73F2534A83464E",
          "decimals": 6,
          "description": "Osmosis: Terra USD token (micro USD)"
        }
      ]
    }
  ],
  "deposit": "10000000stake"
}
```

Submit proposal CLI:

```bash
app gov submit-proposal param-change ./proposal_usc_param_1.json --from tiky
```
