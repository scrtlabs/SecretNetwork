## Post Mortem

* **Chain id:** `enigma-testnet`
* **Date:** 16/03/2020 3am UTC
* **Related issues:** https://github.com/enigmampc/EnigmaBlockchain/issues/95
### Description
* On the 15 Mar 2020, around 9pm UTC the following param-change proposal was submitted:
    ```
    {
      "title": "Rwd Change",
      "description": "This is tansaction to test a parameter-change for rewards.",
      "changes": [
        {
          "subspace": "distribution",
          "key": "baseproposerreward",
          "value": "0.999000000000000000"
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

* At around 3am UTC of the following night the proposal got accepted, and as a result the network halted, with following error:
    ```
    Mar 16 05:02:52 ip-172-31-44-28 enigmad[20612]: I[2020-03-16|05:02:52.767] Executed block                               module=state height=171146 validTxs=0 invalidTxs=0
    Mar 16 05:02:52 ip-172-31-44-28 enigmad[20612]: I[2020-03-16|05:02:52.780] Committed state                              module=state height=171146 txs=0 appHash=FB4739B6F0D4FED77D431922E340B95B8144BF37483D3C1225431311A5BB229D
    Mar 16 05:02:58 ip-172-31-44-28 enigmad[20612]: E[2020-03-16|05:02:58.075] CONSENSUS FAILURE!!!                         module=consensus err="negative coin amount" stack="goroutine 1419 [running]:\nruntime/debug.Stack(0xc0012c2ce0, 0x1115f40, 0x16671b0)\n\t/usr/lib/go-1.13/src/runtime/debug/stack.go:24 +0x9d\ngithub.com/tendermint/tendermint/consensus.(*State).receiveRoutine.func2(0xc002741c00, 0x149b478)\n\t/home/assafmo/workspace/go/pkg/mod/github.com/tendermint/tendermint@v0.33.0/consensus/state.go:613 +0x57\npanic(0x1115f40, 0x16671b0)\n\t/usr/lib/go-1.13/src/runtime/panic.go:679 +0x1b2\ngithub.com/cosmos/cosmos-sdk/types.DecCoins.Sub(...)\n\t/home/assafmo/workspace/go/pkg/mod/github.com/cosmos/cosmos-sdk@v0.38.1/types/dec_coin.go:307\ngithub.com/cosmos/cosmos-sdk/x/distribution/keeper.Keeper.AllocateTokens(0x169cf60, 0xc000eab5b0, 0xc000ab8460, 0xc000ab8460, 0x169cf60, 0xc000eab5e0, 0x169cfa0, 0xc000eab630, 0xc000e9fbe0, 0xc, ...)\n\t/home/assafmo/workspace/go/pkg/mod/github.com/cosmos/cosmos-sdk@v0.38.1/x/distribution/keeper/allocation.go:66 +0x1589\ngithub.com/cosmos/cosmos-sdk/x/distribution.BeginBlocker(0x16ad9a0, 0xc000034048, 0x16c2100, 0xc005382800, 0xa, 0x0, 0x0, 0x0, 0x0, 0x0, ...)\n\t/home/assafmo/workspace/go/pkg/mod/github.com/cosmos/cosmos-sdk@v0.38.1/x/distribution/abci.go:26 +0x2e6\ngithub.com/cosmos/cosmos-sdk/x/distribution.AppModule.BeginBlock(...)\n\t/home/assafmo/workspace/go/pkg/mod/github.com/cosmos/cosmos-sdk@v0.38.1/x/distribution/module.go:147\ngithub.com/cosmos/cosmos-sdk/types/module.(*Manager).BeginBlock(0xc000ab9030, 0x16ad9a0, 0xc000034048, 0x16c2100, 0xc005382800, 0xa, 0x0, 0x0, 0x0, 0x0, ...)\n\t/home/assafmo/workspace/go/pkg/mod/github.com/cosmos/cosmos-sdk@v0.38.1/types/module/module.go:297 +0x1ca\ngithub.com/enigmampc/EnigmaBlockchain.(*EnigmaChainApp).BeginBlocker(...)\n\t/home/assafmo/workspace/enigmachain/app.go:391\ngithub.com/cosmos/cosmos-sdk/baseapp.(*BaseApp).BeginBlock(0xc000d197c0, 0xc004071c00, 0x20, 0x20, 0xa, 0x0, 0x0, 0x0, 0x0, 0x0, ...)\n\t/home/assafmo/workspace/go/pkg/mod/github.com/cosmos/cosmos-sdk@v0.38.1/baseapp/abci.go:136 +0x469\ngithub.com/tendermint/tendermint/abci/client.(*localClient).BeginBlockSync(0xc000e7cd20, 0xc004071c00, 0x20, 0x20, 0xa, 0x0, 0x0, 0x0, 0x0, 0x0, ...)\n\t/home/assafmo/workspace/go/pkg/mod/github.com/tendermint/tendermint@v0.33.0/abci/client/local_client.go:231 +0x101\ngithub.com/tendermint/tendermint/proxy.(*appConnConsensus).BeginBlockSync(0xc000eab1b0, 0xc004071c00, 0x20, 0x20, 0xa, 0x0, 0x0, 0x0, 0x0, 0x0, ...)\n\t/home/assafmo/workspace/go/pkg/mod/github.com/tendermint/tendermint@v0.33.0/proxy/app_conn.go:69 +0x6b\ngithub.com/tendermint/tendermint/state.execBlockOnProxyApp(0x16ae3a0, 0xc0027fa9a0, 0x16bb520, 0xc000eab1b0, 0xc003dc3a40, 0x16c4080, 0xc000011138, 0x6, 0xc00272a9a0, 0xe)\n\t/home/assafmo/workspace/go/pkg/mod/github.com/tendermint/tendermint@v0.33.0/state/execution.go:280 +0x3e1\ngithub.com/tendermint/tendermint/state.(*BlockExecutor).ApplyBlock(0xc0000fce00, 0xa, 0x0, 0xc00272a980, 0x6, 0xc00272a9a0, 0xe, 0x29c8a, 0xc0038a1660, 0x20, ...)\n\t/home/assafmo/workspace/go/pkg/mod/github.com/tendermint/tendermint@v0.33.0/state/execution.go:131 +0x17a\ngithub.com/tendermint/tendermint/consensus.(*State).finalizeCommit(0xc002741c00, 0x29c8b)\n\t/home/assafmo/workspace/go/pkg/mod/github.com/tendermint/tendermint@v0.33.0/consensus/state.go:1431 +0x8f5\ngithub.com/tendermint/tendermint/consensus.(*State).tryFinalizeCommit(0xc002741c00, 0x29c8b)\n\t/home/assafmo/workspace/go/pkg/mod/github.com/tendermint/tendermint@v0.33.0/consensus/state.go:1350 +0x383\ngithub.com/tendermint/tendermint/consensus.(*State).enterCommit.func1(0xc002741c00, 0x0, 0x29c8b)\n\t/home/assafmo/workspace/go/pkg/mod/github.com/tendermint/tendermint@v0.33.0/consensus/state.go:1285 +0x90\ngithub.com/tendermint/tendermint/consensus.(*State).enterCommit(0xc002741c00, 0x29c8b, 0x0)\n\t/home/assafmo/workspace/go/pkg/mod/github.com/tendermint/tendermint@v0.33.0/consensus/state.go:1322 +0x61a\ngithub.com/tendermint/tendermint/consensus.(*State).addVote(0xc002741c00, 0xc005087c20, 0xc003fd0b70, 0x28, 0xc0012c9a38, 0xd31f92, 0x0)\n\t/home/assafmo/workspace/go/pkg/mod/github.com/tendermint/tendermint@v0.33.0/consensus/state.go:1819 +0xa39\ngithub.com/tendermint/tendermint/consensus.(*State).tryAddVote(0xc002741c00, 0xc005087c20, 0xc003fd0b70, 0x28, 0xf136b9f2d600ff82, 0x108, 0x100)\n\t/home/assafmo/workspace/go/pkg/mod/github.com/tendermint/tendermint@v0.33.0/consensus/state.go:1642 +0x59\ngithub.com/tendermint/tendermint/consensus.(*State).handleMsg(0xc002741c00, 0x168b640, 0xc0028f4538, 0xc003fd0b70, 0x28)\n\t/home/assafmo/workspace/go/pkg/mod/github.com/tendermint/tendermint@v0.33.0/consensus/state.go:709 +0x252\ngithub.com/tendermint/tendermint/consensus.(*State).receiveRoutine(0xc002741c00, 0x0)\n\t/home/assafmo/workspace/go/pkg/mod/github.com/tendermint/tendermint@v0.33.0/consensus/state.go:644 +0x6eb\ncreated by github.com/tendermint/tendermint/consensus.(*State).OnStart\n\t/home/assafmo/workspace/go/pkg/mod/github.com/tendermint/tendermint@v0.33.0/consensus/state.go:335 +0x13a\n"
    Mar 16 05:36:48 ip-172-31-44-28 enigmad[20612]: E[2020-03-16|05:36:48.395] Connection failed @ sendRoutine              module=p2p peer=842822b88cb5762ba99cee50a37156c6d0a6c452@149.248.55.89:60440 conn=MConn{149.248.55.89:60440} err="pong timeout"
    Mar 16 05:36:48 ip-172-31-44-28 enigmad[20612]: E[2020-03-16|05:36:48.402] Stopping peer for error                      module=p2p peer="Peer{MConn{149.248.55.89:60440} 842822b88cb5762ba99cee50a37156c6d0a6c452 in}" err="pong timeout"
    ```
* When the vote passed, the `distribution` module parameters changed to:
    ```
    community_tax: "0.020000000000000000"
    base_proposer_reward: "0.999000000000000000"
    bonus_proposer_reward: "0.040000000000000000"
    withdraw_addr_enabled: true
    ```
* The problem occurred because the sum of `baseproposerreward` and `bonusproposerreward` can't be grater than 1 i.e. `0.999 + 0.04 > 1`. This results in miscalculations of the rewards and fees.
* The cause is a bug in Cosmos SDK in the parameter value validation, causing the proposal to pass despite being invalid. More on that here: https://github.com/cosmos/cosmos-sdk/issues/5808

### Additional Notes
* Another invalid proposal was on voting period, and by itself would have caused the network to halt as well:
    ```
    "changes": [
      {
        "subspace": "distribution",
        "key": "bonusproposerreward",
        "value": "\"0.999000000000000000\""
      }
    ]
    ```

### Action Items:
* https://github.com/enigmampc/EnigmaBlockchain/issues/95
* https://github.com/enigmampc/EnigmaBlockchain/issues/97
* https://github.com/enigmampc/EnigmaBlockchain/issues/104

## Recovery Process

1. Logged in to the testnet bootstrap machine.
2. Exported state from the last "rounded" block height:
    ```
    enigmad export --for-zero-height --height=170000 > state_export.json
    ```
3. Removed all references to proposal ids `4` and `5` in:
    ```
    "gov":{
        "deposits":[...],
        "proposals":[...],
        "votes":[...],
        ...
    }
    ```
4. Made sure the `distribution` parameters are still make sense:
    ```
    "distribution":{
        "params":{
                "base_proposer_reward":"0.010000000000000000",
                "bonus_proposer_reward":"0.040000000000000000",
                "community_tax":"0.020000000000000000",
                "withdraw_addr_enabled":true
        },
        ...
    }
    ```
5. Erased the `coins` in possesion of the `gov` ModuleAccount:
    ```
    "auth":{
        "accounts":[
            {
                "type":"cosmos-sdk/ModuleAccount",
                    "value":{
                      "account_number":8,
                      "address":"enigma10d07y265gmmuvt4z0w9aw880jnsr700jt22en3",
                      "coins":[],
                      "name":"gov",
                      "permissions":[
                          "burner"
                      ],
                      "public_key":"",
                      "sequence":0
                    }
            }, ...
    }, ...
    ```
6. "Refund" coins to the account that deposited to these proposals on the first place i.e. added to account's balance in:
    ```
    "app_state":{
      "auth":{
         "accounts":[
            {
               "value":{
                  "coins":[
                     {
                        "amount":"<added to this amount>"
                     }
                  ]
               }
            }
         ]
      }
    }
    ```
7. A problem occured with staking, described at: https://github.com/cosmos/cosmos-sdk/issues/5818
    Changed the following:
    ```
    "distribution":{
      "delegator_starting_infos":[
        {
          ...,
          "starting_info":{
            "...,
            "stake":"999990000.000000000000000000"
          },
          ...
        },
        ...
      ],
      ...
    }
    ```

    To this:
    ```
    "distribution":{
      "delegator_starting_infos":[
        {
          ...,
          "starting_info":{
            "...,
            "stake":"990000000.000000000000000000"
          },
          ...
        },
        ...
      ],
      ...
    }
    ```
8. Then a problem occured with the `compute` module:
    ```
    panic: create wasm contract failed: Wasm Error: Filesystem error: File exists (os error 17)
    ```
    This one got fixed when deleted the `.enigmad/data/.compute` directory.
9. Reset state:
    ```
    enigmad unsafe-reset-all
    ```
10. Restarted the node.
