#! /usr/bin/python3

import logging
import base64

import subprocess
import os
import json

from flask import Flask, request, abort, jsonify
from flask_cors import CORS
# from flask_restplus import Api, Resource
# from flask_restplus import abort

logger = logging.getLogger(__name__)

logging.getLogger("urllib3.connectionpool").setLevel(logging.INFO)
logging.getLogger("werkzeug").setLevel(logging.INFO)

logger.setLevel(logging.INFO)

application = Flask(__name__)
CORS(application)

node_url = os.getenv("RPC_URL", 'localhost:26657')

chain_id = os.getenv("CHAIN_ID", 'secretdev-1')

def get_address(key_name: str) -> str:
    p = subprocess.check_output(['secretd', 'keys', 'list'])
    print(f'{p}')
    res = ''
    for key in filter(lambda x: x['name'] == key_name, json.loads(p.decode())):
        res = key['address']
    if not res:
        raise RuntimeError(f"No key account with required name: {key_name}")
    return res



def send_command(src: str, dest: str, amount: str) -> str:
    exec = ['secretd', 'tx', 'bank', 'send' ]

    address = get_address(src)

    exec.append(address)
    exec.append(dest)
    exec.append(f'{amount}uscrt')
    exec.extend(['--from', src, '--gas-prices', '0.25uscrt', '-y'])
    return exec

# @ns.param('cert', 'Base64 encoded certificate file', 'query')
@application.route('/faucet')
def get():  # pylint: disable=no-self-use
    address: str = request.args.get('address')

    wallet_name = os.getenv('FAUCET_WALLET_NAME', 'a')
    faucet_amount = os.getenv('FAUCET_AMOUNT', '1000000000')

    executable = send_command(wallet_name, address, faucet_amount)

    try:
        logger.info(f"sending: {executable}")
        resp = subprocess.check_output(executable)
        res = json.loads(resp.decode())
        logger.info(f"{resp}")
        if res.get("code", 0) == 0:
            return jsonify(res["txhash"])
        else:
            logger.error(f"Error creating transaction: {res['raw_log']}")
    except Exception as e:
        logger.error(f"Error while trying to create transaction: {e} - {executable}")
        abort(500)
    finally:
        try:
            os.remove(cert_file)
        except Exception:
            pass


def start_server(port):
    application.run(host='0.0.0.0', port=port, debug=False)


if __name__ == '__main__':
    start_server(8081)