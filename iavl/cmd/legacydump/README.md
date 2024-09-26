# LegacyDump

`legacydump` is a command line tool to generate a `iavl` tree based on the legacy format of the node key.
This tool is used for testing the `lazy loading and set` feature of the `iavl` tree.

## Usage

It takes 5 arguments:

    - dbtype: the type of database to use. 
    - dbdir: the directory to store the database.
    - `random` or `sequential`: The `sequential` option will generate the tree from `1` to `version` in order and delete versions from `1` to `removal version`. The `random` option will delete `removal version` versions randomly.
    - version: the upto number of versions to generate.
    - removal version: the number of versions to remove.

```shell
go run . <dbtype> <dbdir> <random|sequential> <version> <removal version>
```
