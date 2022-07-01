# Balances exporter

This tool exports account balances, as found in the trie database, under a specific roothash. The roothash is automatically selected by the tool given the provided epoch.

## How to use

Compile the code as follows:

```
cd elrond-tools-go/balancesExporter
go build .
```

Make sure you have a node database prepared (synchronized or downloaded) in advance.

Then, run the export command for an epoch of your choice:

```
./balancesExporter --log-save --db-path=db/1 --shard=0 --epoch=690 --format=plain-json
```

### Export formats

When running the tool, you can specify the desired export format. The available formats are: 

`plain-text`:

```
erd1... 1000000000000000000 EGLD
```

`plain-json`:

```
[
    {
        "address": "erd1...",
        "balance": "1000000000000000000"
    },
...
]
```

`rosetta-json`:

```
[
    {
        "account_identifier": {
            "address": "erd1..."
        },
        "currency": {
            "symbol": "EGLD",
            "decimals": 18
        },
        "value": "1000000000000000000"
    },
...
]
```