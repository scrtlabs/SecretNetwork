export default class EnigmaUtils {
  private readonly apiUrl;
  readonly seed: Uint8Array;
  private readonly privkey;
  readonly pubkey: Uint8Array;
  private consensusIoPubKey;
  constructor(apiUrl: string, seed?: Uint8Array);
  static GenerateNewKeyPair(): {
    privkey: Uint8Array;
    pubkey: Uint8Array;
  };
  static GenerateNewSeed(): Uint8Array;
  static GenerateNewKeyPairFromSeed(
    seed: Uint8Array,
  ): {
    privkey: Uint8Array;
    pubkey: Uint8Array;
  };
  private getConsensusIoPubKey;
  private getTxEncryptionKey;
  encrypt(msg: object): Promise<Uint8Array>;
  decrypt(ciphertext: Uint8Array, nonce: Uint8Array): Promise<Uint8Array>;
}
