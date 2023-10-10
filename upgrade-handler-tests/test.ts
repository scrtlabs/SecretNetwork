import { spawn } from "child_process";
const exec = require("util").promisify(require("child_process").exec);

async function main() {
  await exec("docker rm -f localsecret");
  let localsecret = spawn(
    `/bin/bash -c "docker run -p 1316:1317 --name localsecret ghcr.io/scrtlabs/localsecret:v1.11.0-beta.19"`,
    { shell: true }
  );

  const waitForUpgradeBlock = (data) => {
    const output = data.toString();
    if (output.includes("Waiting for upgrade block")) {
      console.log("Waiting for upgrade block");
      localsecret.kill();
    }
  };

  localsecret.stdout.setEncoding("utf8");
  localsecret.stdout.on("data", waitForUpgradeBlock);
  localsecret.stderr.setEncoding("utf8");
  localsecret.stderr.on("data", waitForUpgradeBlock);

  await sleep(10000);
}
main();

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
