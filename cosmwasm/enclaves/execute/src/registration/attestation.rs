use core::{mem, slice};

use base64ct::Encoding;
use enclave_crypto::dcap::verify_quote_any;
use log::*;
use rsa::signature::Verifier;
use serde_json::Value;
use sha2::{Digest, Sha256};
use std::collections::{HashMap, HashSet};
use std::convert::TryFrom;
use std::io::Write;
use std::sync::SgxMutex;
use std::untrusted::fs::File;
use std::vec::Vec;

#[cfg(feature = "SGX_MODE_HW")]
use sgx_tse::rsgx_create_report;

use sgx_types::{
    sgx_ql_auth_data_t, sgx_ql_certification_data_t, sgx_ql_ecdsa_sig_data_t, sgx_ql_qv_result_t,
    sgx_quote_t, sgx_report_body_t, sgx_status_t,
};

#[cfg(feature = "SGX_MODE_HW")]
use sgx_types::{sgx_report_data_t, sgx_report_t, sgx_target_info_t};

#[cfg(feature = "SGX_MODE_HW")]
use std::{str, string::String};

#[cfg(all(feature = "SGX_MODE_HW", feature = "production"))]
use crate::registration::offchain::get_attestation_report_dcap;

#[cfg(all(feature = "SGX_MODE_HW", feature = "production"))]
use enclave_crypto::consts::*;

#[cfg(all(feature = "SGX_MODE_HW", feature = "production"))]
use std::sgxfs::remove as SgxFsRemove;

#[cfg(feature = "SGX_MODE_HW")]
use super::ocalls::{
    ocall_get_quote_ecdsa, ocall_get_quote_ecdsa_collateral, ocall_get_quote_ecdsa_params,
};

use ::hex as orig_hex;

#[cfg(all(feature = "SGX_MODE_HW", feature = "production"))]
pub fn validate_enclave_version(kp: &enclave_crypto::KeyPair) -> Result<(), sgx_status_t> {
    let res_dcap = unsafe { get_attestation_report_dcap(&kp.get_pubkey()) };

    match res_dcap {
        Ok(_) => Ok(()),
        Err(e) => Err(e),
    }
}

#[cfg(all(feature = "SGX_MODE_HW", feature = "production"))]
fn remove_secret_file(file_name: &str) {
    let _ = SgxFsRemove(make_sgx_secret_path(file_name));
}

#[cfg(all(feature = "SGX_MODE_HW", feature = "production"))]
#[allow(dead_code)]
fn remove_all_keys() {
    remove_secret_file(&SEALED_FILE_UNITED);
    remove_secret_file(SEALED_FILE_REGISTRATION_KEY);
    remove_secret_file(SEALED_FILE_ENCRYPTED_SEED_KEY_GENESIS);
    remove_secret_file(SEALED_FILE_ENCRYPTED_SEED_KEY_CURRENT);
    remove_secret_file(SEALED_FILE_IRS);
    remove_secret_file(SEALED_FILE_REK);
    remove_secret_file(SEALED_FILE_TX_BYTES);
}

#[cfg(feature = "SGX_MODE_HW")]
#[allow(dead_code)]
pub fn in_grace_period(timestamp: u64) -> bool {
    // Friday, August 21, 2023 2:00:00 PM UTC
    timestamp < 1692626400_u64
}

pub struct KnownJwtKeys {
    pub coll: HashMap<Vec<u8>, rsa::pkcs1v15::VerifyingKey<Sha256>>,
}

fn my_decode_base64(enc: &str) -> Result<Vec<u8>, Box<dyn std::error::Error>> {
    match base64ct::Base64UrlUnpadded::decode_vec(enc) {
        Ok(x) => Ok(x),
        Err(e) => Err(format!("base64 decode failed: {:?}", e).into()),
    }
}

impl KnownJwtKeys {
    fn add_key(&mut self, kid_b64: &str, n_b64: &str) -> Result<(), Box<dyn std::error::Error>> {
        let kid_bytes = base64::decode(kid_b64)?;
        let n_bytes = my_decode_base64(n_b64)?;
        let e_bytes = [1_u8, 00_u8, 1_u8];

        // 2️⃣ Construct RSA public key
        let n = rsa::BigUint::from_bytes_be(&n_bytes);
        let e = rsa::BigUint::from_bytes_be(&e_bytes);
        let pubkey = rsa::RsaPublicKey::new(n, e).map_err(|_| "invalid RSA key components")?;

        let verifying_key = rsa::pkcs1v15::VerifyingKey::<Sha256>::new(pubkey);
        self.coll.insert(kid_bytes, verifying_key);

        Ok(())
    }
}

pub mod allow_list {

    use enclave_utils::KEY_MANAGER;
    use log::*;
    use std::collections::HashMap;

    use crate::registration::attestation::SELF_MACHINE_ID;

    pub const MACHINE_ID_LEN: usize = 20;
    pub const OWNER_LEN: usize = 32;

    pub type MachineID = [u8; MACHINE_ID_LEN];
    pub type Owner = [u8; OWNER_LEN];

    pub struct Data {
        pub m_to_o: HashMap<MachineID, Owner>,
    }

    impl Data {
        fn log_machine_change(machine: &MachineID, owner: &Owner) {
            println!(
                "machine {} owner set to {}",
                hex::encode(machine),
                hex::encode(owner)
            );
        }

        fn on_machine_changed(machine: &MachineID, added: bool, silent: bool) {
            if !silent {
                let action = if added { "added" } else { "deleted" };
                println!("machine {} {}", hex::encode(machine), action);
            }

            if let Some(my_machine) = SELF_MACHINE_ID.as_ref() {
                if my_machine == machine {
                    println!("Self machine included: {}", added);

                    let mut extra = KEY_MANAGER.extra_data.lock().unwrap();
                    extra.machine_allowed = added;
                }
            }
        }

        pub fn update(
            &mut self,
            machine: &MachineID,
            owner: &Owner,
            machine_pop: &MachineID,
        ) -> bool {
            let is_same_machine =
                (*machine_pop == [0u8; MACHINE_ID_LEN]) || (*machine_pop == *machine);
            if is_same_machine {
                let x = match self.m_to_o.get_mut(machine) {
                    Some(x) => x,
                    None => {
                        error!("unknown machine {}", hex::encode(machine));
                        return false;
                    }
                };

                if x == owner {
                    return false; // no error, just no effect
                }

                *x = *owner; // replace the owner of
            } else {
                if let Some(x) = self.m_to_o.get(machine_pop) {
                    if *x != *owner {
                        error!(
                            "unknown machine {} not owned by this actor",
                            hex::encode(machine_pop)
                        );
                        return false;
                    }
                } else {
                    error!("unknown machine {}", hex::encode(machine_pop));
                    return false;
                }

                if self.m_to_o.contains_key(machine) {
                    error!("machine {} already exists", hex::encode(machine));
                    return false;
                }

                self.m_to_o.remove(machine_pop);
                self.m_to_o.insert(*machine, *owner);

                Self::on_machine_changed(machine_pop, false, false);
                Self::on_machine_changed(machine, true, false);
            }

            Self::log_machine_change(machine, owner);
            true
        }

        pub fn add_new(&mut self, machine: MachineID, silent: bool) -> bool {
            if let std::collections::hash_map::Entry::Vacant(e) = self.m_to_o.entry(machine) {
                e.insert([0u8; OWNER_LEN]);
                Self::on_machine_changed(&machine, true, silent);
                true
            } else {
                false
            }
        }
    }
}

lazy_static::lazy_static! {

    static ref KNOWN_JWT_KEYS: KnownJwtKeys  = {
        let mut keys = KnownJwtKeys {
            coll: HashMap::new()
        };

        // https://sharedeus2.eus2.attest.azure.net/certs
        keys.add_key("J0pAPdfXXHqWWimgrH853wMIdh5/fLe1z6uSXYPXCa0=", "2NBrEQdwXUzVy2p-SZ7sBjxbVd4iTGNEQJu_Ot_C0NCzXIDT6DMEAeVZLSoWWcW6oXQ81h-yQWtw-jFW_SPgG4FGSL1UnVO8Zak80thovQk0dbZDo-9lsoOnOfXfPUL0T9AgHtqJpUr3tCfyRRLdC0MgF1tAyjZbMj8bHe2ZmJ9GLTJT5v9E0i5l3S4WZY52vMzZaVpfxw-0_s5tRzcoPGqIrMOnX_7kv5j7sisqZKNq6fP-4MHvLb_tXyHCkW6FzX8mUlwyRNzBP3R4xaXBvykzJMaAiCW_Yr_TxycdnmwsTR7he1Q78q12KnYqLvUVjg_v39_RWGSbFnaP1YX5Hw").unwrap();
        keys.add_key("ZOub53V4dZuruzP0dOOH0axfyks=", "svsohLQjA_aPyQ7EE9ZJYNYtNZ3QlFDMlVBNjurKl1r3WlM9089GMP1oc6pirai20_MuBISvmd-RDH4vLelHEfS9GrMROG-B0OrdB5mB8XlcA9ErN2_ztsqlhG28m3LTsAhMf8guhFR2-78ukOVhH0lJYFtpG9wbE0aCoBxXpSVS7JR_Otadv00EskIUoZkjx0YX94NVE7-fHMS6DD4TWEOng17mNAKELJbwdHgA5DyQsFMW7mVJVK-1BHQ5m14wYWeLadEGnzVgrc1T_x2-VJrAbSai9_xhrbZ7-RsCCTuQs0az2akbQyfb-zlyywNPIl8JO8_9j4JL8zdWNBiRAQ").unwrap();
        keys.add_key("frVjcejaF7GhoPNgEN9FdDQp7/Iwlpl/Ug8/TMh/3eg=", "2WxgwBuJY-wZ_9rmTtxIHXzeqJc0qo72Ft6MYog9qY3K6ZgGmmii_pi1FvwQP43bgXyOALtbWzcmMp5gl3prnZeiNmnb_5bHys1C-0bBWO0z6NhpBevfbejmiBc63WuWl5ZEZF29hzkQyoHm-25NyYpqBtPw2469tvJoxhCng5u_tYLpI2qJljQazxyWcMj2LdTxr_LNBLPR5Naz8DgWPS5xEs3QTzoxzauA6G0PKRPdsIbWa-8ka5PPopdd41580t9j_mD6ia7muslk6D9a4g1VHlIcIA0Kv1CeWx1CLiMoEo1qE25f9qLc5HImZnhCmC25dP6PhEjS1rLaTOA_LQ").unwrap();

        // https://sharedweu.weu.attest.azure.net/certs
        keys.add_key("dRKh+hBcWUfQimSl3Iv6ZhStW3TSOt0ThwiTgUUqZAo=", "vuxsTZrVoO9zJnPaHObAyLubkqsdAf69vIH8mak8j-i84ZGKaTRXM2b05rbKR-ujIqKlaIMTv7_UjO-bJCXAsEO4rD3gApJzW2f9Ezyoar4GdZ0vNNp-HU6i8mgUThRYro-efohsgEq4AZ6MsQvfUO2dMOwCb3nO1xEJJQeEr0bMxBadHiy5lbxWjpXQrNpJI5R-3TmDAJc9LODUBvn9vifSWYkwBr9-BvmsNE3v6PKY08M_7zHRjRv5DvLNH-8mXo6drnJKE4Qt7b-WoTLMBDABferx1H_gPbfkXtwFWxSQwP80gC8mUB6Fs2t60he-nqS1mVfdhIj4CzWNUzIzJQ").unwrap();
        keys.add_key("ZOub53V4dZuruzP0dOOH0axfyks", "svsohLQjA_aPyQ7EE9ZJYNYtNZ3QlFDMlVBNjurKl1r3WlM9089GMP1oc6pirai20_MuBISvmd-RDH4vLelHEfS9GrMROG-B0OrdB5mB8XlcA9ErN2_ztsqlhG28m3LTsAhMf8guhFR2-78ukOVhH0lJYFtpG9wbE0aCoBxXpSVS7JR_Otadv00EskIUoZkjx0YX94NVE7-fHMS6DD4TWEOng17mNAKELJbwdHgA5DyQsFMW7mVJVK-1BHQ5m14wYWeLadEGnzVgrc1T_x2-VJrAbSai9_xhrbZ7-RsCCTuQs0az2akbQyfb-zlyywNPIl8JO8_9j4JL8zdWNBiRAQ").unwrap();
        keys.add_key("wiFDMG11GDLqd14z3T/DUYh/ZmGaVfY0s29siGgYOJs=", "4LWU6vxZEcMRs0wqXFkMhWqQBdZLQyc0lYTsRWaqImFgOlkAElmO-mGVwteE7jZ1K81Ejr6J7PbPSPqZNLapJzWQJeRyBQmiw6s48JqhPcZpfARLTN7u0o9R03JyNsG8Oozlt3y7i7CBF7hz9UEyYdedVO1P7QqD82WuD-lbgqU").unwrap();
        keys.add_key("Ml5Yv3XS9gtdG4MjlAei4waCQHJdP2RegZIF12q8nO8=", "v8pILW0CminDc4lr9JLizZrCDgdL37Y2exoh97lgpE_kaN29nvBigwIwTwUpIFbIjznpRgEJEBHRLleAXClygU7UvU0hbEqNXbOm9IeHZA7ISfen_-UVAHI0jMudz_B-A87qndcuGBAhgGwwdAQCxxfWsCfAAzz9ewtdk0wzNURh8MjKFUXGeEAQGBNquIFJIhzMcrczuQILVqZqjBJ2HcVLXD3HR9TqOshBSO78DqlHvEuXbfU0KzwvazLXfXP9cC3zS2arvg42JdMsnDW8DZ-zCDAj7_CCzBqXnEQdjO6KUUIYCtES42OocIcN3kzPZZB763Mg07EatQjWf8Aq9w").unwrap();
        keys.add_key("9WatARJyoD4YXsBMWo8Ng4Q7CRmvzUMtD54lhSNBKlk=", "yQJmO40XJSWxNZ7c2VRw5oZdSoxsk_VxLC_g5YkizwXUmOZN2zboxoQVakyYjopd_DtUsEjdB3zxy01zOuQ1fUSWCgrS6Q2cW0x6Zt-cbdwAI4q-pU1MnMwZQzpErp5UyB151F5-VDsM3gj2uvngqJUp-Z6sZstXcNjISx0cHJE").unwrap();

        // https://shareduks.uks.attest.azure.net/certs
        keys.add_key("Gg35Is9P2ajSKfCErcfhjj1dmNE2qvK/W90/AkpLMKI=", "k5IJJJbRHM-aJZn_xVku4hvzvSmdq5mY79oETR7E-59oA7MH7LV93Ns1ekJr1iOOvmB5cd6Sn4MY_1Xmb1xVW17KsxynBMmuV5JDv7mX6oZG5u8ll__rSYlUOTjQ1v9knkqSIMK2ilkq7BVhChxKwFfWSdbbWAf6IQQBH_jphk7GaV_joOnvObivtuJG19SENiLQTtdnSZ5fhVqDa3yrQUt9ltuLn6usQRz1mVhCZNZAtrXm14I2Aa-xmsrFrjY6Bf8CJde3Z1Br2XBwaTzAfnmnmJnRu1mf95m6jwL2sbJwuPFTlWLK8ePE4if2v79qOb-ZN9JxO0Gp-5JIFXy2iw").unwrap();
        keys.add_key("7O0yBbj48oMPKpBcshFxYo2TZeo/naWA6HrguQEwoUM=", "1vrcaRrpwSrF1F2eEmCuo0L0z6-xtIw56_9Rkwch-rq6m5ZlQD1Ge-9EP4YkZI8RHh45EmPwtJiRxKVcxew73e1kzScv1K0KgEyPIF7SH9LIsYzvC_yqT_Yh45YN7TkpeYeXomQKmrOVztJeiCbw4033REoRAwL-mzV_8Ny-U7s").unwrap();
        keys.add_key("qv2BX59hNDDfXUYjYv4qz+21oJalFVo/bRVLXuZVS90=", "xH8qOT1V0D_VSuXofPmpkRUlu5QDJ8hww7DKIRbWaEBpmrXuk2m1ikCuBW0lbv2tkMY4k43WVXMVI-lt0GDbsGWFHyfL_30PY0oi1vIuzBbL4cBN-lBLmBhDRCuDn0-aJEWSTPSlJIFjYzc-c9gz83hoOI1SY9USqjB9riMJ2isW2BEDqe27ZqykZXt3SYUKG1U1BZkbiQPV0gD0j1bHRDyGdLT4Kb7JUCWufj_x1tFOu3FK-8RDXiiFtAw7h8iEi2IxQFb5f2RzfxMT270Xq5WYUlXd2nvpdxc4qWxMPSNw8tlZdD0Z3ERNCeEdijmuBmz_3yFxitqd7HKg_zLYKQ").unwrap();
        keys.add_key("rOX5awG1J9LtJ3FzAHP4JpGU4VdN+6kURiJk4ihP+rs=", "3AK5i1XgPkma26rhDNGsX_uhZN9f69WMSJd11YNdUX2CSgn_GPXx-mqHNvAnLeDAiKtPFXr86QQ7utlJm86FrL1CSPGNmp1CCtZrDlfjXvk50DYk71-Fy4ENESB-wzOfUf0G2EFHNp5SBRu98DARXAqC_bEO12uniwnZGzD6OnE").unwrap();

        // https://sharedcae.cae.attest.azure.net/certs
        keys.add_key("DyFmfM2exd0G+b1RNUxb6Gqfb7eGkQnR/Fkzuv664O4=", "1jyOOFgLcVzJUmu5hDaSsfsCzZD8h0pReKavwhLwu33McW-_Gc8_kjaWqLJ-sRpU4D_bFqT_oeoeAxaBa_ybP9nwG1fHMKDwnCUPLDvIe64PA_wd1ryICeccx9-fdpOxHR4N_3AJ5RgDAIHA_RJP_ZG8ufnrt4l8g2W9dH78EEu362F7YvcgQbqvrZaX1n3DoKxOzxcK9-GwPlM_LMixA1p0FzPKMsos3tNyL_brUsyifYkmI5lJwinoN-wILalwFp3_zfo0VhBzC_vznDhRl8xmnObJaB4U46Xnr4FrNySmkvHJiKBObx-qNeKq14BVtRLXURxzzySbW2rBSjIl7w").unwrap();
        keys.add_key("JAemi6NII4g+FQ+c3+F4APw7C2dzdEmvxW5Ek5kibBU=", "7PlUfVZZVMQVVxCv37VucL6d_F5R3ft_1kmrm325zkP4RJ3K_-ps3YwTJp4bvVD1oMyVeTV6SsqWGWxx7Fr132-PhyvW2Jv_ypTgl6Y4GMQIGN97yoxTWmLGzCzRGqu59756aZpEcL1xD3avjqjUNUbJORrMz9Bitm-kx6uysyc").unwrap();
        keys.add_key("jYO/rTZckuJjEVx4yBjIKHazA/7KS96JvWVxa6rUkgk=", "vL6Zk5XVNalVPWQm2VRYGQtpP4KE2amm_ATeE-XA45SEuX-NoZViV1WBaGkiaIsIYJwRv8jQyfffgHfyOaOv-whbOY5bc2OMnLj4PH4jwun8kdUNVHkoZUyIfTgR4OLugRneEKwcgvCO6NC_RRImviCIxHX446SmZ4j1d0BEUGGae7CA5aUpU6jPGbS8lCfilT6PMkQ4hD8C_bRk9d4ocUC_c93Q3gT_YJo1G83aeZrhf1G8FZsDwWhM8LTWcL9PdksD-3-DI6va0T5wSA4C807b8hnh00jddqJmQa3GCswf7tt8jkmBKcsN_7V7gxPNgqMWoRRfP4sHxWLlgDmBLw").unwrap();
        keys.add_key("PDnK1/dvQPqhFD4FcNJ3CXxS8CLpxlEcT9uCJXyklXo=", "p2NN0d4RGGnaN6XwbUVc2GAz3fT5zLdwKoW8I59oO78E1QAG1gS-LV5RHHRzBa7Id6XEP2zSzzPB-8JuGwcewywakY9EIei3L1Jz_PPi0L_E8ogS9PkVM34DBFdfvi2wWMTPC0qcspn-zRu6yaHMw5G_V1qc0Grkv63zdwFH_a8").unwrap();

        keys
    };

    pub static ref PPID_WHITELIST: SgxMutex<allow_list::Data>  = {
        macro_rules! add_machine {
            ($set:expr, $hex:literal) => {
                $set.add_new(hex_literal::hex!($hex), true);
            };
        }

        let mut set = allow_list::Data {
            m_to_o: HashMap::new(),
        };

        add_machine!(set, "01507c957789b7c1afde972d67f1fdd53af1a8da");
        add_machine!(set, "03372b2a39e0713c965a0876b5e807b41d509538");
        add_machine!(set, "04f01407b762af16db04ac648aee5feb24cf6eb8");
        add_machine!(set, "050430408a4ceae5d0b9d95721d651222cbd83d3");
        add_machine!(set, "051f83eb42dfdd7850086cf5696fbb3653f883ae");
        add_machine!(set, "09e9875ed7acd42c7dd19d72a39524f6ae3e87fa");
        add_machine!(set, "1121e6d9a770c9e562ae423012080e52765ecf71");
        add_machine!(set, "13f108bdf8fd3f11e75026d98ed1803075257354");
        add_machine!(set, "14d1236033fde31e0bcd570c3291eaa9bdb7e25e");
        add_machine!(set, "1632ea13051cc4d392cb8d3e016eb5617d8e8b2a");
        add_machine!(set, "188120cd27f658a7292c466f6c7df5af6cdb966e");
        add_machine!(set, "1a2425c4955555f0ea8c0315e116e0135f3a04c9");
        add_machine!(set, "1ae4fb51626d0d274c0bd3851e5a04dee0b0d53b");
        add_machine!(set, "1b3aade441add658f9f95c10ce0f3d754f92a327");
        add_machine!(set, "1c8be811c09185a9f6c7dc3fba81e878a58d0a1b");
        add_machine!(set, "20598fcb4ba579c5a09e8c1a42ec9c0210cb43f3");
        add_machine!(set, "2087688522965eb04c9ae9f33ea94bbf820878f1");
        add_machine!(set, "211f0f5570f7aabfe6a8b7cf77dec8d33dd5feea");
        add_machine!(set, "21cae822314c17721677c41cd8423464983553c1");
        add_machine!(set, "221cd8234e369ef19dfdad94f437d3a190cb2673");
        add_machine!(set, "2491b9a22f4b3d4582af20921919bc2771c80fde");
        add_machine!(set, "26671a0931ebde25273736bf6b4b2ed1038c4443");
        add_machine!(set, "28049f3f69f6558324e52d44ecdef897b1a3b390");
        add_machine!(set, "2854e3e3894986168e90ce3a017bfecf65ce7911");
        add_machine!(set, "29f3c41bbf0dbd38fd4b052d27ca6486a906688d");
        add_machine!(set, "2a003f2b908b08476be25d011c57fe11fc6892f9");
        add_machine!(set, "307bbcfb24abdb94a6b7cabaddb919046403ffe6");
        add_machine!(set, "315db04df669894a3e35429691b8b350d8cbcde2");
        add_machine!(set, "32ea0bc90831a8fb988c208e5328ac7b64eb298d");
        add_machine!(set, "346df4cce9f27b23c5f0823ee9d260c166fed3db");
        add_machine!(set, "3599c48a2a1bbff874dac46d986a512f69fbc8d0");
        add_machine!(set, "36a37506dcc62a2153ae3da741f56a01bed54267");
        add_machine!(set, "3e6830d6a2d839bf36bb108ca8ccc25a1678f82f");
        add_machine!(set, "40832a64b27c12c7abe6be0947163ee483478c61");
        add_machine!(set, "4101a8187d20889c4fd247b4c8279b66318ef091");
        add_machine!(set, "4226d078029b9be4f32a615e9290dbcde3d55f76");
        add_machine!(set, "443b01f07719d7ce6cbde861082a33181bb24e4c");
        add_machine!(set, "44e10597b42e571423915385fe85ced0e840850c");
        add_machine!(set, "461be5de74ce833d3828fcb57c243b6015d76d7f");
        add_machine!(set, "488228894e7265ff74d97495d2d536b9c7f57411");
        add_machine!(set, "4af5d94e2281e2a4c23ed84c4a05aa5e7a3178d2");
        add_machine!(set, "4e09788159008f91d7f3d68b1af2355b3c1772cc");
        add_machine!(set, "4ff7e9724d98c50d6118f65f3004a1fccded1ad8");
        add_machine!(set, "500f6449b97e2be1e68f0cb1ed59dee3c8e6d9c0");
        add_machine!(set, "51c1ede18e73fc7e00e8fc018ede53632f8bb6d9");
        add_machine!(set, "53543f1a7d08c13d69385b8120562a76b871f8ec");
        add_machine!(set, "57198eb36e5c996863b626eb6f23e3877cb2ed17");
        add_machine!(set, "57a6877d0e96ce77a3fafb0c2f7e9ed9e28fb037");
        add_machine!(set, "5af0f73c1e99f55f27d022400f5c4b9216593532");
        add_machine!(set, "5d73a589b357f4e7ad599b4a4a7e4338cd733018");
        add_machine!(set, "608a11e7ce88e1abd1c54a3566cc7b6332be05b5");
        add_machine!(set, "61897d0310110214035bcbad0f7a46ff447ec0e7");
        add_machine!(set, "674497918b4204d3e686ba23408a9aa216b2227a");
        add_machine!(set, "68928aa2c3167d75d566e54b475462aa2872cb2f");
        add_machine!(set, "6925e66c41bb7e7f5ec3808f7ae7a05c419fecc4");
        add_machine!(set, "6a81ac2a32cc2eac4b974b1819ed2b7568c83c04");
        add_machine!(set, "6ce72aa22018622c24a2c0aac45f14617aec59e5");
        add_machine!(set, "70b931353e3925bbf3dd70046f32911c9761fca6");
        add_machine!(set, "7402b63c09f3520931dac2f9f702ce16503d3648");
        add_machine!(set, "782770253c0ae72edd91138ad0017cdb9a7d8dba");
        add_machine!(set, "791773aa0b8ae2dc8cb75eb9d564a0d598125572");
        add_machine!(set, "7a89c9dd7383dde27419b36e2c6d0d942eee85d9");
        add_machine!(set, "7d0af9bbd94bc0cca22684e86ac62b58e5134223");
        add_machine!(set, "7f1c6994729637e3fd4cc2f5faaeb9bffb17c1a2");
        add_machine!(set, "803bcee8a47247aa762a29d9ed59d63defc5f861");
        add_machine!(set, "80fcb4f6f086e6bbd832500c2b729c26b3bf1ad2");
        add_machine!(set, "813bf820fd5435d03db7bbeb04d2c34217f2f949");
        add_machine!(set, "84ca012846315415126521d8afdf5dda5f775306");
        add_machine!(set, "8796f516e23e15624e50e6d34a3ab4d812fdca7e");
        add_machine!(set, "8f06c7f4f5f86842aee784337021ebd59d4af9f1");
        add_machine!(set, "8fc9821dbfa83b217f8f61f1f41f057425dcd3b6");
        add_machine!(set, "8ffbc5ef59ef9f2289e74a3745039d900fba30fe");
        add_machine!(set, "9048c142de1fe70d3ef4a9107ccfbe235df536c0");
        add_machine!(set, "92166c988e5e02184bf2d5b3b97a1e76bd6112fe");
        add_machine!(set, "93bd0a2c4b01d3fb0f21b072c04feeea7e649ae2");
        add_machine!(set, "97587e418dc1afbca9939c06ccb97f3115c58c65");
        add_machine!(set, "98cb371d43682eb87d6fb1ac1c9589d79fcd69ce");
        add_machine!(set, "9ace47eacbe6ab5112bd6d8ebc555b0fef9f2332");
        add_machine!(set, "9c80bc6bf4586edb2689c3093a9a0aeca3014805");
        add_machine!(set, "9d08171ec1cae6ce1ea0ce3f3674fa5d01f6f1b3");
        add_machine!(set, "9d3f1160d215e9438a601c8771a972bcebe097f0");
        add_machine!(set, "9e8320462733ed8a124fc97d7fdf33840b6e3a2c");
        add_machine!(set, "9ebd6f9e550f0d6dd00de5255f03ded61e17e2c7");
        add_machine!(set, "a02f25afce5eeab321a4e76943e56d6e9aa73fe4");
        add_machine!(set, "a0417d627d225e21543cf9a91b2d83b95f091dc6");
        add_machine!(set, "a40b9476fb305c1d39752fd48548c5fa3c371571");
        add_machine!(set, "a46849552c7363285baedb5a6b6814809a59ed92");
        add_machine!(set, "a6adabb2a25d233c62afd1fc0e991d2513dc11d4");
        add_machine!(set, "ac47f3115109baeb98deafdcbd980117b4e286ba");
        add_machine!(set, "af1578fbef76672fef3b9250b30cef407e78fd26");
        add_machine!(set, "b2ed08994ae4d6db46b316d384380f69636404fa");
        add_machine!(set, "b2fbe8a9d6b17f9af5110ab556d8b144bc74964d");
        add_machine!(set, "b3c0a1406894e931961e0fe34a68bd0148337e58");
        add_machine!(set, "b4cdc2e6620dfd8c5d565041284714c38079c74c");
        add_machine!(set, "b7ed7f4a1288c98c49b46a6af856532e65eaedab");
        add_machine!(set, "bb98127d89b2e3f4f9db410b060e0affa48e07ef");
        add_machine!(set, "bbed9ac0c3b9203d4da19cab34fbfb8ce6a4411c");
        add_machine!(set, "bd55e813db245b64f2269fc767ee1db10ad9df94");
        add_machine!(set, "bf101c4de90cdc2e66744438391f60cea977c9cc");
        add_machine!(set, "c133f0c3751a908584dcdf322d72b5177c12e732");
        add_machine!(set, "c2f7f3b60acde8ddff167104c56e1ed5a1e3eb74");
        add_machine!(set, "c3108fa405ef44a71a5d880321e09a620f7f4f76");
        add_machine!(set, "c31a7d5c5080d90a620d364854ba336dc82cb2c1");
        add_machine!(set, "c4ad3980ab8772d4d63b0a7f51badf0065dc095a");
        add_machine!(set, "c4c10766c4489b7177cf5a21a0dc48e689999ae8");
        add_machine!(set, "c4c2ef795e371116eea7fe8b981e38b7e0cf1017");
        add_machine!(set, "c645eccebc5aa8193b4be85c71ec550f6e0bf6e9");
        add_machine!(set, "c66551e2c5576b91e2954dca7679a92665f5894c");
        add_machine!(set, "c8638dc5e92933709f647ca7ab78eeb39d399575");
        add_machine!(set, "c9b252641dfcbdf95878b1cc0319212c741cfead");
        add_machine!(set, "cbfdda923307f8ab91897131b613b7d8e7d622e2");
        add_machine!(set, "cdd128056d8f6bb1cb3117f37f0aeadc1a51f29b");
        add_machine!(set, "ce8a5c6301b18aacde48ff4c838b59e38763e605");
        add_machine!(set, "d0c4657eb40adc662378ad0b6b2513c6138ef551");
        add_machine!(set, "d29e7c5a8b1cbdf63629f0862893a8f89077b41a");
        add_machine!(set, "d41b6b7fda64dc1e4810a779f97797c05fed4e80");
        add_machine!(set, "d754cba13fc2e37b0b8f92106521450c7824e85a");
        add_machine!(set, "d7da520ccc85d56c19256fe67852e87a3d099de6");
        add_machine!(set, "d914c2a49cf19485ef4728e47ceb66043040e0ff");
        add_machine!(set, "d963e81acf291c7aeeb5fa0ff61591efb86d30d2");
        add_machine!(set, "dac4c291870cbd898af53ecc298626474b9b5e9d");
        add_machine!(set, "db7694a475e9d6af4009b60c4be0ec87ad1a6ec6");
        add_machine!(set, "dd14db55a249438d2ec6d1be1f7c1d57966b9ae8");
        add_machine!(set, "df25ed09fca77a19d886ad4f2bca2663c0a5c113");
        add_machine!(set, "e0ae16c0875105d0de9ca5fe53a94c93a401c9b1");
        add_machine!(set, "e0d122f611e3795b28a24157a9c0108bca5792f4");
        add_machine!(set, "e142df5d190e4c77da81eaab6c81886a9ad76139");
        add_machine!(set, "e255632da8cb968a9b8a7cccb6cf020ccecafe22");
        add_machine!(set, "e35a36f5a1ee2ba8fc856905299c84f75b61e830");
        add_machine!(set, "e57183d0f063548689c5a14d3b72d877121441f1");
        add_machine!(set, "e589cfb3fc64c0a69a66f313cba7d19e8b86cf46");
        add_machine!(set, "e6802cc07ca9dcc91edea1d5a88a59a201090611");
        add_machine!(set, "e8bf9390ba54cd80001308c6e257ac77d03c618c");
        add_machine!(set, "edbb7273c0238a72f891b675b6875fa8e66e099c");
        add_machine!(set, "edbe8cf59a642bacd189e4e00562dfabaefd1120");
        add_machine!(set, "ede90ddfdc82cb63ea76f715899663d289f451e4");
        add_machine!(set, "ee90d98f773ef09204863fb6f2af335d33932fbd");
        add_machine!(set, "f02c4eef50b2b4660d291c8136a846342f790b9d");
        add_machine!(set, "f228e8bb0aea2e37066f93db5cbfd9342d8fff0d");
        add_machine!(set, "f3dc488ea5f41156d497cf0fec05449d18aa2a49");
        add_machine!(set, "f4e12f72accdc83ea3e16b730779690b4a9e0bed");
        add_machine!(set, "f8e61a4a80c77028beb33ae4c46e69e6f4904e61");
        add_machine!(set, "f963a2bd05716c429d66647b6ef1538747cce55c");
        add_machine!(set, "fbeb136909163a115f423ef4c35d4cdcccfe336d");
        add_machine!(set, "fc2c11c95a5aadfd3500891ece06651c0b4ee0e5");
        add_machine!(set, "fec9342e9ee4186453f8a7e027fac8c24e7c0c60");
        add_machine!(set, "fff4fe67c52b8d0d425d2b9772e7a4ffcd9dacf3");

        SgxMutex::new(set)
    };

    static ref FMSPC_EOL: HashSet<&'static str> = HashSet::from([
        "00706A100000",
        "00706A800000",
        "00706E470000",
        "00806EA60000",
        "00806EB70000",
        "00906EA10000",
        "00906EA50000",
        "00906EB10000",
        "00906EC10000",
        "00906EC50000",
        "00906ED50000",
        "00A065510000",
        "20806EB70000",
        "20906EC10000",
    ]);

    pub static ref SELF_QUOTE_UNTESTED: Result<AttestationCombined, sgx_types::sgx_status_t> = {
        #[cfg(feature = "SGX_MODE_HW")]
        {
            get_quote_ecdsa_untested(&[])
        }

        #[cfg(not(feature = "SGX_MODE_HW"))]
        {
            Err(sgx_types::sgx_status_t::SGX_ERROR_NO_DEVICE)
        }
    };

    pub static ref SELF_QUOTE_PPID: Option<Vec<u8>> = {
        match &*SELF_QUOTE_UNTESTED {
            Ok(x) => unsafe { x.extract_cpu_cert() },
            _ => None
        }
    };

    pub static ref SELF_MACHINE_ID: Option<allow_list::MachineID> = {
        let ppid = SELF_QUOTE_PPID.as_ref()?;
        let machine_id = crate::registration::offchain::calculate_truncated_hash(&ppid);
        trace!("Self machine_id = {}", hex::encode(&machine_id));
        Some(machine_id)
    };

}

unsafe fn extract_fmspc_from_collateral(vec_coll: &[u8]) -> Option<String> {
    struct CollHdr {
        sizes: [u32; 8],
    }
    let i_tcb_idx = 5;

    let my_p_hdr = vec_coll.as_ptr() as *const CollHdr;

    let mut size0: u64 = mem::size_of::<CollHdr>() as u64;
    for i in 0..i_tcb_idx {
        size0 += (*my_p_hdr).sizes[i] as u64;
    }

    let size_tcb_info = (*my_p_hdr).sizes[i_tcb_idx];
    let size1 = size0 + size_tcb_info as u64;

    if (size1 > size0) && (size1 <= vec_coll.len() as u64) {
        let sub_slice = &vec_coll[size0 as usize..(size1 - 1) as usize];

        let my_val: Result<serde_json::Value, _> = serde_json::from_slice(sub_slice);
        if let Ok(json_val) = my_val {
            // Navigate to fmspc
            let fmspc = &json_val["tcbInfo"]["fmspc"];
            if let Some(fmspc_str) = fmspc.as_str() {
                return Some(fmspc_str.to_string());
            }
        }
    }

    None
}

pub struct AttestationCombined {
    pub quote: Vec<u8>,
    pub coll: Vec<u8>,
    pub jwt_token: Vec<u8>,
}

impl AttestationCombined {
    pub fn from_blob(blob_ptr: *const u8, blob_len: usize) -> AttestationCombined {
        let mut res = AttestationCombined {
            quote: Vec::new(),
            coll: Vec::new(),
            jwt_token: Vec::new(),
        };

        if (blob_len > 0) && (unsafe { *blob_ptr } != 0) {
            // try to deserialize in a newer format
            let mut pos = 0;
            while pos + mem::size_of::<u32>() < blob_len {
                let key = unsafe { *(blob_ptr.add(pos)) };
                pos += 1;

                let value_size =
                    u32::from_le(unsafe { *(blob_ptr.add(pos) as *const u32) }) as usize;
                pos += mem::size_of::<u32>();

                if pos + value_size > blob_len {
                    break;
                }

                let value = unsafe { slice::from_raw_parts(blob_ptr.add(pos), value_size) };
                pos += value_size;

                match key {
                    2 => res.quote = value.to_vec(),
                    3 => res.coll = value.to_vec(),
                    4 => res.jwt_token = value.to_vec(),
                    _ => {}
                };
            }
        } else {
            // legacy
            let n0 = mem::size_of::<u32>() as u32 * 3;

            if blob_len >= n0 as usize {
                let p_blob = blob_ptr as *const u32;
                let s0 = u32::from_le(unsafe { *p_blob });
                let s1 = u32::from_le(unsafe { *(p_blob.offset(1)) });
                let s2 = u32::from_le(unsafe { *(p_blob.offset(2)) });

                let size_total = (n0 as u64) + (s0 as u64) + (s1 as u64) + (s2 as u64);

                if size_total <= blob_len as u64 {
                    //res.epid_quote =
                    //    unsafe { slice::from_raw_parts(blob_ptr.offset(n0 as isize), s0 as usize).to_vec() };
                    res.quote = unsafe {
                        slice::from_raw_parts(blob_ptr.offset((n0 + s0) as isize), s1 as usize)
                            .to_vec()
                    };
                    res.coll = unsafe {
                        slice::from_raw_parts(blob_ptr.offset((n0 + s0 + s1) as isize), s2 as usize)
                            .to_vec()
                    };
                }
            }
        }

        res
    }

    pub fn save(&self, f_out: &mut File) {
        let is_legacy = true;

        if is_legacy {
            let size_epid: u32 = 0;
            let size_dcap_q = self.quote.len() as u32;
            let size_dcap_c = self.coll.len() as u32;

            f_out.write_all(&size_epid.to_le_bytes()).unwrap();
            f_out.write_all(&size_dcap_q.to_le_bytes()).unwrap();
            f_out.write_all(&size_dcap_c.to_le_bytes()).unwrap();

            f_out.write_all(&self.quote).unwrap();
            f_out.write_all(&self.coll).unwrap();
        } else {
            Self::write_section(f_out, 2, &self.quote);
            Self::write_section(f_out, 3, &self.coll);
        }
    }

    fn write_section(f_out: &mut File, key: u8, value: &[u8]) {
        f_out.write_all(&key.to_le_bytes()).unwrap();

        let len = value.len() as u32;
        f_out.write_all(&len.to_le_bytes()).unwrap();

        f_out.write_all(value).unwrap();
    }

    pub unsafe fn extract_cpu_cert(&self) -> Option<Vec<u8>> {
        let my_p_quote = self.quote.as_ptr() as *const sgx_quote_t;

        let sig_len = (*my_p_quote).signature_len as usize;
        let whole_len = sig_len.wrapping_add(mem::size_of::<sgx_quote_t>());
        if (whole_len > sig_len)
            && (whole_len <= self.quote.len())
            && (sig_len >= mem::size_of::<sgx_ql_ecdsa_sig_data_t>())
        {
            let p_ecdsa_sig = (*my_p_quote).signature.as_ptr() as *const sgx_ql_ecdsa_sig_data_t;

            let auth_size_brutto = sig_len - mem::size_of::<sgx_ql_ecdsa_sig_data_t>();
            if auth_size_brutto >= mem::size_of::<sgx_ql_auth_data_t>() {
                let auth_size_max = auth_size_brutto - mem::size_of::<sgx_ql_auth_data_t>();

                let auth_data_wrapper =
                    (*p_ecdsa_sig).auth_certification_data.as_ptr() as *const sgx_ql_auth_data_t;

                let auth_hdr_size = (*auth_data_wrapper).size as usize;
                if auth_hdr_size <= auth_size_max {
                    let auth_size = auth_size_max - auth_hdr_size;

                    if auth_size > mem::size_of::<sgx_ql_certification_data_t>() {
                        let cert_data = (*auth_data_wrapper).auth_data.as_ptr().add(auth_hdr_size)
                            as *const sgx_ql_certification_data_t;

                        let cert_size_max =
                            auth_size - mem::size_of::<sgx_ql_certification_data_t>();
                        let cert_size = (*cert_data).size as usize;
                        if (cert_size <= cert_size_max) && ((*cert_data).cert_key_type == 5) {
                            let cert_data = slice::from_raw_parts(
                                (*cert_data).certification_data.as_ptr(),
                                cert_size,
                            );

                            return Self::extract_cpu_cert_raw(cert_data);
                        }
                    }
                }
            }
        }

        None
    }

    fn extract_cpu_cert_raw(cert_data: &[u8]) -> Option<Vec<u8>> {
        let pem_text = match std::str::from_utf8(cert_data) {
            Ok(x) => x,
            Err(_) => {
                return None;
            }
        };

        // Find the first PEM block
        let begin_marker = "-----BEGIN CERTIFICATE-----";
        let end_marker = "-----END CERTIFICATE-----";
        let start = match pem_text.find(begin_marker) {
            Some(x) => x + begin_marker.len(),
            None => {
                println!("no begin");
                return None;
            }
        };

        let end = match pem_text.find(end_marker) {
            Some(x) => x,
            None => {
                println!("no end");
                return None;
            }
        };
        let b64 = &pem_text[start..end];

        // Remove whitespace and line breaks
        let b64_clean: String = b64.chars().filter(|c| !c.is_whitespace()).collect();

        // Decode Base64 into DER
        let der_bytes = match base64::decode(b64_clean) {
            Ok(x) => x,
            Err(_) => {
                return None;
            }
        };

        let ppid_oid = &[
            0x06, 0x09, 0x2A, 0x86, 0x48, 0x86, 0xF8, 0x4D, 0x01, 0x0D, 0x01,
        ];

        let res = match crate::registration::cert::extract_asn1_value(&der_bytes, ppid_oid) {
            Ok(x) => x,
            Err(_) => {
                return None;
            }
        };

        Some(res)
    }

    pub unsafe fn verify_fmspc(&self) -> bool {
        if let Some(fmspc) = extract_fmspc_from_collateral(&self.coll) {
            let set = &FMSPC_EOL;
            let fmspc_str: &str = &fmspc;
            if set.contains(fmspc_str) {
                error!("The CPU is deprecated. Running forbidden");
                return false;
            }
            // fmspc.starts_with("0090")
        } else {
            warn!("failed to fetch fmspc from attestation");
        }

        true
    }

    fn decode_jwt(jwt_token: &str) -> Result<serde_json::Value, Box<dyn std::error::Error>> {
        let parts: Vec<&str> = jwt_token.split('.').collect();
        if parts.len() != 3 {
            return Err("JWT must have exactly 3 parts".into());
        }

        let (header_b64, claims_b64, sig_b64) = (parts[0], parts[1], parts[2]);

        let header_bytes = my_decode_base64(header_b64)?;
        let header_json: Value = serde_json::from_slice(&header_bytes)?;

        // Base64url decode and JSON parse claims
        let claims_bytes = my_decode_base64(claims_b64)?;
        let claims_json: Value = serde_json::from_slice(&claims_bytes)?;

        // Base64url decode signature (raw bytes)
        let signature_bytes = my_decode_base64(sig_b64)?;

        // println!("JWT header {}", &header_json);
        // println!("JWT claims {}", &claims_json);
        // println!("JWT signature {}", hex::encode(&signature_bytes));

        let kid_str = header_json
            .get("kid")
            .and_then(|v| v.as_str())
            .ok_or("missing 'kid' in header")?;

        //println!("kid_str {}", kid_str);

        let kid_bytes = base64::decode(kid_str)?;
        //println!("Kid {}", hex::encode(&kid_bytes));

        // 3️⃣ Prepare the signed message (header + '.' + claims)
        let mut message = Vec::new();
        message.extend_from_slice(header_b64.as_bytes());
        message.push(b'.');
        message.extend_from_slice(claims_b64.as_bytes());

        let known_keys = &KNOWN_JWT_KEYS;
        if let Some(verifying_key) = known_keys.coll.get(&kid_bytes) {
            let signature = rsa::pkcs1v15::Signature::try_from(signature_bytes.as_slice())
                .map_err(|e| format!("invalid signature: {e}"))?;

            // 5️⃣ Verify signature
            verifying_key
                .verify(&message, &signature)
                .map_err(|_| "invalid signature")?;
        } else {
            let poc_key = hex_literal::hex!(
                "5ea69fede5bcf71054395b273bad67f67158c242d77945d436374020ece525cb"
            );
            if kid_bytes == poc_key {
                let pubkey = ed25519_dalek::PublicKey::from_bytes(&poc_key)
                    .map_err(|e| format!("invalid POC key: {e}"))?;
                let sig = ed25519_dalek::Signature::from_bytes(signature_bytes.as_slice())
                    .map_err(|e| format!("invalid POC sig: {e}"))?;
                pubkey
                    .verify_strict(&message, &sig)
                    .map_err(|e| format!("invalid POC sig: {e}"))?;
            } else {
                return Err(format!("Unknown kid: {}", kid_str).into());
            }
        }

        Ok(claims_json)
    }

    pub fn verify_jwt_token(&self) -> bool {
        let s = match std::str::from_utf8(&self.jwt_token) {
            Ok(s) => s,
            Err(e) => {
                println!("Not a valid decode_jwt token: {}", e);
                return false;
            }
        };

        let claims_json = match Self::decode_jwt(s) {
            Ok(x) => x,
            Err(e) => {
                println!("decode_jwt failed: {}", e);
                return false;
            }
        };

        let quotehash_str =
            if let Some(x) = claims_json["x-ms-sgx-collateral"]["quotehash"].as_str() {
                x
            } else {
                println!("quotehash not found");
                return false;
            };

        let quote_hash = match hex::decode(quotehash_str) {
            Ok(x) => x,
            Err(e) => {
                println!("quotehash decode failed {}", e);
                return false;
            }
        };

        let mut hasher = sha2::Sha256::new();
        hasher.update(&self.quote);
        let quote_hash_actual = hasher.finalize();

        if (&quote_hash_actual as &[u8]) != quote_hash {
            println!(
                "quotehash masmatch. in-token: {}, actual: {}",
                quotehash_str,
                hex::encode(quote_hash_actual)
            );
            return false;
        }

        true
    }
}

pub struct VerifiedSgxQuote {
    pub body: sgx_report_body_t,
    pub qv_result: sgx_ql_qv_result_t,
    pub machine_id_hash: Option<allow_list::MachineID>,
}

pub fn verify_quote_sgx(
    attestation: &AttestationCombined,
    time_s: i64,
    check_ppid_wl: bool,
) -> sgx_types::SgxResult<VerifiedSgxQuote> {
    let qv_result = verify_quote_any(&attestation.quote, &attestation.coll, time_s)?;

    if attestation.quote.len() < mem::size_of::<sgx_quote_t>() {
        trace!("Quote too small");
        return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
    }

    let my_p_quote = attestation.quote.as_ptr() as *const sgx_quote_t;

    unsafe {
        let version = (*my_p_quote).version;
        if version != 3 {
            trace!("Unrecognized quote version: {}", version);
            Err(sgx_status_t::SGX_ERROR_UNEXPECTED)
        } else {
            if !attestation.verify_fmspc() {
                return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
            }

            let machine_id_opt = if let Some(ppid) = attestation.extract_cpu_cert() {
                Some(crate::registration::offchain::calculate_truncated_hash(
                    &ppid,
                ))
            } else {
                None
            };

            let is_in_wl = match &machine_id_opt {
                Some(machine_id_hash) => {
                    let wl = PPID_WHITELIST.lock().unwrap();
                    if wl.m_to_o.contains_key(machine_id_hash) {
                        true
                    } else {
                        println!("Unknown Machine ID: {}", orig_hex::encode(machine_id_hash));
                        false
                    }
                }
                None => {
                    println!("Machine ID couldn't be extracted");
                    false
                }
            };

            let jwt_token_valid = if attestation.jwt_token.is_empty() {
                false
            } else {
                if !attestation.verify_jwt_token() {
                    return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
                }
                println!("JWT token is valid");
                true
            };

            if check_ppid_wl && (!is_in_wl && !jwt_token_valid) {
                return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
            }

            Ok(VerifiedSgxQuote {
                body: (*my_p_quote).report_body,
                qv_result: qv_result,
                machine_id_hash: machine_id_opt,
            })
        }
    }
}

#[cfg(feature = "SGX_MODE_HW")]
fn test_sgx_call_res(
    res: sgx_status_t,
    retval: sgx_status_t,
) -> Result<sgx_status_t, sgx_status_t> {
    if sgx_status_t::SGX_SUCCESS != res {
        return Err(res);
    }

    if sgx_status_t::SGX_SUCCESS != retval {
        return Err(retval);
    }

    Ok(sgx_status_t::SGX_SUCCESS)
}

#[cfg(not(feature = "SGX_MODE_HW"))]
pub fn get_quote_ecdsa(_pub_k: &[u8]) -> Result<AttestationCombined, sgx_status_t> {
    Err(sgx_status_t::SGX_ERROR_NO_DEVICE)
}

#[cfg(feature = "SGX_MODE_HW")]
pub fn get_quote_ecdsa_untested(pub_k: &[u8]) -> Result<AttestationCombined, sgx_status_t> {
    let mut qe_target_info = sgx_target_info_t::default();
    let mut quote_size: u32 = 0;
    let mut rt: sgx_status_t = sgx_status_t::default();

    let mut res: sgx_status_t = unsafe {
        ocall_get_quote_ecdsa_params(
            &mut rt as *mut sgx_status_t,
            &mut qe_target_info,
            &mut quote_size,
        )
    };

    if let Err(e) = test_sgx_call_res(res, rt) {
        trace!("ocall_get_quote_ecdsa_params err = {}", e);
        return Err(e);
    }

    trace!("ECDSA quote size = {}", quote_size);

    let mut report_data: sgx_report_data_t = sgx_report_data_t::default();
    report_data.d[..pub_k.len()].copy_from_slice(pub_k);

    let my_report: sgx_report_t = match rsgx_create_report(&qe_target_info, &report_data) {
        Ok(r) => r,
        Err(e) => {
            trace!("sgx_create_report = {}", e);
            return Err(e);
        }
    };

    let mut vec_quote: Vec<u8> = vec![0; quote_size as usize];

    res = unsafe {
        ocall_get_quote_ecdsa(
            &mut rt as *mut sgx_status_t,
            &my_report,
            vec_quote.as_mut_ptr(),
            vec_quote.len() as u32,
        )
    };

    if let Err(e) = test_sgx_call_res(res, rt) {
        trace!("ocall_get_quote_ecdsa err = {}", e);
        return Err(e);
    }

    let mut vec_coll: Vec<u8> = vec![0; 0x4000];
    let mut size_coll: u32 = 0;

    res = unsafe {
        ocall_get_quote_ecdsa_collateral(
            &mut rt as *mut sgx_status_t,
            vec_quote.as_ptr(),
            vec_quote.len() as u32,
            vec_coll.as_mut_ptr(),
            vec_coll.len() as u32,
            &mut size_coll,
        )
    };

    if let Err(e) = test_sgx_call_res(res, rt) {
        trace!("ocall_get_quote_ecdsa_collateral err = {}", e);
        return Err(e);
    }

    trace!("Collateral size = {}", size_coll);

    let call_again = size_coll > vec_coll.len() as u32;
    vec_coll.resize(size_coll as usize, 0);

    if call_again {
        res = unsafe {
            ocall_get_quote_ecdsa_collateral(
                &mut rt as *mut sgx_status_t,
                vec_quote.as_ptr(),
                vec_quote.len() as u32,
                vec_coll.as_mut_ptr(),
                vec_coll.len() as u32,
                &mut size_coll,
            )
        };

        if let Err(e) = test_sgx_call_res(res, rt) {
            trace!("ocall_get_quote_ecdsa_collateral again err = {}", e);
            return Err(e);
        }
    }

    println!(
        "mr_signer = {}",
        orig_hex::encode(my_report.body.mr_signer.m)
    );
    println!(
        "mr_enclave = {}",
        orig_hex::encode(my_report.body.mr_enclave.m)
    );
    println!(
        "report_data = {}",
        orig_hex::encode(my_report.body.report_data.d)
    );

    Ok(AttestationCombined {
        quote: vec_quote,
        coll: vec_coll,
        jwt_token: Vec::new(),
    })
}

#[cfg(feature = "SGX_MODE_HW")]
pub fn get_quote_ecdsa(pub_k: &[u8]) -> Result<AttestationCombined, sgx_status_t> {
    let attestation = get_quote_ecdsa_untested(pub_k)?;

    // test self
    match verify_quote_sgx(&attestation, 0, false) {
        Ok(res) => {
            trace!("Self quote verified ok");
            if res.qv_result != sgx_ql_qv_result_t::SGX_QL_QV_RESULT_OK {
                // TODO: strict policy wrt own quote verification
                trace!("WARNING: {}", res.qv_result);
            }
        }
        Err(e) => {
            trace!("Self quote verification failed: {}", e);
            return Err(e);
        }
    };

    Ok(attestation)
}
