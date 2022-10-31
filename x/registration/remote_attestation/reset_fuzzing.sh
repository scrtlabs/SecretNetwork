#!/usr/bin/env bash


echo "Clearing old results.."
rm crashers/*
rm suppressions/*


echo "Building new package.."
/home/bob/go/bin/go-fuzz-build github.com/scrtlabs/SecretNetwork/x/registration/remote_attestation

/home/bob/go/bin/go-fuzz -bin=remote_attestation-fuzz.zip -workdir=.
