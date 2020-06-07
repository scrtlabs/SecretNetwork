export default class EnigmaUtils {
  private readonly apiUrl;
  private consensusIoPubKey;
  constructor(apiUrl: string);
  private getTxSenderKeyPair;
  private getConsensusIoPubKey;
  private getTxEncryptionKey;
  encrypt(msg: object): Promise<Uint8Array>;
  decrypt(ciphertext: Uint8Array, nonce: Uint8Array): Promise<Uint8Array>;
}
