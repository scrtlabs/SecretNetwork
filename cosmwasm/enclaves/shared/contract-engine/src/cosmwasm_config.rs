/// api_marker is based on this compatibility chart:
/// https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/vm/README.md#compatibility
pub mod api_marker {
    pub const V0_10: &str = "cosmwasm_vm_version_3";
    pub const V1: &str = "interface_version_8";
}

pub mod features {
    pub const RANDOM: &str = "requires_random";
}

/// Right now ContractOperation is used to detect queris and prevent state changes
#[derive(Clone, Copy, Debug)]
pub enum ContractOperation {
    Init,
    Handle,
    Query,
    Migrate,
}

#[allow(unused)]
impl ContractOperation {
    pub fn is_init(&self) -> bool {
        matches!(self, ContractOperation::Init)
    }

    pub fn is_handle(&self) -> bool {
        matches!(self, ContractOperation::Handle)
    }

    pub fn is_query(&self) -> bool {
        matches!(self, ContractOperation::Query)
    }

    pub fn is_migrate(&self) -> bool {
        matches!(self, ContractOperation::Migrate)
    }
}

//pub const MAX_LOG_LENGTH: usize = 8192;
