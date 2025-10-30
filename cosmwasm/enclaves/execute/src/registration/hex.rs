#![cfg_attr(not(feature = "SGX_MODE_HW"), allow(unused))]

use std::char;

#[allow(unused)]
fn encode_hex_digit(digit: u8) -> char {
    match char::from_digit(digit as u32, 16) {
        Some(c) => c,
        _ => panic!(),
    }
}

#[allow(unused)]
fn encode_hex_byte(byte: u8) -> [char; 2] {
    [encode_hex_digit(byte >> 4), encode_hex_digit(byte & 0x0Fu8)]
}

#[allow(unused)]
pub fn encode_hex(bytes: &[u8]) -> String {
    let strs: Vec<String> = bytes
        .iter()
        .map(|byte| encode_hex_byte(*byte).iter().copied().collect())
        .collect();
    strs.join(" ")
}

#[cfg(test)]
mod test {

    use super::encode_hex;

    #[test]
    fn test_encode_hex() {
        assert_eq!("".to_string(), encode_hex(&[]));
        assert_eq!("00".to_string(), encode_hex(&[0x00]));
        assert_eq!("ab".to_string(), encode_hex(&[0xab]));
        assert_eq!(
            "01 a2 1a fe".to_string(),
            encode_hex(&[0x01, 0xa2, 0x1a, 0xfe])
        );
    }
}
