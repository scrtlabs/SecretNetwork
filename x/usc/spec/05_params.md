# Parameters

The contains the following parameters:

| Key              | Type                | Example                                                                                                                                                       |
|------------------|---------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------|
| RedeemDur        | string (time in ns) | "86400000000000" / "24h"                                                                                                                                      |
| MaxRedeemEntries | uint32              | 7                                                                                                                                                             |
| USCMeta          | TokenMeta           | { "denom": "uusc", "decimals": 6, "description": "USC native token (micro USC)" }                                                                             |
| CollateralMetas  | []TokenMeta         | [ { "denom": "ibc/312F13C9A9ECCE611FE8112B5ABCF0A14DE2C3937E38DEBF6B73F2534A83464E", "decimals": 6, "description": "Osmosis: Terra USD token (micro USD)" } ] |
