# End-block

Section describes the module state change on the ABCI end block call.

## Redeeming queue

Finalise all mature redeeming queue items with the following procedure:

- transfer locked within the `Reddeming` pool funds to a client account address;
- remove the redeeming queue time slice item;
- update / remove (if no operations left) a corresponding to a particular account address `RedeemEntry` object;
