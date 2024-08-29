use std::env;

fn main () {
    // Generate bindings
    let crate_dir = env::var("CARGO_MANIFEST_DIR").unwrap();

    let generated = cbindgen::generate(crate_dir).expect("Unable to generate bindings");
    generated.write_to_file("./internal/api/bindings.h");

    let is_sim = env::var("SGX_MODE").unwrap_or_else(|_| "HW".to_string());

    // Link enclave and libraries
    println!("cargo:rustc-link-search=native=../sgxvm/sgx-artifacts/lib");
    println!("cargo:rustc-link-lib=static=Enclave_u");

    println!("cargo:rustc-link-search=native=/opt/intel/sgxsdk/lib64");
    println!("cargo:rustc-link-lib=sgx_uprotected_fs");
    
    println!("cargo:rustc-link-lib=dylib=sgx_dcap_ql");
    println!("cargo:rustc-link-lib=dylib=sgx_dcap_quoteverify");
	println!("cargo:rustc-link-lib=dylib=dcap_quoteprov");

    match is_sim.as_ref() {
        "SW" => {
            println!("cargo:rustc-link-lib=dylib=sgx_urts_sim");
            println!("cargo:rustc-link-lib=dylib=sgx_epid_sim");
            println!("cargo:rustc-link-lib=dylib=sgx_quote_ex_sim");
            println!("cargo:rustc-link-lib=dylib=sgx_launch_sim");
        }
        "HW" | _ => {
            println!("cargo:rustc-link-lib=dylib=sgx_urts");
            println!("cargo:rustc-link-lib=dylib=sgx_epid");
            println!("cargo:rustc-link-lib=dylib=sgx_quote_ex");
            println!("cargo:rustc-link-lib=dylib=sgx_launch");
        }
    }
}