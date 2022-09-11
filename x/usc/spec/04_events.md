# Events

The module emits the following events:

## EndBlocker

| Type                   | Attribute key   | Attribute value                  |
|------------------------|-----------------|----------------------------------|
| collateral_redeem_done | sender          | {accountAddress}                 |
| collateral_redeem_done | amount          | {collateralTokensRedeemedAmount} |
| collateral_redeem_done | completion_time | {currentBlockTime}               |

## Messages

### MsgMintUSC

| Type       | Attribute key | Attribute value              |
|------------|---------------|------------------------------|
| usc_minted | sender        | {accountAddress}             |
| usc_minted | minted_amount | {uscTokenMintedAmount}       |
| usc_minted | used_amount   | {collateralTokensUsedAmount} |

### MsgRedeemCollateral

| Type                     | Attribute key   | Attribute value                  |
|--------------------------|-----------------|----------------------------------|
| collateral_redeem_queued | sender          | {accountAddress}                 |
| collateral_redeem_queued | used_amount     | {uscTokenUsedAmount}             |
| collateral_redeem_queued | redeemed_amount | {collateralTokensRedeemedAmount} |
| collateral_redeem_queued | completion_time | {operationEndTime}               |
