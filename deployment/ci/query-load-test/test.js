const {
    SecretNetworkClient,
    Wallet,
} = require("secretjs");
const fs = require("fs");

const initial_token_amount = "100000";

const ENDPOINT = process.env["ENDPOINT"] || "http://bootstrap:1317";
const CHAIN_ID = "secretdev-1";

const c_mnemonic =
    "chair love bleak wonder skirt permit say assist aunt credit roast size obtain minute throw sand usual age smart exact enough room shadow charge";

const getValueFromEvents = (events, key) => {
  if (!events) {
    return "";
  }

  for (const e of events) {
    for (const a of e.attributes) {
      if (`${e.type}.${a.key}` === key) {
        return String(a.value);
      }
    }
  }

  return "";
}

// Returns a client with which we can interact with secret network
const initializeClient = async (endpoint, chainId) => {
    const wallet = new Wallet(c_mnemonic);
    const accAddress = wallet.address;
    return new SecretNetworkClient({
        // Create a client to interact with the network
        url: endpoint,
        chainId: chainId,
        wallet: wallet,
        walletAddress: accAddress,
    });
};

async function runContractQueryLoad(
    client,
    QUERY_LOAD_COUNT,
    contract_address,
    code_hash,
) {

    /**
     * We want to support multiple tx functions in the future, in order to do so we need to randomly choose the tx to send each time.
     * We are measuring time and we don't want the random function to affect the time so we will calculate the random numbers before running the timer
     */
    const permit = await client.utils.accessControl.permit.sign(
        client.address,
        "secretdev-1",
        "test",
        [contract_address],
        ["owner", "balance"],
        false,
    );

    const time_pre_run_single = Date.now();
    await client.query.snip20.getBalance(
        {
            contract: {address: contract_address, code_hash},
            address: client.address,
            auth: {permit}
        })
    const time_post_run_single = Date.now();

    const q_promises = [];
    const time_pre_run = Date.now();
    for (let i = 0; i < QUERY_LOAD_COUNT; ++i) {
        q_promises.push(client.query.snip20.getBalance(
            {
                contract: {address: contract_address, code_hash},
                address: client.address,
                auth: {permit}
            }));
    }

    let result = [];
    try {
        result = await Promise.all(q_promises);
    } catch (e) {
        console.log(`Error getting promises: ${JSON.stringify(e)}`)
    }
    const time_post_run = Date.now();

    let success = 0;
    for (const t of result) {
        try {
            if (t.balance["amount"] === initial_token_amount ) {
                success++;
            }
        } catch (e) {

        }
    }
    console.log(
        `------------------------------[Running Wasm Query load]------------------------------\n
        \t ** Time for a single snip20 balance query with permit: 
        ${time_post_run_single - time_pre_run_single}[ms] ** \n
        \t *** Total time for ${QUERY_LOAD_COUNT} snip20 balance queries is: ${
            time_post_run - time_pre_run
        }[ms] ***\n
        \t Success rate: ${success} / ${q_promises.length}
        --------------------------------------------------------------------------------------
        `
    );
}

// async function runTxLoad(
//     client,
//     TX_LOAD_COUNT
// ) {
//     console.log(
//         `------------------------------[Running TX load]------------------------------`
//     );
//     /**
//      * We want to support multiple tx functions in the future, in order to do so we need to randomly choose the tx to send each time.
//      * We are measuring time and we don't want the random function to affect the time so we will calculate the random numbers before running the timer
//      */
//     const tx_promises = [];
//     const time_pre_run = Date.now();
//     for (let i = 0; i < TX_LOAD_COUNT; ++i) {
//         tx_promises.push(client.query.bank.balance({address: c_address, denom: "uscrt"}));
//     }
//
//     const result = await Promise.all(tx_promises);
//     const time_post_run = Date.now();
//
//     let success = 0;
//     for (const t of result) {
//         try {
//             if (Number(t.balance["amount"]) > 10000 ) {
//                 success++;
//             }
//         } catch (e) {
//
//         }
//     }
//
//     console.log(
//         `++++ Total time for ${TX_LOAD_COUNT} bank queries is: ${
//             time_post_run - time_pre_run
//         } ms ++++`);
//
//     console.log(`Success rate: ${success} / ${tx_promises.length}`)
//
// }

const uploadContract = async (
    client,
    contractPath,
    contractName
) => {
    const wasmCode = fs.readFileSync(contractPath);
    console.log(`Uploading ${contractName} contract`);

    const uploadReceipt = await client.tx.compute.storeCode(
        {
            wasm_byte_code: wasmCode,
            sender: client.address,
            source: "",
            builder: "",
        },
        {
            gasLimit: 5000000,
        }
    );

    if (uploadReceipt.code !== 0) {
        console.log(
            `Failed to get code id: ${JSON.stringify(uploadReceipt.rawLog)}`
        );
        throw new Error(`Failed to upload contract`);
    }

    const codeIdKv = getValueFromEvents(uploadReceipt.events, "message.code_id");

    const codeId = Number(codeIdKv);
    console.log(`${contractName} contract codeId: ${codeId}`);

    const codeHash = (await client.query.compute.codeHashByCodeId({code_id: String(codeId)})).code_hash;
    console.log(`${contractName} contract hash: ${JSON.stringify(codeHash)}`);

    return [codeId, codeHash];
};

const initializeContract = async (
    client,
    contractPath
) => {
    const [codeId, codeHash] = await uploadContract(
        client,
        contractPath,
        "test-contract"
    );

    const contract = await client.tx.compute.instantiateContract(
        {
            sender: client.address,
            code_id: codeId,
            code_hash: codeHash,
            label: "contract" + Math.ceil(Math.random() * 10000), // The label should be unique for every contract, add random string in order to maintain uniqueness
            init_msg: {
                name: "Secret SCRT",
                admin: client.address,
                symbol: "SSCRT",
                decimals: 6,
                initial_balances: [{address: client.address, amount: initial_token_amount}],
                prng_seed: "eW8=",
                config: {
                    public_total_supply: true,
                    enable_deposit: true,
                    enable_redeem: true,
                    enable_mint: false,
                    enable_burn: false,
                },
                supported_denoms: ["uscrt"],
            },
        },
        {
            gasLimit: 4000000,
        }
    );
    console.log(`decrypt: ${JSON.stringify(contract)}`)

    if (contract.code !== 0) {
        throw new Error(
            `Failed to instantiate the contract with the following error ${contract.rawLog}`
        );
    }

    const contractAddress = (await client.query.compute.contractsByCodeId({code_id: String(codeId)}))
        .contract_infos[0].contract_address;

    console.log(`Contract address: ${contractAddress}`);

    return [codeHash, contractAddress];
};

const sleep = ms => new Promise(r => setTimeout(r, ms));

(async () => {
    let endpoint = ENDPOINT;
    let chainId = CHAIN_ID;

    // todo: replace this with a loop that waits for the node
    console.log(`Sleeping for 60s till nodes start`);
    await sleep(60000);

    try {
        const client = await initializeClient(endpoint, chainId);

        const [code_hash, contract_address] = await initializeContract(client, "snip20-ibc.wasm.gz");

        for (let i = 0; i < 1; ++i) {
            await runContractQueryLoad(client, 100, contract_address, code_hash);
            console.log("\n\n");
        }
    } catch (e) {
        console.log(`Error running or initializing contract: ${JSON.stringify(e)}`);
    }

    // let prints to stdout catch up
    await sleep(10000);
})();
