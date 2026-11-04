const http = require("http");
const querystring = require("querystring");
const execFile = require("child_process").execFile;

const FAUCET_WALLET_NAME = process.env.FAUCET_WALLET_NAME || "a";
const FAUCET_AMOUNT = process.env.FAUCET_AMOUNT || "1000000000";
const DENOM = process.env.DENOM || "uscrt";

let faucet_address;

/**
 * Execute a program directly (no shell) and return its JSON output as a Promise.
 * Arguments are passed as an array so user-controlled values are never
 * interpreted by a shell.
 * @param cmd {string} program to run
 * @param args {string[]} arguments to pass to the program
 * @return {Promise<string>}
 */
function execShellCommand(cmd, args = []) {
  return new Promise((resolve, reject) => {
    execFile(cmd, args, (error, stdout, stderr) => {
      if (error) {
        console.error("error in execShellCommand", error);
        reject(error);
      } else if (stderr) {
        console.error("stderr in execShellCommand", stderr);
        reject(stderr);
      } else {
        resolve(JSON.parse(stdout));
      }
    });
  });
}

/**
 * Command to send coins.
 * @param src_key_name source account key name, default 'a'
 * @param src_address  source account's secret address
 * @param dest_address destination address
 * @param amount amount to send
 * @returns result of executing the command.
 */
async function send_command(src_key_name, src_address, dest_address, amount) {
  // Build the argument list explicitly. `dest_address` is passed as a single,
  // literal argument, so it cannot be interpreted as shell syntax.
  const send_args = [
    "tx", "bank", "send",
    src_address,
    dest_address,
    `${amount}${DENOM}`,
    "--from", src_key_name,
    "--gas-prices", "0.25uscrt",
    "-y",
  ];
  console.log(`send_message: \n secretd ${send_args.join(" ")}`);

  const result = await execShellCommand("secretd", send_args);
  console.log(`Sent tokens with txhash: ${result.txhash}`);
  return result.txhash;
}

/**
 * Returns the address for the requested account key.
 * @param key_name faucet account key to use, default 'a'
 * @returns address
 */
async function get_address(key_name) {
  // Already looked up, won't change while running
  if (faucet_address !== undefined) {
    return faucet_address;
  }

  const result = await execShellCommand("secretd", ["keys", "list"]);

  for (index in result) {
    const key = result[index];
    if (key["name"] == key_name) {
      console.log(`Found key with address: ${key["address"]}`);
      faucet_address = key["address"];
      break;
    }
  }

  return faucet_address;
}

// Start the http server
const server = http.createServer();
server.on("request", async (req, res) => {
  try {
    // for root or status, return the configured faucet address and amount sent
    if (req.url === "/" || req.url === "/status") {
      const faucet_address = await get_address(FAUCET_WALLET_NAME);

      if (faucet_address === undefined) {
        console.error(
          `No key account with required name: ${FAUCET_WALLET_NAME}`
        );

        res.writeHead(500, { "Content-Type": "application/json" });
        res.write(
          JSON.stringify({
            error: `No key account with required name: ${FAUCET_WALLET_NAME}`,
          })
        );
        res.end();
        return;
      } else {
        res.writeHead(200, { "Content-Type": "application/json" });
        res.write(
          JSON.stringify({
            faucet_address: faucet_address,
            amount: FAUCET_AMOUNT,
          })
        );
        res.end();
      }
    } else if (req.url.startsWith("/faucet")) {
      // ensure address is present, not necessarily valid checksum
      if (!req.url.startsWith("/faucet?address=")) {
        res.writeHead(400, { "Content-Type": "application/json" });
        res.write(JSON.stringify({ error: "address is required" }));
        res.end();
        return;
      }

      const address = querystring.parse(req.url)["/faucet?address"];
      const faucet_address = await get_address(FAUCET_WALLET_NAME);

      if (faucet_address === undefined) {
        console.error(
          `No key account with required name: ${FAUCET_WALLET_NAME}`
        );

        res.writeHead(500, { "Content-Type": "application/json" });
        res.write(
          JSON.stringify({
            error: `No key account with required name: ${FAUCET_WALLET_NAME}`,
          })
        );
        res.end();
        return;
      } else {
        const txhash = await send_command(
          FAUCET_WALLET_NAME,
          faucet_address,
          address,
          FAUCET_AMOUNT
        );

        res.writeHead(200, { "Content-Type": "application/json" });
        res.write(JSON.stringify({ txhash: txhash }));
        res.end();
      }
    } else {
      res.end("Invalid Request!");
    }
  } catch (err) {
    res.writeHead(500, { "Content-Type": "application/json" });
    res.write(JSON.stringify({ error: `${err.message}` }));
    res.end();
  }
});

server.listen(5000);

console.log("Secret Faucet is running on port 5000 ...");
