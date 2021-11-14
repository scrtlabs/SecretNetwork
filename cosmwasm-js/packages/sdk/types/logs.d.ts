export interface Attribute {
  key: string;
  value: string;
}
export interface Event {
  readonly type: string;
  readonly attributes: readonly Attribute[];
}
export interface Log {
  msg_index: number;
  log: string;
  events: readonly Event[];
}
export declare function parseAttribute(input: unknown): Attribute;
export declare function parseEvent(input: unknown): Event;
export declare function parseLog(input: unknown): Log;
export declare function parseLogs(input: unknown): readonly Log[];
/**
 * Searches in logs for the first event of the given event type and in that event
 * for the first first attribute with the given attribute key.
 *
 * Throws if the attribute was not found.
 */
export declare function findAttribute(
  logs: readonly Log[],
  eventType: "message" | "transfer",
  attrKey: string,
): Attribute;
