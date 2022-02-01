fn main() {
    protobuf::cleanup();
    protobuf::build_protobuf_parsers();
}

#[cfg(not(feature = "build-protobuf"))]
mod protobuf {
    pub fn cleanup() {}
    pub fn build_protobuf_parsers() {}
}

#[cfg(feature = "build-protobuf")]
mod protobuf {
    use std::path::PathBuf;

    pub fn cleanup() {
        let src_entries = std::fs::read_dir("src").expect("couldn't read src directory");
        for entry in src_entries {
            let entry = entry.expect("couldn't read src entry");
            if entry.file_name() != "lib.rs" {
                let entry_type = entry.file_type().expect("couldn't read entry type");
                let entry_path = entry.path();
                if entry_type.is_dir() {
                    std::fs::remove_dir_all(&entry_path)
                        .expect(&format!("failed to remove {:?}", entry_path));
                } else if entry_type.is_file() || entry_type.is_symlink() {
                    std::fs::remove_file(&entry_path)
                        .expect(&format!("failed to remove {:?}", entry_path));
                }
            }
        }
    }

    fn from_base(path: &str) -> PathBuf {
        let mut full_path = PathBuf::from("../../../..");
        full_path.push(path);
        full_path
            .canonicalize()
            .expect(&format!("could not canonicalize {:?}", path))
    }

    fn from_cosmos(path: &str) -> PathBuf {
        let mut full_path = PathBuf::from("../../../../third_party/proto/cosmos");
        full_path.push(path);
        full_path
            .canonicalize()
            .expect(&format!("could not canonicalize {:?}", path))
    }

    pub fn build_protobuf_parsers() {
        let protoc_err_msg = "protoc failed to generate protobuf parsers";
        let mut library_dir = dirs::home_dir().unwrap();
        library_dir.extend(&[".local", "include"]);

        let directories: &[(&str, &[PathBuf])] = &[
            ("src/base", &[from_cosmos("base/v1beta1/coin.proto")]),
            (
                "src/crypto/multisig",
                &[
                    from_cosmos("crypto/multisig/keys.proto"),
                    from_cosmos("crypto/multisig/v1beta1/multisig.proto"),
                ],
            ),
            (
                "src/crypto/secp256k1",
                &[from_cosmos("crypto/secp256k1/keys.proto")],
            ),
            (
                "src/crypto/secp256r1",
                &[from_cosmos("crypto/secp256r1/keys.proto")],
            ),
            (
                "src/crypto/ed25519",
                &[from_cosmos("crypto/ed25519/keys.proto")],
            ),
            (
                "src/tx",
                &[
                    from_cosmos("tx/v1beta1/tx.proto"),
                    from_cosmos("tx/signing/v1beta1/signing.proto"),
                ],
            ),
            (
                "src/cosmwasm",
                &[from_base("proto/secret/compute/v1beta1/msg.proto")],
            ),
        ];

        for (out_dir, inputs) in directories {
            let dir_err_msg = format!("failed to generate directory {:?}", out_dir);
            std::fs::create_dir_all(*out_dir).expect(&dir_err_msg);

            protoc_rust::Codegen::new()
                .include(from_base("proto"))
                .include(from_base("third_party/proto"))
                .include(&library_dir) // google types
                .out_dir(*out_dir)
                .inputs(*inputs)
                .run()
                .expect(protoc_err_msg);
        }
    }
}
