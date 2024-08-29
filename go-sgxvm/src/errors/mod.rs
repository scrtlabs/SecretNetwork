mod go;
mod rust;

pub use self::go::GoError;
pub use self::rust::{
    handle_c_error_default, RustError as Error,
};
