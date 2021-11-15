fn main() {
    protobuf::build_protobuf_parsers()
}

#[cfg(not(feature = "build-protobuf"))]
mod protobuf {
    pub fn build_protobuf_parsers() {}
}

#[cfg(feature = "build-protobuf")]
mod protobuf {
    pub fn build_protobuf_parsers() {
        let protoc_err_msg = "protoc failed to generate protobuf parsers";
        let mut library_dir = dirs::home_dir().unwrap();
        library_dir.extend(&[".local", "include"]);

        let directories: &[(&str, &[&str])] = &[
            ("src/proto/base", &["proto/cosmos/base/v1beta1/coin.proto"]),
            (
                "src/proto/crypto/multisig",
                &[
                    "proto/cosmos/crypto/multisig/keys.proto",
                    "proto/cosmos/crypto/multisig/v1beta1/multisig.proto",
                ],
            ),
            (
                "src/proto/crypto/secp256k1",
                &["proto/cosmos/crypto/secp256k1/keys.proto"],
            ),
            (
                "src/proto/crypto/secp256r1",
                &["proto/cosmos/crypto/secp256r1/keys.proto"],
            ),
            (
                "src/proto/crypto/ed25519",
                &["proto/cosmos/crypto/ed25519/keys.proto"],
            ),
            (
                "src/proto/tx",
                &[
                    "proto/cosmos/tx/v1beta1/tx.proto",
                    "proto/cosmos/tx/signing/v1beta1/signing.proto",
                ],
            ),
            (
                "src/proto/cosmwasm",
                &["../../../proto/secret/compute/v1beta1/msg.proto"],
            ),
        ];

        for (out_dir, inputs) in directories {
            let dir_err_msg = format!("failed to generate directory {:?}", out_dir);
            std::fs::create_dir_all(*out_dir).expect(&dir_err_msg);

            protoc_rust::Codegen::new()
                .include("../../../proto/secret/compute/v1beta1") // cosmwasm
                .include("proto") // cosmos and gogoproto
                .include(&library_dir) // google types
                .out_dir(*out_dir)
                .inputs(*inputs)
                .run()
                .expect(protoc_err_msg);
        }
    }
}
