## 1. Import the Enigma public GPG key:

```bash
gpg2 --recv-keys 91831DE812C6415123AFAA7B420BF1CB005FBCE6
```

## 2. Check release hashes:

```bash
sha256sum --check SHA256SUMS.asc
```

## 3. Verify the hashes are singed by Enigma:

```bash
gpg2 --verify SHA256SUMS.asc
```

If you get this warning and you feel a lack of satisfaction:

```
gpg: WARNING: This key is not certified with a trusted signature!
gpg:          There is no indication that the signature belongs to the owner.
```

You may contact us on a channel you deem secure and we'll verify the fingerprint for you.

Then to eliminate future warnings run:

```bash
gpg2 --lsign 91831DE812C6415123AFAA7B420BF1CB005FBCE6
```

(If you don't already have a personal private key, generate one with `gpg2 --gen-key` or `gpg2 --expert --full-gen-key`)
