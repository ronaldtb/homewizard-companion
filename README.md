# HomeWizard Companion

This repository contains a command line program that uses the local [HomeWizard P1 Meter](https://www.homewizard.com) API to 
synchronize the gas meter readings to [mindergas.nl](https://www.mindergas.nl).

## Installation

> Make sure [local API access](https://api-documentation.homewizard.com/docs/introduction/) has been enabled.

Checkout this repository.

Create a `.homewizard-companion` file in your home directory containing the following settings, with the values between `<` and `>` replaced with the actual values.  
```yaml
p1:
  ip: <ip address of the p1 meter>
mindergas:
  token: '<token>'
```

## Running

> This application is tested with go version go1.21 on Ubuntu x86_64 and on a Raspberry Pi 4 (aarch64)

You can start the application directly from the source code using `go run main.go sync-mindergas`, but it is recommended to 
create a build.

1. Run `go build`
2. Copy the resulting `homewizard-companion` to your `/usr/local/bin/` or other directory on your `PATH`
3. Run the application in the background using `homewizard-companion sync-mindergas > sync-mindergas.log &`
4. This will make sure the application keeps running when you close your terminal and will write the log output to `sync-mindergas.log`
