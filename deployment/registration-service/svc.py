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

logging.getLogger("urllib3.connectionpool").setLevel(logging.ERROR)
logging.getLogger("werkzeug").setLevel(logging.ERROR)

application = Flask(__name__)
CORS(application)

wallet_name = 'a'
node_url = os.getenv("RPC_URL", 'secret-2.node.enigma.co:26657')
cert_file = 'attestation_cert.der'
chain_id = os.getenv("CHAIN_ID", 'supernova-1')

executable = ['secretcli', 'tx', 'register', 'auth', cert_file, '--node', 'tcp://' + node_url, '-y', '--from', wallet_name, '--gas', '250000', '--gas-prices', '0.25uscrt', '--chain-id', chain_id]


# @ns.param('cert', 'Base64 encoded certificate file', 'query')
@application.route('/register')
def get():  # pylint: disable=no-self-use
    filename: str = request.args.get('cert')

    decoded_cert = base64.decodebytes(filename.encode('ascii'))

    try:
        with open(cert_file, '+wb') as f:
            f.write(decoded_cert)

    except PermissionError:
        logger.critical(f'No permissions to write to file')
        return abort(500)

    try:
        resp = subprocess.check_output(executable)
        res = json.loads(resp.decode())

        if res.get("code", 0) == 0:
            return jsonify(res["txhash"])
        else:
            logger.error(f"Error creating transaction: {res['raw_log']}")
    except Exception as e:
        logger.error(f"Error while trying to create transaction: {e}")
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