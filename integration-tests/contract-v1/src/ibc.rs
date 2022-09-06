use cosmwasm_std::{entry_point, to_binary, DepsMut, Env, IbcBasicResponse, IbcChannelCloseMsg, IbcChannelConnectMsg, IbcChannelOpenMsg, IbcMsg, IbcPacketAckMsg, IbcPacketReceiveMsg, IbcPacketTimeoutMsg, IbcReceiveResponse, StdResult, Ibc3ChannelOpenResponse, from_binary};
use serde_json_wasm::from_slice;

use crate::msg::PacketMsg;
use crate::state::{channel_store};

/// packets live one hour
pub const PACKET_LIFETIME: u64 = 60 * 60;

#[entry_point]
pub fn ibc_channel_open(
    _deps: DepsMut,
    _env: Env,
    msg: IbcChannelOpenMsg,
) -> StdResult<Option<Ibc3ChannelOpenResponse>> {
    let channel = msg.channel();
    // todo maybe save this to state to check
    let _counter_port_id = channel.counterparty_endpoint.port_id.clone();

    Ok(Some(Ibc3ChannelOpenResponse {
        version: "test".to_string(),
    }))
}

#[entry_point]
pub fn ibc_channel_connect(
    deps: DepsMut,
    env: Env,
    msg: IbcChannelConnectMsg,
) -> StdResult<IbcBasicResponse> {
    let channel = msg.channel();
    let channel_id = &channel.endpoint.channel_id;

    // save channel to state
    channel_store(deps.storage).save(channel_id)?;

    // construct a packet to send
    // let packet = PacketMsg::Test {};
    // let msg = IbcMsg::SendPacket {
    //     channel_id: channel_id.clone(),
    //     data: to_binary(&packet)?,
    //     timeout: env.block.time.plus_seconds(PACKET_LIFETIME).into(),
    // };

    Ok(IbcBasicResponse::new()
        // .add_message(msg)
        .add_attribute("action", "ibc_connect")
        .add_attribute("channel_id", channel_id))
}

#[entry_point]
pub fn ibc_channel_close(
    _deps: DepsMut,
    _env: Env,
    msg: IbcChannelCloseMsg,
) -> StdResult<IbcBasicResponse> {
    let channel = msg.channel();
    let channel_id = &channel.endpoint.channel_id;

    Ok(IbcBasicResponse::new()
        .add_attribute("action", "ibc_close")
        .add_attribute("channel_id", channel_id))
}

#[entry_point]
/// never should be called as the other side never sends packets
pub fn ibc_packet_receive(
    _deps: DepsMut,
    _env: Env,
    packet: IbcPacketReceiveMsg,
) -> StdResult<IbcReceiveResponse> {
    let msg: PacketMsg = from_binary(&packet.packet.data)?;

    let mut response = IbcReceiveResponse::new();
    response = match msg {
        PacketMsg::Test { } => response.set_ack(b"test"),
        PacketMsg::Message { value} => response
            .set_ack(("recv".to_string() + &value).as_bytes())
            .add_attribute("acknowledging", value),
    };

    Ok(response.add_attribute("action", "ibc_packet_ack"))
}

#[entry_point]
pub fn ibc_packet_ack(
    _deps: DepsMut,
    _env: Env,
    msg: IbcPacketAckMsg,
) -> StdResult<IbcBasicResponse> {
    // which local channel was this packet send from
    let caller = msg.original_packet.src.channel_id.clone();
    let ack_data: String = from_binary(&msg.acknowledgement.data)?;

    Ok(IbcBasicResponse::new()
        .add_attribute("caller", caller)
        .add_attribute("data", ack_data + "end")
    )
}

#[entry_point]
/// we just ignore these now.
pub fn ibc_packet_timeout(
    _deps: DepsMut,
    _env: Env,
    _msg: IbcPacketTimeoutMsg,
) -> StdResult<IbcBasicResponse> {
    Ok(IbcBasicResponse::new().add_attribute("action", "ibc_packet_timeout"))
}