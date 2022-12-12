# Check hardware tool

## How to run

Extract `check_hw_<version>.tar.gz`:

```bash
tar -xzf check_hw_<version>.tar.gz
```

Run the `check-hw` tool:

```bash
cd check-hw/
LOG_LEVEL=WARN ./check-hw
```

If your machine is compatible and patched, you should get:

```bash
Creating enclave instance..
Platform Okay!
```
