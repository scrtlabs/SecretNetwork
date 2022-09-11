# Messages

Section describes the processing of the module messages.

[Proto file with fields description](../../../proto/gaia/usc/v1beta1/tx.proto).

## MsgMintUSC

**USC** tokens mint is done using the `MsgMintUSC` message.

On success:

- minted **USC** tokens are transferred to a client account address;
- used collateral tokens are transferred to the `Active` pool;

This message is expected to fail if:

- collateral tokens passed with the `collateral_amount` field are not supported by the module;
- **USC** converted amount is zero due to small collateral amounts provided;

## MsgRedeemCollateral

To exchange **USC** tokens back to collaterals the `MsgRedeemCollateral` message is used.

On success:

- used **USC** tokens are burned;
- converted collateral tokens are transferred from the `Active` pool to the `Redeeming` pool;
- `RedeemEntry` object is created / updated for a client account address with a new redeeming operation;
- the redeeming queue time slice object is created / updated;

This message is expected to fail if:

- converted collateral amount is zero due to:
    - small **USC** amount provided;
    - insufficient `Active` pool supply (that could happen if one of the supported collaterals was removed from the
      whitelist);
