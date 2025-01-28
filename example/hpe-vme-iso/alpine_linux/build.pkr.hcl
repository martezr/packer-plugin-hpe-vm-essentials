/*
packer {
  required_plugins {
    hpe-vme = {
      version = "0.0.1"
      source  = "github.com/martezr/hpe-vme"
    }
  }
}
*/

locals {
  timestamp = formatdate("mmss", timestamp())
  boot_command = [
    "root<enter><wait>",
    "setup-alpine<enter>", // Start Alpine Linux installation
    "us<enter>",           // Enter keyboard
    "us<enter>",
    "alpinetemp<enter>", // Enter virtual machine hostname
    "eth0<enter>",
    "dhcp<enter>",
    "n<enter>",
    "password123<enter>", // Enter Password
    "password123<enter>", // Confirm Password
    "<enter>",            // Timezone - Lowercase doesn't register properly
    "none<enter>",        // Proxy
    "r<enter>",           // APK Mirror
    "no<enter>",          // Create user
    "openssh<enter>",     // SSH Server
    "yes<enter>",         // Allow root login
    "none<enter>",        // SSH Key
    "vda<enter>",         // Install DISK
    "sys<enter>",         // Install DISK
    "y<enter>",           // Wipe Disk
    "mount /dev/vda3 /mnt<enter>",
    "chroot /mnt<enter>",
    "sed -i -e 's/#//g' /etc/apk/repositories<enter>",
    "apk update<enter>",
    "apk -U add qemu-guest-agent && rc-service qemu-guest-agent start<enter>",
    "rc-update add qemu-guest-agent default<enter>",
    "exit<enter>",
    "umount /mnt<enter>",
    "reboot<enter>" // Reboot System
  ]
  vm_name = "packiso-${local.timestamp}"
}

source "hpe-vme-iso" "demo" {
  url                 = var.vme_url
  username            = var.vme_username
  password            = var.vme_password
  cluster_name        = "vmecluster01"
  boot_command        = local.boot_command
  boot_wait           = "5s"
  convert_to_template = true
  vm_name          = local.vm_name
  template_name    = "alpinenix"
  virtual_image_id = 18
  group            = "Platform Engineering"
  cloud            = "HPE Demo"
  plan_id          = 21
  ip_wait_timeout = "5m"

  network_interface {
    network_id                = 3
    network_interface_type_id = 4
  }

  storage_volume {
    name            = "root"
    root_volume     = true
    size            = 10
    storage_type_id = 1
    datastore_id    = 2
  }

  # Raise the timeout, when installation takes longer
  ssh_timeout  = "55m"
  communicator = "ssh"
  ssh_username = "root"
  ssh_password = "password123"
}

build {
  sources = [
    "source.hpe-vme-iso.demo"
  ]

  provisioner "shell" {
    script = "setup.sh"
   }
}
