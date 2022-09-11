fn main() {
    println!("cargo:rerun-if-changed=./src/contract.rs");
}
