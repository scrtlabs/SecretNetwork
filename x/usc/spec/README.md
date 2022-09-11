# `usc`

## Abstract

The module enables Cosmos-SDK based blockchain to exchange multiple collateral tokens (**USDT**, **USDC**, **BUSD**,
etc) to a single **USC** token in 1:1 ratio.
The redeeming procedure (exchange **USC** back to collaterals) includes a redeeming period to prevent spam swap
operations.

Adding (removing) support of a particular collateral is done via changing the module parameters.
This change could be done using `x/gov` parameter change proposal.

Each supported token metadata has a *decimals* field to properly convert token amounts with the different level of
precision (1.0 **USC** with 6 decimals -> 1000000 in `Int` representation).
Having the *decimals* data could cause partial conversion of amounts (with leftovers).
For example converting 0.0011 of **someDenom** with 6 decimals to **USC** with 3 decimals will mint 0.001 **USC** and
leave the leftover part (0.0001 **someDenom**) on a client balance.

## Contents

1. **[State](01_state.md)**
2. **[Messages](02_messages.md)**
3. **[End-Block](03_end_block.md)**
4. **[Events](04_events.md)**
5. **[Parameters](05_params.md)**
6. **[Client](06_client.md)**
