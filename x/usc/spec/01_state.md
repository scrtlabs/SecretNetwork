# State

Section describes all stored by the module objects and their storage keys.

[Proto file with fields description](../../../proto/gaia/usc/v1beta1/usc.proto).

## Pools

There are two pools used:

- `Active` - keeps the collateral tokens supply that can be returned on a client balance using the redeeming procedure
  in exchange for **USC** tokens;
- `Redeeming` - keeps the collateral tokens supply that is locked by the redeeming procedure (would be transferred on a
  client balance after the redeeming period);

## Params

- Params: `Paramsspace("usc") -> legacy_amino(params)`

Params is a module-wide configuration structure.

## RedeemEntry

- RedeemEntry: `0x11 | AccountAddr -> ProtocolBuffer(RedeemEntry)`

To convert **USC** tokens back to supported collateral tokens, a client starts the redeeming procedure.
`RedeemEntry` tracks current redeem operations for a particular account address.

Each operation has the `completion_time` field which indicated when a particular redeem operation is done.
A single account address could have up to `max_redeem_entries` concurrent redeem operations (defined within the module
params).

## Redeeming queue

- Redeeming queue: `0x10 | format(time) -> ProtocolBuffer(RedeemingQueueData)`

To track mature redeem operations (ready to finalise) the `Redeeming queue` index is used.
Queue items are time slices where:

- timestamp key defines the completion time for a set of redeem operations;
- value is a set of account addresses to link a particular queue item with a corresponding `RedeemEntry` object;
