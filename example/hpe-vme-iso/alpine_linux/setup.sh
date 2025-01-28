#!/bin/sh

apk add util-linux e2fsprogs-extra qemu-guest-agent sudo
apk add py3-netifaces cloud-init
passwd -d root
setup-cloud-init
#poweroff