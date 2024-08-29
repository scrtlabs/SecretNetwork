use deoxysii::{DeoxysII, KEY_SIZE, NONCE_SIZE};

use criterion::{black_box, criterion_group, criterion_main, Bencher, Criterion};
use rand::{rngs::OsRng, RngCore};

fn mrae_seal_4096(b: &mut Bencher) {
    let mut rng = OsRng;

    // Set up the key.
    let mut key = [0u8; KEY_SIZE];
    rng.fill_bytes(&mut key);
    let d2 = DeoxysII::new(&key);

    // Set up the payload.
    let mut nonce = [0u8; NONCE_SIZE];
    rng.fill_bytes(&mut nonce);
    let mut text = [0u8; 4096];
    rng.fill_bytes(&mut text);
    let mut aad = [0u8; 64];
    rng.fill_bytes(&mut aad);

    // Benchmark sealing.
    b.iter(|| {
        let text = text.to_vec();
        let aad = aad.to_vec();
        let _sealed = black_box(d2.seal(&nonce, text, aad));
    });
}

fn mrae_open_4096(b: &mut Bencher) {
    let mut rng = OsRng;

    // Set up the key.
    let mut key = [0u8; KEY_SIZE];
    rng.fill_bytes(&mut key);
    let d2 = DeoxysII::new(&key);

    // Set up the payload.
    let mut nonce = [0u8; NONCE_SIZE];
    rng.fill_bytes(&mut nonce);
    let mut text = [0u8; 4096];
    rng.fill_bytes(&mut text);
    let mut aad = [0u8; 64];
    rng.fill_bytes(&mut aad);

    // Seal the payload.
    let ciphertext = d2.seal(&nonce, text.to_vec(), aad.to_vec());

    // Benchmark opening.
    b.iter(|| {
        let ct = ciphertext.to_vec();
        let aad = aad.to_vec();
        let _opened = black_box(d2.open(&nonce, ct, aad));
    });
}

fn mrae_new(b: &mut Bencher) {
    let mut rng = OsRng;

    // Set up the key.
    let mut key = [0u8; KEY_SIZE];
    rng.fill_bytes(&mut key);

    b.iter(|| {
        let _d2 = black_box(DeoxysII::new(&key));
    });
}

fn criterion_benchmark(c: &mut Criterion) {
    c.bench_function("mrae_open_4096", mrae_open_4096);
    c.bench_function("mrae_seal_4096", mrae_seal_4096);
    c.bench_function("mrae_new", mrae_new);
}

criterion_group!(benches, criterion_benchmark);
criterion_main!(benches);
