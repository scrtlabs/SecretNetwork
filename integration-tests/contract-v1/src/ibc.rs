use cosmwasm_std::{
    entry_point, from_binary, DepsMut, Env, Ibc3ChannelOpenResponse, IbcBasicResponse,
    IbcChannelCloseMsg, IbcChannelConnectMsg, IbcChannelOpenMsg, IbcPacketAckMsg,
    IbcPacketReceiveMsg, IbcPacketTimeoutMsg, IbcReceiveResponse, StdResult,
};

use crate::msg::PacketMsg;
use crate::state::{ack_store, channel_store, receive_store, timeout_store};

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
    _env: Env,
    msg: IbcChannelConnectMsg,
) -> StdResult<IbcBasicResponse> {
    let channel = msg.channel();
    let channel_id = &channel.endpoint.channel_id;

    // save channel to state
    channel_store(deps.storage).save(channel_id)?;

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
    deps: DepsMut,
    _env: Env,
    packet: IbcPacketReceiveMsg,
) -> StdResult<IbcReceiveResponse> {
    let msg: PacketMsg = from_binary(&packet.packet.data)?;

    let mut response = IbcReceiveResponse::new();

    response = match msg {
        PacketMsg::Test {} => response.set_ack(b"test"),
        PacketMsg::Message { value } => {
            receive_store(deps.storage).save(&value)?;
            response
                .set_ack(("recv".to_string() + &value).as_bytes())
                .add_attribute("acknowledging", value)
        }
    };

    Ok(response.add_attribute("action", "ibc_packet_ack"))
}

#[entry_point]
pub fn ibc_packet_ack(
    deps: DepsMut,
    _env: Env,
    msg: IbcPacketAckMsg,
) -> StdResult<IbcBasicResponse> {
    // which local channel was this packet send from
    let caller = msg.original_packet.src.channel_id.clone();
    ack_store(deps.storage).save(&msg.acknowledgement.data.to_base64())?;

    Ok(IbcBasicResponse::new().add_attribute("caller", caller))
}

#[entry_point]
pub fn ibc_packet_timeout(
    deps: DepsMut,
    _env: Env,
    msg: IbcPacketTimeoutMsg,
) -> StdResult<IbcBasicResponse> {
    let timeout_data: String = from_binary(&msg.packet.data)?;
    timeout_store(deps.storage).save(&timeout_data)?;

    Ok(IbcBasicResponse::new().add_attribute("action", "ibc_packet_timeout"))
}
