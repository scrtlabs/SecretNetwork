// Copyright (c) 2019 Oasis Labs Inc. <info@oasislabs.com>
//
// Permission is hereby granted, free of charge, to any person obtaining
// a copy of this software and associated documentation files (the
// "Software"), to deal in the Software without restriction, including
// without limitation the rights to use, copy, modify, merge, publish,
// distribute, sublicense, and/or sell copies of the Software, and to
// permit persons to whom the Software is furnished to do so, subject to
// the following conditions:
//
// The above copyright notice and this permission notice shall be
// included in all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
// EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
// MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
// NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS
// BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN
// ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
// CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

use super::*;

use base64::decode;
use serde_json::{Map, Value};

#[test]
fn test_mrae_basic() {
    let key = [0u8; KEY_SIZE];
    let d2 = DeoxysII::new(&key);

    // Should successfully seal the text.
    let nonce = [1u8; NONCE_SIZE];
    let text = std::string::String::from("This is a test!").as_bytes().to_vec();
    let aad = vec![42; 10];
    let ciphertext = d2.seal(&nonce, text.clone(), aad.clone());

    // Should successfully open the text and the text should match.
    let opened = d2.open(&nonce, ciphertext.clone(), aad.clone());
    assert!(opened.unwrap() == text);

    // Should fail if the nonce is different.
    let fake_nonce = [2u8; NONCE_SIZE];
    let fail_opened = d2.open(&fake_nonce, ciphertext.clone(), aad.clone());
    assert!(fail_opened.is_err());

    // Should fail if the additional data is different.
    let fake_aad = vec![47; 10];
    let fail_opened = d2.open(&nonce, ciphertext.clone(), fake_aad.clone());
    assert!(fail_opened.is_err());

    // Should fail if the both the nonce and the additional data are different.
    let fake_nonce = [3u8; NONCE_SIZE];
    let fake_aad = vec![4; 5];
    let fail_opened = d2.open(&fake_nonce, ciphertext.clone(), fake_aad.clone());
    assert!(fail_opened.is_err());

    // Should handle too short ciphertext.
    let fail_opened = d2.open(&nonce, vec![1, 2, 3], aad.clone());
    assert!(fail_opened.is_err());

    // Should fail on damaged ciphertext.
    let mut malformed_ciphertext = ciphertext.clone();
    malformed_ciphertext[3] ^= 0xa5;
    let fail_opened = d2.open(&nonce, malformed_ciphertext, aad.clone());
    assert!(fail_opened.is_err());

    // Should fail on truncated ciphertext.
    let mut truncated_ciphertext = ciphertext.clone();
    truncated_ciphertext.truncate(ciphertext.len() - 5);
    let fail_opened = d2.open(&nonce, truncated_ciphertext, aad.clone());
    assert!(fail_opened.is_err());
}

#[test]
fn test_mrae_nonblocksized() {
    let key = [42u8; KEY_SIZE];
    let d2 = DeoxysII::new(&key);

    // Should successfully seal msg with non-block-sized additional data.
    let nonce = [7u8; NONCE_SIZE];
    let mut text = Vec::with_capacity(BLOCK_SIZE * 7 + 3);
    for i in 0..text.capacity() {
        text.push(i as u8);
    }
    let mut aad = Vec::with_capacity(BLOCK_SIZE + 5);
    for i in 0..aad.capacity() {
        aad.push(i as u8);
    }
    let ciphertext = d2.seal(&nonce, text.clone(), aad.clone());

    // Should successfully open the text and the text should match.
    let opened = d2.open(&nonce, ciphertext.clone(), aad.clone());
    assert!(opened.unwrap() == text);
}

#[test]
fn test_mrae_blocksized() {
    let key = [42u8; KEY_SIZE];
    let d2 = DeoxysII::new(&key);

    // Should successfully seal msg with block-sized additional data.
    let nonce = [7u8; NONCE_SIZE];
    let mut text = Vec::with_capacity(BLOCK_SIZE * 8);
    for i in 0..text.capacity() {
        text.push(i as u8);
    }
    let mut aad = Vec::with_capacity(BLOCK_SIZE);
    for i in 0..aad.capacity() {
        aad.push(i as u8);
    }
    let ciphertext = d2.seal(&nonce, text.clone(), aad.clone());

    // Should successfully open the text and the text should match.
    let opened = d2.open(&nonce, ciphertext.clone(), aad.clone());
    assert!(opened.unwrap() == text);
}

#[test]
fn test_mrae_vectors() {
    let test_vectors = include_str!("../test-data/Deoxys-II-256-128.json");
    let test_vectors: Map<std::string::String, Value> = serde_json::from_str(test_vectors).unwrap();

    let key_vec = decode(test_vectors["Key"].as_str().unwrap())
        .unwrap()
        .to_vec();
    let msg = decode(test_vectors["MsgData"].as_str().unwrap())
        .unwrap()
        .to_vec();
    let aad = decode(test_vectors["AADData"].as_str().unwrap())
        .unwrap()
        .to_vec();
    let nonce_vec = decode(test_vectors["Nonce"].as_str().unwrap())
        .unwrap()
        .to_vec();

    let mut key = [0u8; KEY_SIZE];
    let mut nonce = [0u8; NONCE_SIZE];
    key.copy_from_slice(&key_vec);
    nonce.copy_from_slice(&nonce_vec);

    let d2 = DeoxysII::new(&key);

    for v in test_vectors["KnownAnswers"].as_array().unwrap().iter() {
        let ciphertext = decode(v["Ciphertext"].as_str().unwrap()).unwrap().to_vec();
        let tag = decode(v["Tag"].as_str().unwrap()).unwrap().to_vec();
        let length: usize = v["Length"].as_u64().unwrap() as usize;

        let ct = d2.seal(&nonce, msg[..length].to_vec(), aad[..length].to_vec());

        assert_eq!(ct.len(), length + TAG_SIZE);

        let t = ct[length..].to_vec();
        let ct = ct[..length].to_vec();

        assert_eq!(ciphertext, ct);
        assert_eq!(tag, t);
    }
}
