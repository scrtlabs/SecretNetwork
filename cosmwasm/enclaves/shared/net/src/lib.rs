pub(crate) mod consts;
mod error;
pub mod http;
pub(crate) mod socket;
pub mod tls;
pub mod endpoints;

#[cfg(all(not(target_env = "sgx"), not(test)))]
#[macro_use]
extern crate sgx_tstd as std;

extern crate sgx_types;

#[cfg(test)]
mod tests {
    #[test]
    fn it_works() {
        let result = 2 + 2;
        assert_eq!(result, 4);
    }
}
