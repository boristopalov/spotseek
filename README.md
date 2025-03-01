# spotseek

Golang client for the Soulseek protocol

## Features

- Support for searching and downloading files
- Participation in the distributed search network
- Terminal UI for searching and downloading files

## Usage

1. Make sure you have `make` installed. You can do so via `brew install make` on macOS
2. Create a `.env` file with your slsk username, password, as well as the path for sharing files (only single directory is supported currently). See `.env.example` for an example
3. Run `make run-tui` to start the client and launch the TUI

## Todo

- [ ] Add tests !!
- [ ] Support for being a parent in the distributed network
- [ ] User-configurable settings for share paths and multiple share paths
- [ ] Retrying incomplete downloads
- [ ] Persistence mechanisms
- [ ] API support

## Protocol Documentation

Documentation on the protocol is available [here](https://nicotine-plus.org/doc/SLSKPROTOCOL.html)
