import { cosmos } from "./ProtoDefs.js";

// todo: dynamically create this type from protobuf
export class MsgData {
  msgType: string;
  data: string | undefined;

  constructor(msgType: string, data: string | undefined) {
    this.msgType = msgType;
    this.data = data;
  }
}

export function decodeTxData(data: Uint8Array): MsgData[] {
  const message = cosmos.base.abci.v1beta1.TxMsgData.decode(data);
  const object = cosmos.base.abci.v1beta1.TxMsgData.toObject(message, {
    longs: String,
    enums: String,
    bytes: String,
    // see ConversionOptions
  });

  return object["data"].map((item: any) => {
    return new MsgData(item.msgType, item?.data);
  });
}
