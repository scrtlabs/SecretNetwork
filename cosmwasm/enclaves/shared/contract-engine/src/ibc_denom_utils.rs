use sha2::{Digest, Sha256};

/// ReceiverChainIsSource returns true if the denomination originally came
/// from the receiving chain and false otherwise.
pub fn receiver_chain_is_source(source_port: &str, source_channel: &str, denom: &str) -> bool {
    // The prefix passed in should contain the SourcePort and SourceChannel.
    // If the receiver chain originally sent the token to the sender chain
    // the denom will have the sender's SourcePort and SourceChannel as the
    // prefix.
    let voucher_prefix = get_denom_prefix(source_port, source_channel);
    denom.starts_with(&voucher_prefix)
}

/// GetDenomPrefix returns the receiving denomination prefix
pub fn get_denom_prefix(port_id: &str, channel_id: &str) -> String {
    format!("{}/{}/", port_id, channel_id)
}

/// DenomTrace contains the base denomination for ICS20 fungible tokens and the
/// source tracing information path.
pub struct DenomTrace {
    /// path defines the chain of port/channel identifiers used for tracing the
    /// source of the fungible token.
    pub path: String,
    /// base denomination of the relayed fungible token.
    pub base_denom: String,
}

impl DenomTrace {
    /// Hash returns the hex bytes of the SHA256 hash of the DenomTrace fields using the following formula:
    /// hash = sha256(tracePath + "/" + baseDenom)
    pub fn hash(&self) -> Vec<u8> {
        let hash = Sha256::digest(self.get_full_denom_path().as_bytes());
        hash.to_vec()
    }

    /// IBCDenom a coin denomination for an ICS20 fungible token in the format
    /// 'ibc/{hash(tracePath + baseDenom)}'. If the trace is empty, it will return the base denomination.
    pub fn ibc_denom(&self) -> String {
        if !self.path.is_empty() {
            format!("ibc/{}", hex::encode(self.hash()))
        } else {
            self.base_denom.clone()
        }
    }

    /// GetFullDenomPath returns the full denomination according to the ICS20 specification:
    /// tracePath + "/" + baseDenom
    /// If there exists no trace then the base denomination is returned.
    pub fn get_full_denom_path(&self) -> String {
        if self.path.is_empty() {
            self.base_denom.clone()
        } else {
            self.get_prefix() + &self.base_denom
        }
    }

    // GetPrefix returns the receiving denomination prefix composed by the trace info and a separator.
    pub fn get_prefix(&self) -> String {
        format!("{}/", self.path)
    }
}

/// ParseDenomTrace parses a string with the ibc prefix (denom trace) and the base denomination
/// into a DenomTrace type.
///
/// Examples:
///
/// - "portidone/channel-0/uatom" => DenomTrace{Path: "portidone/channel-0", BaseDenom: "uatom"}
/// - "portidone/channel-0/portidtwo/channel-1/uatom" => DenomTrace{Path: "portidone/channel-0/portidtwo/channel-1", BaseDenom: "uatom"}
/// - "portidone/channel-0/gamm/pool/1" => DenomTrace{Path: "portidone/channel-0", BaseDenom: "gamm/pool/1"}
/// - "gamm/pool/1" => DenomTrace{Path: "", BaseDenom: "gamm/pool/1"}
/// - "uatom" => DenomTrace{Path: "", BaseDenom: "uatom"}
pub fn parse_denom_trace(raw_denom: &str) -> DenomTrace {
    let denom_split: Vec<&str> = raw_denom.split('/').collect();

    if denom_split.len() == 1 {
        return DenomTrace {
            path: "".to_string(),
            base_denom: raw_denom.to_string(),
        };
    }

    let (path, base_denom) = extract_path_and_base_from_full_denom(&denom_split);

    DenomTrace { path, base_denom }
}

/// extract_path_and_base_from_full_denom returns the trace path and the base denom from
/// the elements that constitute the complete denom.
pub fn extract_path_and_base_from_full_denom(full_denom_items: &[&str]) -> (String, String) {
    let mut path = vec![];
    let mut base_denom = vec![];

    let length = full_denom_items.len();
    let mut i = 0;

    while i < length {
        // The IBC specification does not guarantee the expected format of the
        // destination port or destination channel identifier. A short term solution
        // to determine base denomination is to expect the channel identifier to be the
        // one ibc-go specifies. A longer term solution is to separate the path and base
        // denomination in the ICS20 packet. If an intermediate hop prefixes the full denom
        // with a channel identifier format different from our own, the base denomination
        // will be incorrectly parsed, but the token will continue to be treated correctly
        // as an IBC denomination. The hash used to store the token internally on our chain
        // will be the same value as the base denomination being correctly parsed.
        if i < length - 1 && length > 2 && is_valid_channel_id(full_denom_items[i + 1]) {
            path.push(full_denom_items[i].to_owned());
            path.push(full_denom_items[i + 1].to_owned());
            i += 2;
        } else {
            base_denom = full_denom_items[i..].to_vec();
            break;
        }
    }

    (path.join("/"), base_denom.join("/"))
}

/// IsValidChannelID checks if a channelID is valid and can be parsed to the channel
/// identifier format.
pub fn is_valid_channel_id(channel_id: &str) -> bool {
    parse_channel_sequence(channel_id).is_some()
}

/// ParseChannelSequence parses the channel sequence from the channel identifier.
pub fn parse_channel_sequence(channel_id: &str) -> Option<&str> {
    channel_id.strip_prefix("channel-")
}
