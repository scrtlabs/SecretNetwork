import { Msg, StdFee, StdTx } from "./types";
export declare function marshalTx(tx: StdTx): Uint8Array;
export declare function makeSignBytes(
  msgs: readonly Msg[],
  fee: StdFee,
  chainId: string,
  memo: string,
  accountNumber: number,
  sequence: number,
): Uint8Array;
/**
 * The document to be signed
 *
 * @see https://docs.cosmos.network/master/modules/auth/03_types.html#stdsigndoc
 */
export interface StdSignDoc {
  readonly chain_id: string;
  readonly account_number: string;
  readonly sequence: string;
  readonly fee: StdFee;
  readonly msgs: readonly Msg[];
  readonly memo: string;
}
