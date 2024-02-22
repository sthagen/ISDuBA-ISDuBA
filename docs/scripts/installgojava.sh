#!/usr/bin/env bash
set -e

apt-get update

apt install -y openjdk-17-jre-headless

wget https://go.dev/dl/go1.22.0.linux-amd64.tar.gz

rm  /usr/local/go -r && tar -C /usr/local -xzf go1.22.0.linux-amd64.tar.gz

rm go1.22.0.linux-amd64.tar.gz

echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile

ln -snf /usr/local/go/bin/go /usr/local/bin/go
