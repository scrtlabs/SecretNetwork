#[derive(Debug)]
pub enum AttestationError {
    IntelCommunication,
    SSSCommunication,
    HttpResponse,
    SeedFetch,
    SeedParsing,
}
