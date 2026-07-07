#[cfg(feature = "test")]
pub mod tests {
    use super::sha::sha_256;

    pub fn test_sha_256() {
        // Test vector for SHA-256
        // Input: "abc"
        // Expected output: ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad
        
        let input = b"abc";
        let expected_output = hex::decode("ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad").unwrap();
        
        let result = sha_256(input);
        
        assert_eq!(result.to_vec(), expected_output);
        
        // Test with empty string
        // Expected output: e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
        let empty_input = b"";
        let expected_empty_output = hex::decode("e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855").unwrap();
        
        let empty_result = sha_256(empty_input);
        
        assert_eq!(empty_result.to_vec(), expected_empty_output);
    }
} 
