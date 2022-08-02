use crate::addresses::Addr;
use crate::timestamp::Timestamp;
use serde::{Deserialize, Serialize};

/// The message that is passed into `ibc_channel_open`
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
#[serde(rename_all = "snake_case")]
pub enum IbcChannelOpenMsg {
    /// The ChanOpenInit step from https://github.com/cosmos/ibc/tree/master/spec/core/ics-004-channel-and-packet-semantics#channel-lifecycle-management
    OpenInit { channel: IbcChannel },
    /// The ChanOpenTry step from https://github.com/cosmos/ibc/tree/master/spec/core/ics-004-channel-and-packet-semantics#channel-lifecycle-management
    OpenTry {
        channel: IbcChannel,
        counterparty_version: String,
    },
}

/// IbcChannel defines all information on a channel.
/// This is generally used in the hand-shake process, but can be queried directly.
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub struct IbcChannel {
    pub endpoint: IbcEndpoint,
    pub counterparty_endpoint: IbcEndpoint,
    pub order: IbcOrder,
    /// Note: in ibcv3 this may be "", in the IbcOpenChannel handshake messages
    pub version: String,
    /// The connection upon which this channel was created. If this is a multi-hop
    /// channel, we only expose the first hop.
    pub connection_id: String,
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub struct IbcEndpoint {
    pub port_id: String,
    pub channel_id: String,
}

/// IbcOrder defines if a channel is ORDERED or UNORDERED
/// Values come from https://github.com/cosmos/cosmos-sdk/blob/v0.40.0/proto/ibc/core/channel/v1/channel.proto#L69-L80
/// Naming comes from the protobuf files and go translations.
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub enum IbcOrder {
    #[serde(rename = "ORDER_UNORDERED")]
    Unordered,
    #[serde(rename = "ORDER_ORDERED")]
    Ordered,
}

/// The message that is passed into `ibc_channel_connect`
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
#[serde(rename_all = "snake_case")]
pub enum IbcChannelConnectMsg {
    /// The ChanOpenAck step from https://github.com/cosmos/ibc/tree/master/spec/core/ics-004-channel-and-packet-semantics#channel-lifecycle-management
    OpenAck {
        channel: IbcChannel,
        counterparty_version: String,
    },
    /// The ChanOpenConfirm step from https://github.com/cosmos/ibc/tree/master/spec/core/ics-004-channel-and-packet-semantics#channel-lifecycle-management
    OpenConfirm { channel: IbcChannel },
}

/// The message that is passed into `ibc_channel_close`
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
#[serde(rename_all = "snake_case")]
pub enum IbcChannelCloseMsg {
    /// The ChanCloseInit step from https://github.com/cosmos/ibc/tree/master/spec/core/ics-004-channel-and-packet-semantics#channel-lifecycle-management
    CloseInit { channel: IbcChannel },
    /// The ChanCloseConfirm step from https://github.com/cosmos/ibc/tree/master/spec/core/ics-004-channel-and-packet-semantics#channel-lifecycle-management
    CloseConfirm { channel: IbcChannel }, // pub channel: IbcChannel,
}

/// The message that is passed into `ibc_packet_receive`
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub struct IbcPacketReceiveMsg {
    pub packet: IbcPacket,
    pub relayer: Addr,
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub struct IbcPacket {
    /// The raw data sent from the other side in the packet
    pub data: Binary,
    /// identifies the channel and port on the sending chain.
    pub src: IbcEndpoint,
    /// identifies the channel and port on the receiving chain.
    pub dest: IbcEndpoint,
    /// The sequence number of the packet on the given channel
    pub sequence: u64,
    pub timeout: IbcTimeout,
}

/// In IBC each package must set at least one type of timeout:
/// the timestamp or the block height. Using this rather complex enum instead of
/// two timeout fields we ensure that at least one timeout is set.
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
#[serde(rename_all = "snake_case")]
pub struct IbcTimeout {
    // use private fields to enforce the use of constructors, which ensure that at least one is set
    block: Option<IbcTimeoutBlock>,
    timestamp: Option<Timestamp>,
}

/// IBCTimeoutHeight Height is a monotonically increasing data type
/// that can be compared against another Height for the purposes of updating and
/// freezing clients.
/// Ordering is (revision_number, timeout_height)
#[derive(Serialize, Deserialize, Copy, Clone, Debug, PartialEq, Eq)]
pub struct IbcTimeoutBlock {
    /// the version that the client is currently on
    /// (eg. after reseting the chain this could increment 1 as height drops to 0)
    pub revision: u64,
    /// block height after which the packet times out.
    /// the height within the given revision
    pub height: u64,
}

/// The message that is passed into `ibc_packet_ack`
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub struct IbcPacketAckMsg {
    pub acknowledgement: IbcAcknowledgement,
    pub original_packet: IbcPacket,
    pub relayer: Addr,
}
