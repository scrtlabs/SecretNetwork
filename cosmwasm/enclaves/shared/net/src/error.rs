#[derive(Debug)]
pub enum NetError {
    SocketCreateFailed,
    IPv4LookupError,
    BadHttpResponse,
    MalformedHttpHeaders,
    BodyNotUtf8InResponse,
    Base64DecodeError,
    InvalidDnsName,
}
