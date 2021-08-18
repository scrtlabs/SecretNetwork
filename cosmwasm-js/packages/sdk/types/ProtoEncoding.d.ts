export declare class MsgData {
  msgType: string;
  data: string | undefined;
  constructor(msgType: string, data: string | undefined);
}
export declare function decodeTxData(data: Uint8Array): MsgData[];
