#![cfg_attr(not(feature = "SGX_MODE_HW"), allow(unused))]

use log::*;
use sgx_types::sgx_spid_t;
use std::char;

fn decode_hex_digit(digit: char) -> u8 {
    match digit {
        '0'..='9' => digit as u8 - b'0',
        'a'..='f' => digit as u8 - b'a' + 10,
        'A'..='F' => digit as u8 - b'A' + 10,
        _ => panic!(),
    }
}

pub fn decode_spid(hex: &str) -> sgx_spid_t {
    let mut spid = sgx_spid_t::default();
    let hex = hex.trim();

    if hex.len() < 16 * 2 {
        warn!("Input spid file len ({}) is incorrect!", hex.len());
        return spid;
    }

    let decoded_vec = decode_hex(hex);

    spid.id.copy_from_slice(&decoded_vec[..16]);

    spid
}

pub fn decode_hex(hex: &str) -> Vec<u8> {
    let mut r: Vec<u8> = Vec::new();
    let mut chars = hex.chars().enumerate();
    loop {
        let (pos, first) = match chars.next() {
            None => break,
            Some(elt) => elt,
        };
        if first == ' ' {
            continue;
        }
        let (_, second) = match chars.next() {
            None => panic!("pos = {}d", pos),
            Some(elt) => elt,
        };
        r.push((decode_hex_digit(first) << 4) | decode_hex_digit(second));
    }
    r
}

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

// taken from https://github.com/apache/incubator-teaclave-sgx-sdk/blob/master/samplecode/mutual-ra/enclave/src/cert.rs
// isn't actually hex, but I'm sticking it here anyway
pub fn percent_decode(orig: String) -> String {
    let v: Vec<&str> = orig.split('%').collect();
    let mut ret = String::new();
    ret.push_str(v[0]);
    if v.len() > 1 {
        for s in v[1..].iter() {
            ret.push(u8::from_str_radix(&s[0..2], 16).unwrap() as char);
            ret.push_str(&s[2..]);
        }
    }
    ret
}

#[cfg(test)]
mod test {

    use super::decode_hex;
    use super::encode_hex;

    #[test]
    fn test_decode_hex() {
        assert_eq!(decode_hex(""), [].to_vec());
        assert_eq!(decode_hex("00"), [0x00u8].to_vec());
        assert_eq!(decode_hex("ff"), [0xffu8].to_vec());
        assert_eq!(decode_hex("AB"), [0xabu8].to_vec());
        assert_eq!(decode_hex("fa 19"), [0xfau8, 0x19].to_vec());
    }

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
