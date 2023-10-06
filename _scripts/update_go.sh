#!/usr/bin/env bash

GO_INSTALL=go$GO_VERSION.linux-amd64.tar.gz

# remove exinsting go
rm -rf /usr/local/go

# download new one
curl -s https://dl.google.com/go/$GO_INSTALL -o $GO_INSTALL

# extract it
tar -C /usr/local -xzf $GO_INSTALL

#version
go version
