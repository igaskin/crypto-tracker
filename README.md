# crypto-tracker
cli for tracking crypto.com CRO total earning via google sheets: [example](https://bit.ly/3tt9whB)

## Usage
```bash
$ crypto-tracker -h
Import Crypto.com transactions into google sheets

Usage:
  crypto-tracker [command]

Available Commands:
  help        Help about any command
  import      Import crypto transaction csv data into google sheets
  login       Enable authentication to google sheets

Flags:
      --config string   config file (default is $HOME/.crypto-tracker.yaml)
  -h, --help            help for crypto-tracker
  -t, --toggle          Help message for toggle

Use "crypto-tracker [command] --help" for more information about a command.

$ crypto-tracker import -s <google-sheet id>
```

### Purchasing CRO
Purchasing CRO can be done via the Crypto.com App.  Installing the app with this [referral code](https://crypto.com/app/n6u6k2qya2) can earn $25 USD in CRO.

