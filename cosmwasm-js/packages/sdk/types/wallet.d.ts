import { StdSignature } from "./types";
import { StdSignDoc } from "./encoding";
export declare type PrehashType = "sha256" | "sha512" | null;
export declare type Algo = "secp256k1" | "ed25519" | "sr25519";
export interface AccountData {
  readonly address: string;
  readonly algo: Algo;
  readonly pubkey: Uint8Array;
}
export interface SignResponse {
  /**
   * The sign doc that was signed.
   * This may be different from the input signDoc when the signer modifies it as part of the signing process.
   */
  readonly signed: StdSignDoc;
  readonly signature: StdSignature;
}
export interface OfflineSigner {
  /**
   * Get AccountData array from wallet. Rejects if not enabled.
   */
  readonly getAccounts: () => Promise<readonly AccountData[]>;
  /**
   * Request signature from whichever key corresponds to provided bech32-encoded address. Rejects if not enabled.
   */
  readonly sign: (signerAddress: string, signDoc: StdSignDoc) => Promise<SignResponse>;
}
