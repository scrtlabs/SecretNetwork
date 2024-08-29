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

/// Macro to generate the constant vectors for conditioning the sub-tweak
/// keys for each round.
macro_rules! generate_rcon_matrix {
    ( $( $x:expr ),* ) => {
        [$(m128i_vec![1, 2, 4, 8, $x, $x, $x, $x, 0, 0, 0, 0, 0, 0, 0, 0],)*]
    };
}

/// Vectors from the generated RCON matrix are used when deriving partial
/// sub-tweak keys from the actual key (see `stk_derive_k()`).
const RCON: [__m128i; STK_COUNT] = generate_rcon_matrix![
    0x2f, 0x5e, 0xbc, 0x63, 0xc6, 0x97, 0x35, 0x6a, 0xd4, 0xb3, 0x7d, 0xfa, 0xef, 0xc5, 0x91, 0x39,
    0x72
];

/// Derives the K component of the sub-tweak key (STK) for each round.
/// The derived partial STK is passed to seal/open instead of the actual
/// key.
fn stk_derive_k(key: &[u8; KEY_SIZE]) -> [[u8; STK_SIZE]; STK_COUNT] {
    debug_assert!(STK_SIZE == BLOCK_SIZE);

    unsafe {
        #[repr(align(16))]
        struct DKS([[u8; STK_SIZE]; STK_COUNT]);
        let mut derived_ks = DKS([[0u8; STK_SIZE]; STK_COUNT]);

        // LFSR masks for the vector bitops.
        let lfsr_x0_mask = _mm_set1_epi8(1);
        let lfsr_invx0_mask = _mm_set1_epi8(-2); // 0xfe
        let lfsr_x7_mask = _mm_set1_epi8(-128); // 0x80
        let lfsr_invx7_mask = _mm_set1_epi8(127); // 0x7f

        let mut tk2 = _mm_loadu_si128(key[16..32].as_ptr() as *const __m128i);
        let mut tk3 = _mm_loadu_si128(key[0..16].as_ptr() as *const __m128i);

        // First iteration.
        let mut dk0 = _mm_xor_si128(tk2, tk3);
        dk0 = _mm_xor_si128(dk0, RCON[0]);
        _mm_store_si128(derived_ks.0[0].as_mut_ptr() as *mut __m128i, dk0);

        // Remaining iterations.
        for i in 1..ROUNDS + 1 {
            // Tk2(i+1) = h(LFSR2(Tk2(i)))
            let x1sr7 = _mm_srli_epi64(tk2, 7);
            let x1sr5 = _mm_srli_epi64(tk2, 5);
            tk2 = _mm_slli_epi64(tk2, 1);
            tk2 = _mm_and_si128(tk2, lfsr_invx0_mask);
            let x7xorx5 = _mm_xor_si128(x1sr7, x1sr5);
            let x7xorx5_and_1 = _mm_and_si128(x7xorx5, lfsr_x0_mask);
            tk2 = _mm_or_si128(tk2, x7xorx5_and_1);

            tk2 = _mm_shuffle_epi8(tk2, H_SHUFFLE);

            // Tk3(i+1) = h(LFSR3(Tk3(i)))
            let x2sl7 = _mm_slli_epi64(tk3, 7);
            let x2sl1 = _mm_slli_epi64(tk3, 1);
            tk3 = _mm_srli_epi64(tk3, 1);
            tk3 = _mm_and_si128(tk3, lfsr_invx7_mask);
            let x7xorx1 = _mm_xor_si128(x2sl7, x2sl1);
            let x7xorx1_and_1 = _mm_and_si128(x7xorx1, lfsr_x7_mask);
            tk3 = _mm_or_si128(tk3, x7xorx1_and_1);

            tk3 = _mm_shuffle_epi8(tk3, H_SHUFFLE);

            let mut dki = _mm_xor_si128(tk2, tk3);
            dki = _mm_xor_si128(dki, RCON[i]);

            _mm_store_si128(derived_ks.0[i].as_mut_ptr() as *mut __m128i, dki);
        }

        sanitize_xmm_registers();

        derived_ks.0
    }
}

/// Performs block encryption using the block cipher in-place.
#[inline]
fn bc_encrypt_in_place(
    block: &mut [u8; BLOCK_SIZE],
    derived_ks: &[[u8; STK_SIZE]; STK_COUNT], // MUST be 16 byte aligned.
    tweak: &[u8; TWEAK_SIZE],
) {
    unsafe {
        // First iteration: load plaintext, derive first sub-tweak key, then
        // xor it with the plaintext.
        let pt = _mm_loadu_si128(block.as_ptr() as *const __m128i);
        let dk0 = _mm_load_si128(derived_ks[0].as_ptr() as *const __m128i);
        let mut tk1 = _mm_loadu_si128(tweak.as_ptr() as *const __m128i);
        let stk1 = _mm_xor_si128(dk0, tk1);
        let mut ct = _mm_xor_si128(pt, stk1);

        // Remaining iterations.
        for i in 1..ROUNDS + 1 {
            // Derive sub-tweak key for this round.
            tk1 = _mm_shuffle_epi8(tk1, H_SHUFFLE);
            let dki = _mm_load_si128(derived_ks[i].as_ptr() as *const __m128i);

            // Perform AESENC on the block.
            ct = _mm_aesenc_si128(ct, _mm_xor_si128(dki, tk1));
        }

        _mm_storeu_si128(block.as_mut_ptr() as *mut __m128i, ct);
    }
}

#[inline(always)]
fn or_block_num(block: __m128i, block_num: usize) -> __m128i {
    unsafe {
        let bnum = _mm_set_epi64x(0, block_num as i64);
        let bnum_be = _mm_shuffle_epi8(bnum, LE2BE_SHUFFLE);
        _mm_or_si128(bnum_be, block)
    }
}

#[inline(always)]
fn xor_block_num(block: __m128i, block_num: usize) -> __m128i {
    unsafe {
        let bnum = _mm_set_epi64x(0, block_num as i64);
        let bnum_be = _mm_shuffle_epi8(bnum, LE2BE_SHUFFLE);
        _mm_xor_si128(bnum_be, block)
    }
}

#[inline]
fn accumulate_blocks(
    tag: &mut [u8; BLOCK_SIZE],
    derived_ks: &[[u8; STK_SIZE]; STK_COUNT], // MUST be 16 byte aligned.
    prefix: u8,
    block_num: usize,
    plaintext: &[u8],
    nr_blocks: usize,
) {
    debug_assert!(plaintext.len() >= BLOCK_SIZE * nr_blocks);

    let mut n = nr_blocks;
    let mut i = 0usize;

    unsafe {
        let mut t = _mm_loadu_si128(tag.as_ptr() as *const __m128i);
        let p = (prefix << PREFIX_SHIFT) as i8;
        let xp = _mm_set_epi8(0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, p);

        while n >= 4 {
            let mut tweak0 = or_block_num(xp, i + block_num);
            let mut tweak1 = or_block_num(xp, i + block_num + 1);
            let mut tweak2 = or_block_num(xp, i + block_num + 2);
            let mut tweak3 = or_block_num(xp, i + block_num + 3);

            let pt0 = _mm_loadu_si128(plaintext[i * BLOCK_SIZE..].as_ptr() as *const __m128i);
            let pt1 = _mm_loadu_si128(plaintext[(i + 1) * BLOCK_SIZE..].as_ptr() as *const __m128i);
            let pt2 = _mm_loadu_si128(plaintext[(i + 2) * BLOCK_SIZE..].as_ptr() as *const __m128i);
            let pt3 = _mm_loadu_si128(plaintext[(i + 3) * BLOCK_SIZE..].as_ptr() as *const __m128i);

            let dk = _mm_load_si128(derived_ks[0].as_ptr() as *const __m128i);
            let mut ct0 = _mm_xor_si128(pt0, _mm_xor_si128(dk, tweak0));
            let mut ct1 = _mm_xor_si128(pt1, _mm_xor_si128(dk, tweak1));
            let mut ct2 = _mm_xor_si128(pt2, _mm_xor_si128(dk, tweak2));
            let mut ct3 = _mm_xor_si128(pt3, _mm_xor_si128(dk, tweak3));

            for j in 1..ROUNDS + 1 {
                tweak0 = _mm_shuffle_epi8(tweak0, H_SHUFFLE);
                tweak1 = _mm_shuffle_epi8(tweak1, H_SHUFFLE);
                tweak2 = _mm_shuffle_epi8(tweak2, H_SHUFFLE);
                tweak3 = _mm_shuffle_epi8(tweak3, H_SHUFFLE);

                let dk = _mm_load_si128(derived_ks[j].as_ptr() as *const __m128i);
                ct0 = _mm_aesenc_si128(ct0, _mm_xor_si128(dk, tweak0));
                ct1 = _mm_aesenc_si128(ct1, _mm_xor_si128(dk, tweak1));
                ct2 = _mm_aesenc_si128(ct2, _mm_xor_si128(dk, tweak2));
                ct3 = _mm_aesenc_si128(ct3, _mm_xor_si128(dk, tweak3));
            }

            t = _mm_xor_si128(ct0, t);
            t = _mm_xor_si128(ct1, t);
            t = _mm_xor_si128(ct2, t);
            t = _mm_xor_si128(ct3, t);

            i += 4;
            n -= 4;
        }

        while n > 0 {
            let mut tweak = or_block_num(xp, i + block_num);
            let pt = _mm_loadu_si128(plaintext[i * BLOCK_SIZE..].as_ptr() as *const __m128i);

            let dk = _mm_load_si128(derived_ks[0].as_ptr() as *const __m128i);
            let mut ct = _mm_xor_si128(pt, _mm_xor_si128(dk, tweak));

            for j in 1..ROUNDS + 1 {
                tweak = _mm_shuffle_epi8(tweak, H_SHUFFLE);

                let dk = _mm_load_si128(derived_ks[j].as_ptr() as *const __m128i);
                ct = _mm_aesenc_si128(ct, _mm_xor_si128(dk, tweak));
            }

            t = _mm_xor_si128(ct, t);

            i += 1;
            n -= 1;
        }

        _mm_storeu_si128(tag.as_mut_ptr() as *mut __m128i, t);
    }
}

#[inline]
fn bc_xor_blocks(
    ciphertext: &mut [u8],
    derived_ks: &[[u8; STK_SIZE]; STK_COUNT], // MUST be 16 byte aligned.
    tag: &[u8; BLOCK_SIZE],
    block_num: usize,
    nonce: &[u8; BLOCK_SIZE],
    plaintext: &[u8],
    nr_blocks: usize,
) {
    debug_assert!(plaintext.len() == ciphertext.len());
    debug_assert!(plaintext.len() >= BLOCK_SIZE * nr_blocks);

    let mut n = nr_blocks;
    let mut i: usize = 0;

    unsafe {
        let xtag = _mm_loadu_si128(tag.as_ptr() as *const __m128i);
        let xnonce = _mm_loadu_si128(nonce.as_ptr() as *const __m128i);

        while n >= 4 {
            let mut tweak0 = xor_block_num(xtag, i + block_num);
            let mut tweak1 = xor_block_num(xtag, i + block_num + 1);
            let mut tweak2 = xor_block_num(xtag, i + block_num + 2);
            let mut tweak3 = xor_block_num(xtag, i + block_num + 3);

            let dk = _mm_load_si128(derived_ks[0].as_ptr() as *const __m128i);
            let mut ks0 = _mm_xor_si128(xnonce, _mm_xor_si128(dk, tweak0));
            let mut ks1 = _mm_xor_si128(xnonce, _mm_xor_si128(dk, tweak1));
            let mut ks2 = _mm_xor_si128(xnonce, _mm_xor_si128(dk, tweak2));
            let mut ks3 = _mm_xor_si128(xnonce, _mm_xor_si128(dk, tweak3));

            for j in 1..ROUNDS + 1 {
                tweak0 = _mm_shuffle_epi8(tweak0, H_SHUFFLE);
                tweak1 = _mm_shuffle_epi8(tweak1, H_SHUFFLE);
                tweak2 = _mm_shuffle_epi8(tweak2, H_SHUFFLE);
                tweak3 = _mm_shuffle_epi8(tweak3, H_SHUFFLE);

                let dk = _mm_load_si128(derived_ks[j].as_ptr() as *const __m128i);
                ks0 = _mm_aesenc_si128(ks0, _mm_xor_si128(dk, tweak0));
                ks1 = _mm_aesenc_si128(ks1, _mm_xor_si128(dk, tweak1));
                ks2 = _mm_aesenc_si128(ks2, _mm_xor_si128(dk, tweak2));
                ks3 = _mm_aesenc_si128(ks3, _mm_xor_si128(dk, tweak3));
            }

            let pt0 = _mm_loadu_si128(plaintext[i * BLOCK_SIZE..].as_ptr() as *const __m128i);
            let pt1 = _mm_loadu_si128(plaintext[(i + 1) * BLOCK_SIZE..].as_ptr() as *const __m128i);
            let pt2 = _mm_loadu_si128(plaintext[(i + 2) * BLOCK_SIZE..].as_ptr() as *const __m128i);
            let pt3 = _mm_loadu_si128(plaintext[(i + 3) * BLOCK_SIZE..].as_ptr() as *const __m128i);
            _mm_storeu_si128(
                ciphertext[i * BLOCK_SIZE..].as_ptr() as *mut __m128i,
                _mm_xor_si128(pt0, ks0),
            );
            _mm_storeu_si128(
                ciphertext[(i + 1) * BLOCK_SIZE..].as_ptr() as *mut __m128i,
                _mm_xor_si128(pt1, ks1),
            );
            _mm_storeu_si128(
                ciphertext[(i + 2) * BLOCK_SIZE..].as_ptr() as *mut __m128i,
                _mm_xor_si128(pt2, ks2),
            );
            _mm_storeu_si128(
                ciphertext[(i + 3) * BLOCK_SIZE..].as_ptr() as *mut __m128i,
                _mm_xor_si128(pt3, ks3),
            );

            i += 4;
            n -= 4;
        }

        while n > 0 {
            let mut tweak = xor_block_num(xtag, i + block_num);

            let dk = _mm_load_si128(derived_ks[0].as_ptr() as *const __m128i);
            let mut ks = _mm_xor_si128(xnonce, _mm_xor_si128(dk, tweak));

            for j in 1..ROUNDS + 1 {
                tweak = _mm_shuffle_epi8(tweak, H_SHUFFLE);

                let dk = _mm_load_si128(derived_ks[j].as_ptr() as *const __m128i);
                ks = _mm_aesenc_si128(ks, _mm_xor_si128(dk, tweak));
            }

            let pt = _mm_loadu_si128(plaintext[i * BLOCK_SIZE..].as_ptr() as *const __m128i);
            _mm_storeu_si128(
                ciphertext[i * BLOCK_SIZE..].as_ptr() as *mut __m128i,
                _mm_xor_si128(pt, ks),
            );

            i += 1;
            n -= 1;
        }
    }
}

#[inline(always)]
fn sanitize_xmm_registers() {
    unsafe {
        // This is overly heavy handed, but the downside to using intrinsics
        // is that there's no way to tell which registers end up with sensitive
        // key material.
        std::arch::asm!(
            "
            pxor xmm0, xmm0
            pxor xmm1, xmm1
            pxor xmm2, xmm2
            pxor xmm3, xmm3
            pxor xmm4, xmm4
            pxor xmm5, xmm5
            pxor xmm6, xmm6
            pxor xmm7, xmm7
            pxor xmm8, xmm8
            pxor xmm9, xmm9
            pxor xmm10, xmm10
            pxor xmm11, xmm11
            pxor xmm12, xmm12
            pxor xmm13, xmm13
            pxor xmm14, xmm14
            pxor xmm15, xmm15
            ",
            lateout("xmm0") _,
            lateout("xmm1") _,
            lateout("xmm2") _,
            lateout("xmm3") _,
            lateout("xmm4") _,
            lateout("xmm5") _,
            lateout("xmm6") _,
            lateout("xmm7") _,
            lateout("xmm8") _,
            lateout("xmm9") _,
            lateout("xmm10") _,
            lateout("xmm11") _,
            lateout("xmm12") _,
            lateout("xmm13") _,
            lateout("xmm14") _,
            lateout("xmm15") _,
            options(nostack)
        );
    }
}
